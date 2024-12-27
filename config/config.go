package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`

	Server struct {
		Address string `yaml:"address"`
	} `yaml:"server"`

	Deal struct {
		LotusPath  string `yaml:"lotus_path"`
		BoostPath  string `yaml:"boost_path"`
		DealDelay  int    `yaml:"deal_delay"`  // 发单间隔时间（毫秒）
	} `yaml:"deal"`

	Auth struct {
		JWTSecret string `yaml:"jwt_secret"`
		TokenExpireHours int `yaml:"token_expire_hours"`
	} `yaml:"auth"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Database: struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			DBName   string `yaml:"dbname"`
			SSLMode  string `yaml:"sslmode"`
		}{
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Password: "postgres",
			DBName:   "lotus_car",
			SSLMode:  "disable",
		},
		Server: struct {
			Address string `yaml:"address"`
		}{
			Address: ":8080",
		},
		Deal: struct {
			LotusPath  string `yaml:"lotus_path"`
			BoostPath  string `yaml:"boost_path"`
			DealDelay  int    `yaml:"deal_delay"`  // 发单间隔时间（毫秒）
		}{
			LotusPath:  "",
			BoostPath:  "",
			DealDelay:  0,
		},
		Auth: struct {
			JWTSecret string `yaml:"jwt_secret"`
			TokenExpireHours int `yaml:"token_expire_hours"`
		}{
			JWTSecret: "secret",
			TokenExpireHours: 2,
		},
	}
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Start with default configuration
	config := DefaultConfig()

	// If config path is empty, return default config
	if configPath == "" {
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Parse YAML
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return config, nil
}

// SaveConfig saves the configuration to a YAML file
func SaveConfig(config *Config, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %v", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %v", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	return nil
}

// SaveDefaultConfig saves the default configuration to a file
func SaveDefaultConfig(configPath string) error {
	return SaveConfig(DefaultConfig(), configPath)
}
