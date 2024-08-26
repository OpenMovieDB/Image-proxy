package config

import (
	"github.com/caarlos0/env/v8"
)

type Dragonfly struct {
	Host     string `env:"DRAGONFLY_HOST,notEmpty"`
	Port     int    `env:"DRAGONFLY_PORT,required"`
	DB       int    `env:"DRAGONFLY_DB,required"`
	Password string `env:"DRAGONFLY_PASSWORD"`
}

func NewDragonflyConfig() *Dragonfly {
	conf := &Dragonfly{}

	if err := env.Parse(conf); err != nil {
		panic(err)
	}

	return conf
}
