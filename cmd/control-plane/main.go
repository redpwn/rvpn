package main

import (
	"github.com/caarlos0/env/v6"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type config struct {
	Production bool `env:"API_PRODUCTION"`
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
	} else {
		log, _ = zap.NewDevelopment()
	}

	a := &app{
		log: log,
	}

	r := fiber.New()

	r.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("will send web client from here")
	})

	r.Get("/api/targets", a.targets)

	log.Info("control-plane started")
	r.Listen(":8080")
}
