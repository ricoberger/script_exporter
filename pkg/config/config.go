package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

// Making MaxTimeout a pointer to a float64 allows us to tell the
// difference between an explicit 0 and an unconfigured setting.
// Ditto for Enforced.
type timeout struct {
	MaxTimeout *float64 `yaml:"max_timeout"`
	Enforced   *bool    `yaml:"enforced"`
}

// Config represents the structure of the configuration file.
type Config struct {
	TLS struct {
		Enabled bool   `yaml:"enabled"`
		Crt     string `yaml:"crt"`
		Key     string `yaml:"key"`
	} `yaml:"tls"`

	BasicAuth struct {
		Enabled  bool   `yaml:"enabled"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"basicAuth"`

	BearerAuth struct {
		Enabled    bool   `yaml:"enabled"`
		SigningKey string `yaml:"signingKey"`
	} `yaml:"bearerAuth"`

	Scripts []ScriptConfig `yaml:"scripts"`

	Discovery struct {
		Host   string `yaml:"host"`
		Port   string `yaml:"port"`
		Scheme string `yaml:"scheme"`
		Path   string `yaml:"path"`
	} `yaml:"discovery"`
}

// ScriptConfig is the configuration for a single script.
type ScriptConfig struct {
	Name               string            `yaml:"name"`
	Script             string            `yaml:"script"`
	Command            string            `yaml:"command"`
	Args               []string          `yaml:"args"`
	Env                map[string]string `yaml:"env"`
	IgnoreOutputOnFail bool              `yaml:"ignoreOutputOnFail"`
	Timeout            timeout
}

// LoadConfig reads the configuration file and umarshal the data into the config struct
func (c *Config) LoadConfig(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return err
	}

	return nil
}

// ValidateConfig validates no contradictory config options are set.
func ValidateConfig(c *Config) []error {
	var errs []error
	for _, script := range c.Scripts {
		if script.Command == "" && script.Script == "" {
			err := fmt.Errorf("script config %s has neither 'script' nor 'command'", script.Name)
			errs = append(errs, err)
		}
		if script.Script != "" && (script.Command != "" || len(script.Args) > 0) {
			err := fmt.Errorf("script config %s combines mutually exclusive settings "+
				"'script' and 'command' / 'args'", script.Name)
			errs = append(errs, err)
		}
	}
	return errs
}

// GetRunArgs returns the parameters that will be passed to exec.Command to execute the script.
// Errors if the scriptName doesn't exist in the config.
func GetRunArgs(c *Config, scriptName string) ([]string, error) {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			if script.Script != "" {
				// Deprecated: scrip.Script will be replaced by 'command' and 'args'.
				return strings.Split(script.Script, " "), nil
			}
			var runArgs []string
			runArgs = append(runArgs, script.Command)
			runArgs = append(runArgs, script.Args...)
			return runArgs, nil
		}
	}
	return nil, errors.New("script doesn't exist in the config")
}

// GetRunEnv returns the env variables for a given script name.
func (c *Config) GetRunEnv(scriptName string) map[string]string {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			if len(script.Env) > 0 {
				return script.Env
			}
			break
		}
	}
	return nil
}

// GetIgnoreOutputOnFail returns the ignoreOutputOnFail parameter for the provided script.
func (c *Config) GetIgnoreOutputOnFail(scriptName string) bool {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			return script.IgnoreOutputOnFail
		}
	}
	return false
}

// GetMaxTimeout returns the max_timeout for a given script name.
func (c *Config) GetMaxTimeout(scriptName string) float64 {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			if script.Timeout.MaxTimeout != nil {
				return *script.Timeout.MaxTimeout
			}
			break
		}
	}
	return 0
}

// GetTimeoutEnforced returns whether or not timeouts should be
// enforced by script_exporter for a particular script name.
func (c *Config) GetTimeoutEnforced(scriptName string) bool {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			if script.Timeout.Enforced != nil {
				return *script.Timeout.Enforced
			}
			break
		}
	}
	return false
}
