package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	URL      string `mapstructure:"url"`
	Token    string `mapstructure:"token"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	APIKey   string `mapstructure:"api_key"` // Community X-API-Key
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Look in ~/.config/portainer-tui/ and current dir
	home, _ := os.UserHomeDir()
	viper.AddConfigPath(filepath.Join(home, ".config", "portainer-tui"))
	viper.AddConfigPath(".")

	// Env var overrides: PORTAINER_URL, PORTAINER_TOKEN, etc.
	viper.SetEnvPrefix("PORTAINER")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("url", "http://localhost:9000")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// WriteDefault creates a default config file in ~/.config/portainer-tui/
func WriteDefault(url, token string) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "portainer-tui")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")
	content := fmt.Sprintf("url: %s\ntoken: %s\n", url, token)
	return os.WriteFile(path, []byte(content), 0600)
}
