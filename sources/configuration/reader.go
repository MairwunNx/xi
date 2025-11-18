package configuration

import (
	"fmt"
	"os"
	"regexp"
	"ximanager/sources/tracing"

	"gopkg.in/yaml.v3"
)

// NewYaml reads the configuration from the specified file path (default: config.yaml)
// and returns a Config struct. It supports environment variable expansion.
func NewYaml(log *tracing.Logger) (*Config, error) {
	defer tracing.ProfilePoint(log, "Configuration loaded", "configuration.load")()

	filePath := os.Getenv("CONFIG_PATH")
	if filePath == "" {
		filePath = "config.yaml"
	}

	log.I("reading configuration", "path", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		log.E("failed to read configuration file", tracing.InnerError, err, "path", filePath)
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	expandedContent := expandEnv(string(content))

	var config Config
	if err := yaml.Unmarshal([]byte(expandedContent), &config); err != nil {
		log.E("failed to parse configuration file", tracing.InnerError, err, "path", filePath)
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	return &config, nil
}

// expandEnv replaces ${VAR} or ${VAR:default} with environment values.
func expandEnv(content string) string {
	// Regex to match ${VAR} or ${VAR:default}
	re := regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?::([^}]*))?\}`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		matches := re.FindStringSubmatch(match)
		key := matches[1]
		defaultValue := ""
		if len(matches) > 2 {
			defaultValue = matches[2]
		}

		value, exists := os.LookupEnv(key)
		if !exists {
			// If no default value is provided (defaultValue is empty and not strictly ":"),
			// we might want to keep the empty string or keep the variable.
			// Logic here: if it has a default (even empty), use it.
			// If simply ${VAR}, and not set, replace with empty string.
			return defaultValue
		}
		return value
	})
}