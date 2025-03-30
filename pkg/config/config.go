package config

import (
	"os"
)

// Config holds environment configuration
type Config struct {
	NetplanPath string
}

// LoadConfig reads environment variables and returns a Config struct
func LoadConfig() *Config {
	return &Config{
		NetplanPath: GetEnvVar("NETPLAN_CONFIG_PATH", "/etc/netplan"), // Default to /etc/netplan
	}
}

// GetEnvVar gets an environment variable or returns a default value
func GetEnvVar(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
