package main

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

/* Returns available connection targets, this is NOT exhaustive
 */
func (a *app) getTargets(c *fiber.Ctx) error {
	fmt.Println("out", c.Locals("user"))
	st, er := a.SignToken("jimmytoken")
	fmt.Println(st, er)
	return c.SendString("TODO")
}
