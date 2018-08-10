package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config represents the structur of the configuration file
type Config struct {
	Scripts []struct {
		Name   string `yaml:"name"`
		Script string `yaml:"script"`
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
