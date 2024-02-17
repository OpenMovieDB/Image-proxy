package config

import (
	"github.com/caarlos0/env/v8"
	"log/slog"
)

type Config struct {
	S3Region    string `env:"S3_REGION"`
	S3Bucket    string `env:"S3_BUCKET,required"`
	S3AccessKey string `env:"S3_ACCESS_KEY,required"`
	S3SecretKey string `env:"S3_SECRET_KEY,required"`
	S3Endpoint  string `env:"S3_ENDPOINT,required"`
}

func New() *Config {
	conf := &Config{}

	if err := env.Parse(conf); err != nil {
		slog.Error(err.Error())

		panic("Failed to parse config")
	}

	return conf
}
