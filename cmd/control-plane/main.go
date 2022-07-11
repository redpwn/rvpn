package main

import (
	"database/sql"

	"github.com/caarlos0/env/v6"
	"github.com/gofiber/fiber/v2"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type config struct {
	Production  bool   `env:"API_PRODUCTION"`
	JwtSecret   string `env:"JWT_SECRET"`
	PostgresURL string `env:"POSTGRES_URL"`
}

type app struct {
	log       *zap.Logger
	db        *sql.DB
	jwtSecret []byte
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
		cfg.PostgresURL = "postgres://rvpn:rvpn@localhost/rvpn"
	}

	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Error("could not connect to db", zap.Error(err))
	}

	a := &app{
		log:       log,
		db:        db,
		jwtSecret: []byte(cfg.JwtSecret),
	}

	r := fiber.New()

	r.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("will send web client from here")
	})

	api := r.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/target", a.AuthUserMiddleware, a.getTargets)
	v1.Put("/target/:target", a.AuthUserMiddleware, a.createTarget)
	v1.Patch("/target/:target/connection", a.AuthUserMiddleware, a.createConnection)

	log.Info("control-plane started")
	r.Listen(":8080")
}
