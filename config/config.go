package config

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	Ignore        bool   `yaml:"ignore"`
	IgnoreOnError bool   `yaml:"ignore_on_error"`
	Format        string `yaml:"format"`
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
	C                   *Config
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
	return &SafeConfig{C: &Config{}, configReloadSuccess: configReloadSuccess, configReloadSeconds: configReloadSeconds}
}

func (sc *SafeConfig) ReloadConfig(configFiles string, logger *slog.Logger) (err error) {
	if strings.HasPrefix(configFiles, "https://") || strings.HasPrefix(configFiles, "http://") {
		logger.Debug("Reload configuration from url", "url", configFiles)
		return sc.reloadConfigFromUrl(configFiles)
	}

	logger.Debug("Reload configuration from files", "files", configFiles)
	return sc.reloadConfigFromFiles(configFiles)
}

func (sc *SafeConfig) reloadConfigFromFiles(configFiles string) (err error) {
	var c = &Config{}
	defer func() {
		if err != nil {
			sc.configReloadSuccess.Set(0)
		} else {
			sc.configReloadSuccess.Set(1)
			sc.configReloadSeconds.SetToCurrentTime()
		}
	}()

	files, err := filepath.Glob(configFiles)
	if err != nil {
		return err
	}

	for _, file := range files {
		var fc = &Config{}

		yamlReader, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("error reading config file: %s", err)
		}
		defer yamlReader.Close()
		decoder := yaml.NewDecoder(yamlReader, yaml.DisallowUnknownField())

		if err = decoder.Decode(fc); err != nil {
			return fmt.Errorf("error parsing config file: %s", err)
		}

		c.Scripts = append(c.Scripts, fc.Scripts...)
	}

	sc.Lock()
	sc.C = c
	sc.Unlock()

	return nil
}

func (sc *SafeConfig) reloadConfigFromUrl(configFiles string) (err error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configFiles, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get config from: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid http status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var c Config
	if err := yaml.Unmarshal(body, &c); err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	sc.Lock()
	sc.C = &c
	sc.Unlock()

	return nil
}
