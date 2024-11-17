package config

import (
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server Server `yaml:"server"`
	Redis  Redis  `yaml:"redis"`
	Ninjas Ninjas `yaml:"ninjas"`
}

type Server struct {
	Port string `yaml:"port"`
}

type Redis struct {
	Host      string `yaml:"host"`
	Port      string `yaml:"port"`
	CacheTime int    `yaml:"cachetime"`
}

type Ninjas struct {
	DefinitionURL    string `yaml:"definitionurl"`
	DefAPIKey        string `yaml:"defapikey"`
	WordGeneratorURL string `yaml:"wordgeneratorurl"`
}

func InitConfig(filename string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(filename)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	//setting DefAPIKey from the environment variable
	if apiKey := os.Getenv("DEF_API_KEY"); apiKey != "" {
		cfg.Ninjas.DefAPIKey = apiKey
	}

	return cfg, nil
}
