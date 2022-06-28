package main

//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --config=oapi-codegen-config.yaml ../../openapi/api.yml

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v6"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type config struct {
	Production  bool   `env:"API_PRODUCTION"`
	BindAddress string `env:"BIND_ADDRESS"`
	Port        int    `env:"PORT" envDefault:"8080"`
}

type app struct {
	log *zap.Logger
}

func main() {
	cfg := &config{}
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}

	var log *zap.Logger
	if cfg.Production {
		log, _ = zap.NewProduction()
		gin.SetMode(gin.ReleaseMode)
	} else {
		log, _ = zap.NewDevelopment()
	}

	a := &app{
		log: log,
	}

	r := gin.New()
	r.Use(ginzap.Ginzap(log, time.RFC3339, true))
	r.Use(ginzap.RecoveryWithZap(log, true))

	listenAddress := fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.Port)
	log.Info("start", zap.String("address", listenAddress))
	if err := r.Run(listenAddress); err != nil {
		log.Fatal("listen", zap.Error(err))
	}
}
