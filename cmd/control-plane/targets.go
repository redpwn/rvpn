package main

import (
	"github.com/gofiber/fiber/v2"
)

func (a *app) targets(c *fiber.Ctx) error {
	return c.SendString("TODO")
}
