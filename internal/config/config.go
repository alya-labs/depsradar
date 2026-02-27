package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	IgnoreIDs      []string `toml:"ignore_ids"`
	IgnorePackages []string `toml:"ignore_packages"`
	CacheTTL       int      `toml:"cache_ttl"`
	Parallel       int      `toml:"parallel"`
}

func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = ".depsradar.toml"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{}, nil
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) IsIgnoredID(id string) bool {
	for _, i := range c.IgnoreIDs {
		if i == id {
			return true
		}
	}
	return false
}

func (c *Config) IsIgnoredPackage(name string) bool {
	for _, p := range c.IgnorePackages {
		if p == name {
			return true
		}
	}
	return false
}
