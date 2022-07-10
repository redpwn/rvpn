package main

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

/* Signs a token with specified user and returns it */
func (a *app) SignToken(user string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user": user,
	})

	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

/* Returns user for which token is signed */
func (a *app) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// validate that alg is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}

		return a.jwtSecret, nil
	})
	if err != nil {
		return "", errors.New("failed to validate token")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// validated token
		return claims["user"].(string), nil
	} else {
		// failed to validate token
		return "", errors.New("failed to validate token")
	}
}

/* Middlware to perform authentication
 * If authentication successful, adds user to locals
 * If authentication fails, user locals should be nil
 */
func (a *app) AuthUserMiddleware(c *fiber.Ctx) error {
	reqHeaders := c.GetReqHeaders()
	authHeader, ok := reqHeaders["Authorization"]
	if ok && strings.HasPrefix(authHeader, "Bearer ") {
		authToken := authHeader[7:]
		user, err := a.ValidateToken(authToken)
		if err == nil {
			c.Locals("user", user)
		}
	}
	return c.Next()
}
