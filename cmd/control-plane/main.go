package main

import (
	"database/sql"

	"github.com/caarlos0/env/v6"
	"github.com/gofiber/fiber/v2"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type config struct {
	Production bool   `env:"API_PRODUCTION"`
	JwtSecret  string `evn:"JWT_SECRET"`
}

type app struct {
	log       *zap.Logger
	db        *sql.DB
	jwtSecret string
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
		cfg.JwtSecret = "DEVSECRET"
	}

	db, err := sql.Open("sqlite3", "./rvpn.db")
	if err != nil {
		log.Error("could not connect to db", zap.Error(err))
	}

	a := &app{
		log:       log,
		db:        db,
		jwtSecret: cfg.JwtSecret,
	}

	r := fiber.New()

	r.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("will send web client from here")
	})

	api := r.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/targets", a.targets)

	log.Info("control-plane started")
	r.Listen(":8080")
}
