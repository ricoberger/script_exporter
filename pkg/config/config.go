package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Making MaxTimeout a pointer to a float64 allows us to tell the
// difference between an explicit 0 and an unconfigured setting.
// Ditto for Enforced.
type timeout struct {
	MaxTimeout *float64 `yaml:"max_timeout"`
	Enforced   *bool    `yaml:"enforced"`
}

// Config represents the structur of the configuration file
type Config struct {
	TLS struct {
		Active bool   `yaml:"active"`
		Crt    string `yaml:"crt"`
		Key    string `yaml:"key"`
	} `yaml:"tls"`

	BasicAuth struct {
		Active   bool   `yaml:"active"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"basicAuth"`

	BearerAuth struct {
		Active     bool   `yaml:"active"`
		SigningKey string `yaml:"signingKey"`
	} `yaml:"bearerAuth"`

	Timeouts timeout `yaml:"timeouts"`

	Scripts []struct {
		Name    string `yaml:"name"`
		Script  string `yaml:"script"`
		Timeout timeout
	} `yaml:"scripts"`
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

// GetScript returns a script for a given name
func (c *Config) GetScript(scriptName string) string {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			return script.Script
		}
	}

	return ""
}

// GetMaxTimeout returns the max_timeout for a given script name,
// which comes from either the script's specific setting (if set)
// or the global setting.
func (c *Config) GetMaxTimeout(scriptName string) float64 {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			if script.Timeout.MaxTimeout != nil {
				return *script.Timeout.MaxTimeout
			}
			break
		}
	}
	if c.Timeouts.MaxTimeout != nil {
		return *c.Timeouts.MaxTimeout
	}
	return 0
}

// GetTimeoutEnforced returns whether or not timeouts should be
// enforced by script_exporter for a particular script.
func (c *Config) GetTimeoutEnforced(scriptName string) bool {
	for _, script := range c.Scripts {
		if script.Name == scriptName {
			if script.Timeout.Enforced != nil {
				return *script.Timeout.Enforced
			}
			break
		}
	}
	if c.Timeouts.Enforced != nil {
		return *c.Timeouts.Enforced
	}
	return false
}
