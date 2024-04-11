package config

import "github.com/spf13/viper"

type Config struct {
	Server   string `yaml:"server" json:"server"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	Path     string `yaml:"path" json:"path"`
}

func Load() (Config, error) {
	var cfg Config
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}
