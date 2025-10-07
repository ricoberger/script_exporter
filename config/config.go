package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Config struct {
	Scripts []Script `yaml:"scripts"`
}

func (c *Config) GetScript(name string) *Script {
	for _, script := range c.Scripts {
		if name == script.Name {
			return &script
		}
	}

	return nil
}

type Script struct {
	Name              string            `yaml:"name"`
	Command           []string          `yaml:"command"`
	Args              []string          `yaml:"args"`
	Env               map[string]string `yaml:"env"`
	AllowEnvOverwrite bool              `yaml:"allow_env_overwrite"`
	Sudo              bool              `yaml:"sudo"`
	Output            Output            `yaml:"output"`
	Timeout           Timeout           `yaml:"timeout"`
	Cache             Cache             `yaml:"cache"`
	Discovery         Discovery         `yaml:"discovery"`
}

type Output struct {
	Ignore        bool `yaml:"ignore"`
	IgnoreOnError bool `yaml:"ignore_on_error"`
}

type Timeout struct {
	MaxTimeout float64 `yaml:"max_timeout"`
	Enforced   bool    `yaml:"enforced"`
	WaitDelay  float64 `yaml:"wait_delay"`
}

type Cache struct {
	Duration               *float64 `yaml:"duration"`
	CacheOnError           bool     `yaml:"cache_on_error"`
	UseExpiredCacheOnError bool     `yaml:"use_expired_cache_on_error"`
}

type Discovery struct {
	Params         map[string]string `yaml:"params"`
	ScrapeInterval string            `yaml:"scrape_interval"`
	ScrapeTimeout  string            `yaml:"scrape_timeout"`
}

type SafeConfig struct {
	sync.RWMutex
	C               *Config
	ConfigSource    string
	IsURL           bool
	lastConfigHash  string // Hash of the last loaded config body from URL
	configReloadSuccess prometheus.Gauge
	configReloadSeconds prometheus.Gauge
}

func NewSafeConfig(reg prometheus.Registerer) *SafeConfig {
	configReloadSuccess := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Namespace: "script_exporter",
		Name:      "config_last_reload_successful",
		Help:      "Script Exporter config loaded successfully.",
	})

	configReloadSeconds := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Namespace: "script_exporter",
		Name:      "config_last_reload_success_timestamp_seconds",
		Help:      "Timestamp of the last successful configuration reload.",
	})
	return &SafeConfig{
		C:                  &Config{},
		ConfigSource:       "",
		IsURL:              false,
		lastConfigHash:     "",
		configReloadSuccess: configReloadSuccess,
		configReloadSeconds: configReloadSeconds,
	}
}

func (sc *SafeConfig) loadFromURL(logger *slog.Logger) error {
	resp, err := http.Get(sc.ConfigSource)
	if err != nil {
		return fmt.Errorf("failed to fetch config from %s: %w", sc.ConfigSource, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, sc.ConfigSource)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Compute hash of the body
	hash := sha256.Sum256(body)
	newHash := hex.EncodeToString(hash[:])

	// Check if changed
	if newHash == sc.lastConfigHash {
		logger.Debug("No change in config detected from URL", "url", sc.ConfigSource)
		return nil
	}

	// Parse and apply new config
	var c Config
	if err := yaml.Unmarshal(body, &c); err != nil {
		return fmt.Errorf("failed to parse YAML from URL: %w", err)
	}

	sc.Lock()
	sc.C = &c
	sc.lastConfigHash = newHash
	sc.Unlock()

	logger.Info("Config reloaded from URL due to content change", "url", sc.ConfigSource)
	return nil
}

func (sc *SafeConfig) ReloadConfig(logger *slog.Logger) (err error) {
	var c = &Config{}
	defer func() {
		if err != nil {
			sc.configReloadSuccess.Set(0)
		} else {
			sc.configReloadSuccess.Set(1)
			sc.configReloadSeconds.SetToCurrentTime()
		}
	}()

	if sc.IsURL {
		return sc.loadFromURL(logger)
	}

	files, err := filepath.Glob(sc.ConfigSource)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no config files found matching %s", sc.ConfigSource)
	}

	for _, file := range files {
		var fc = &Config{}

		yamlReader, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("error reading config file %s: %w", file, err)
		}
		defer yamlReader.Close()
		decoder := yaml.NewDecoder(yamlReader, yaml.DisallowUnknownField())

		if err = decoder.Decode(fc); err != nil {
			return fmt.Errorf("error parsing config file %s: %w", file, err)
		}

		c.Scripts = append(c.Scripts, fc.Scripts...)
	}

	sc.Lock()
	sc.C = c
	sc.Unlock()

	return nil
}
