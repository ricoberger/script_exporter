package config

import (
	"reflect"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	type testCase struct {
		config         Config
		expectedErrors int
		description    string
	}
	testCases := []testCase{
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:   "unittest",
						Script: "unit test script",
					},
				},
			},
			expectedErrors: 0,
			description:    "Valid config with script",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    "unittest",
						Command: "unit test command",
					},
				},
			},
			expectedErrors: 0,
			description:    "Valid config with command",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    "unittest",
						Command: "unit test command",
						Args:    []string{"arg1", "arg2"},
					},
				},
			},
			expectedErrors: 0,
			description:    "Valid config with command + args",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    "unittest",
						Script:  "unit test script",
						Command: "unit test command",
						Args:    []string{"arg1", "arg2"},
					},
				},
			},
			expectedErrors: 1,
			description:    "script + args + command is rejected",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    "unittest",
						Script:  "unit test script",
						Command: "unit test command",
					},
				},
			},
			expectedErrors: 1,
			description:    "script + command is rejected",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:   "unittest",
						Script: "unit test script",
						Args:   []string{"arg1", "arg2"},
					},
				},
			},
			expectedErrors: 1,
			description:    "script + args is rejected",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    "unittest1",
						Script:  "unit test script",
						Command: "unit test command",
						Args:    []string{"arg1", "arg2"},
					},
					{
						Name:    "unittest2",
						Script:  "unit test script",
						Command: "unit test command",
						Args:    []string{"arg1", "arg2"},
					},
				},
			},
			expectedErrors: 2,
			description:    "script + command + args is rejected, multiple times",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name: "unittest1",
						Args: []string{"arg1", "arg2"},
					},
				},
			},
			expectedErrors: 1,
			description:    "Neither script nor command",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			errs := ValidateConfig(&tc.config)
			if len(errs) != tc.expectedErrors {
				t.Errorf("Expected %d errors, got %d", tc.expectedErrors, len(errs))
			}
		})
	}
}

func TestConfig_GetRunArgs(t *testing.T) {
	type testCase struct {
		config      Config
		expected    []string
		err         bool
		description string
	}
	scriptName := "unittest"
	testCases := []testCase{
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:   scriptName,
						Script: "script a b",
					},
				},
			},
			expected:    []string{"script", "a", "b"},
			description: "Valid config with script",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    scriptName,
						Command: "test command",
					},
				},
			},
			expected:    []string{"test command"},
			description: "Valid config with command",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{
					{
						Name:    scriptName,
						Command: "test command",
						Args:    []string{"arg1", "arg2"},
					},
				},
			},
			expected:    []string{"test command", "arg1", "arg2"},
			description: "Valid config with command and args",
		},
		{
			config: Config{
				Scripts: []ScriptConfig{},
			},
			err:         true,
			description: "Missing script",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			runArgs, err := GetRunArgs(&tc.config, scriptName)
			if err != nil && !tc.err {
				t.Errorf("Got unexpected error %v", err)
			}
			if err == nil && tc.err {
				t.Error("Expected error")
			}
			if !reflect.DeepEqual(tc.expected, runArgs) {
				t.Errorf("Expected runArgs %v, got %v", tc.expected, runArgs)
			}
		})
	}
}
