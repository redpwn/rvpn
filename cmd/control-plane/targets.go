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

	res, err := a.db.Exec("INSERT INTO targets VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING",
		target, authUser, "10.8.0.0", "/23", "1.1.1.1", "", "", "10.8.0.1", "/23")
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	numRowsAffected, err := res.RowsAffected()
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	if numRowsAffected == 1 {
		return c.Status(200).SendString("successfully created target")
	} else {
		return c.Status(400).JSON(ErrorResponse("target already exists"))
	}
}

/* Returns available connection targets, this is NOT exhaustive */
func (a *app) getTargets(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	rows, err := a.db.Query("SELECT target FROM target_acl WHERE principal=$1", authUser)
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}
	defer rows.Close()

	ret := make(ListTargetsResponse, 0)
	for rows.Next() {
		var target string
		err := rows.Scan(&target)
		if err != nil {
			a.log.Error("failed to parse sql row", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		formattedTarget := struct {
			Name string `json:"name"`
		}{
			Name: target,
		}
		ret = append(ret, formattedTarget)
	}

	// Also return targets where user is the owner
	rows, err = a.db.Query("SELECT name FROM targets WHERE owner=$1", authUser)
	if err != nil {
		a.log.Error("something went wrong with database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}
	defer rows.Close()

	for rows.Next() {
		var target string
		err := rows.Scan(&target)
		if err != nil {
			a.log.Error("failed to parse sql row", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		formattedTarget := struct {
			Name string `json:"name"`
		}{
			Name: target,
		}
		ret = append(ret, formattedTarget)
	}

	return c.Status(200).JSON(ret)
}

/* Creates a new connection using provided name and pubkey */
func (a *app) createConnection(c *fiber.Ctx) error {
	return c.Status(200).SendString("tmp")
}
