package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type config struct {
	Production  bool   `env:"API_PRODUCTION"`
	JwtSecret   string `env:"JWT_SECRET"`
	PostgresURL string `env:"POSTGRES_URL"`
	BaseURL     string `env:"BASE_URL"`
	OauthId     string `env:"OAUTH_ID"`
	OauthSecret string `env:"OAUTH_SECRET"`
}

type app struct {
	log         *zap.Logger
	db          *sql.DB
	jwtSecret   []byte
	httpClient  *http.Client
	baseURL     string
	oauthId     string
	oauthSecret string
}

func updateWsMiddlware(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
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
		cfg.BaseURL = "http://localhost:8080"
		cfg.OauthId = "320141988326-bebrlvs8pch1re7bbtulgml7m2n4agcc.apps.googleusercontent.com"
		cfg.OauthSecret = "--"
	}

	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Error("could not connect to db", zap.Error(err))
	}

	client := http.Client{}
	client.Timeout = time.Second * 5

	a := &app{
		log:         log,
		db:          db,
		jwtSecret:   []byte(cfg.JwtSecret),
		httpClient:  &client,
		baseURL:     cfg.BaseURL,
		oauthId:     cfg.OauthId,
		oauthSecret: cfg.OauthSecret,
	}

	r := fiber.New()

	r.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("will send web client from here")
	})

	api := r.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/target", a.AuthUserMiddleware, a.getTargets)
	v1.Put("/target/:target", a.AuthUserMiddleware, a.createTarget)
	v1.Post("/target/:target/connect", a.AuthUserMiddleware, a.createConnection)

	v1.Get("/target/:target/serve", updateWsMiddlware, a.createConnection)

	v1.Get("/auth/login", a.oauthLogin)

	log.Info("control-plane started")
	r.Listen(":8080")
}
