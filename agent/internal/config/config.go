package config

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
)

// TODO: Make error placeholder configurable
// TODO: Send which image, program and/or version is outputing the logs (if known)
const ErrorPlaceholder = "$ERROR"

var SystemPrompt = "You are ErrorDebuggingGPT. Your sole purpose in this world is to help software engineers by diagnosing software system errors and bugs that can occur in any type of computer system. With this role, users will submit a set of log messages to you for analisis and diagnostis. You will serve them with the best of your ability giving as much context and details as possible. Focus specifically on the very last log lines as those are the one triggering the diagnosis event."

var UserPrompt = "The message following the first line containing \"ERROR:\" up until the end of the prompt is a computer error no more and no less. It is your job to try to diagnose and fix what went wrong. Ready?\nERROR:\n" + ErrorPlaceholder

type config struct {
	SystemPrompt string         `yaml:"systemPrompt,omitempty"`
	Prompt       string         `yaml:"prompt,omitempty"`
	Parsers      []parserConfig `yaml:"parsers"`
}

type parserConfig struct {
	Regex    string            `yaml:"regex"`
	Triggers []VariableMatcher `yaml:"triggers,omitempty"`
	Filters  []VariableMatcher `yaml:"filters,omitempty"`
	Excludes []VariableMatcher `yaml:"excludes,omitempty"`
}

type VariableMatcher struct {
	Variable string `yaml:"variable"`
	Regex    string `yaml:"regex"`
}

type ConfigProvider func(log *zap.SugaredLogger, configFile string) (config, error)

func FileConfigProvider(log *zap.SugaredLogger, configFile string) (config, error) {
	// Read configuration
	var config config
	bytes, err := readBytes(configFile)
	if err != nil {
		return config, fmt.Errorf("Failed to open config file: %w", err)
	}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return config, fmt.Errorf("Invalid config: %w", err)
	}
	return config, nil
}

func readBytes(path string) ([]byte, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
