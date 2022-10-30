package main

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

/* Create a target */
func (a *app) createTarget(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	target := c.Params("target")
	if target == "" {
		return c.Status(400).JSON(ErrorResponse("target must not be empty"))
	}

	createdTarget, err := a.db.createTarget(c.Context(), target, authUser.(string), "10.8.0.0", "/23", "1.1.1.1", "", "", "", "10.8.0.1", "/23", "")
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	if createdTarget {
		return c.Status(200).SendString("successfully created target")
	} else {
		return c.Status(400).JSON(ErrorResponse("target already exists"))
	}
}

/* Returns available connection targets */
func (a *app) getTargets(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	authorizedTargets, err := GetAuthTargetsByPrincipal(c.Context(), a.db, authUser.(string))
	if err != nil {
		a.log.Error("something went wrong with getting authorized targets", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	ret := make(ListTargetsResponse, 0, len(authorizedTargets))
	for i := range authorizedTargets {
		formattedTarget := struct {
			Name string `json:"name"`
		}{
			Name: authorizedTargets[i],
		}
		ret = append(ret, formattedTarget)
	}

	return c.Status(200).JSON(ret)
}
