package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

/* Registers a device for a target */
func (a *app) registerDevice(c *fiber.Ctx) error {
	authUser := c.Locals("user")
	if authUser == nil {
		return c.Status(401).JSON(ErrorResponse("unauthorized"))
	}

	target := c.Params("target")
	if target == "" {
		return c.Status(400).JSON(ErrorResponse("target must not be empty"))
	}

	// we first authorize authUser to access target
	authorizedTargets, err := GetAuthTargetsByPrincipal(c.Context(), a.db, authUser.(string))
	if err != nil {
		a.log.Error("something went wrong with getting authorized targets", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	userAuthorized := false
	for _, authorizedTarget := range authorizedTargets {
		if target == authorizedTarget {
			userAuthorized = true
			break
		}
	}

	if !userAuthorized {
		return c.Status(401).JSON(ErrorResponse("user is not authorized for this target"))
	}

	// below this point the user is authorized to access the target

	var registerDeviceInfo RegisterDeviceRequest
	if err := c.BodyParser(&registerDeviceInfo); err != nil {
		return err
	}

	newUUID := uuid.New().String()
	createdDevice, err := a.db.createDevice(c.Context(), authUser.(string), *registerDeviceInfo.HardwareId, newUUID)
	if err != nil {
		a.log.Error("something went wrong with create device database query", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	var deviceId string
	if createdDevice {
		// device was created
		deviceId = newUUID
	} else {
		// device is already registered for the principal / hardware id, get existing
		// FIXME: this logic / database chain is raceable
		deviceId, err = a.db.getDeviceId(c.Context(), authUser.(string), *registerDeviceInfo.HardwareId)
		if err != nil {
			a.log.Error("something went wrong with get device database query", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}
	}

	// sign device token and build response
	signedDeviceToken, err := a.SignDeviceToken(deviceId)
	if err != nil {
		a.log.Error("failed to sign device token", zap.Error(err))
		return c.Status(500).JSON(ErrorResponse("something went wrong"))
	}

	resp := RegisterDeviceResponse{
		DeviceId:    &deviceId,
		DeviceToken: &signedDeviceToken,
	}

	return c.Status(200).JSON(resp)
}
