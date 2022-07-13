package main

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const oauthURL string = "https://accounts.google.com/o/oauth2/auth"
const tokenURL string = "https://oauth2.googleapis.com/token"

type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectUri  string `json:"redirect_uri"`
	Code         string `json:"code"`
}

func (a *app) oauthLogin(c *fiber.Ctx) error {
	if c.Query("code") == "" {
		// no code was provided, this is a direct visit so redirect to oauth
		queryValues := url.Values{}

		queryValues.Set("response_type", "code")
		queryValues.Set("client_id", a.oauthId)
		queryValues.Set("redirect_uri", a.baseURL+"/api/v1/auth/login")
		queryValues.Set("scope", "openid email")
		queryValues.Set("state", "teststate")

		queryString := queryValues.Encode()
		return c.Redirect(oauthURL + "?" + queryString)
	} else {
		// code was provided, continue with token validation flow
		tokenRequestObj := tokenRequest{
			GrantType:    "authorization_code",
			ClientId:     a.oauthId,
			ClientSecret: a.oauthSecret,
			RedirectUri:  a.baseURL + "/api/v1/auth/login",
			Code:         c.Query("code"),
		}

		tokenRequestJson, err := json.Marshal(tokenRequestObj)
		if err != nil {
			a.log.Error("error creating token request JSON", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		req, err := http.NewRequest("POST", tokenURL, bytes.NewBuffer(tokenRequestJson))
		req.Header.Set("Content-Type", "application/json")
		if err != nil {
			a.log.Error("failed to create token request http request", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		resp, err := a.httpClient.Do(req.WithContext(c.Context()))
		if err != nil {
			a.log.Error("failed to do token request http request", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}
		defer resp.Body.Close()

		respObj := make(map[string]interface{})
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&respObj)
		if err != nil {
			a.log.Error("failed to parse token response", zap.Error(err))
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		// the response is received directly from Google, thus we can trust the token contents
		idToken, ok := respObj["id_token"]
		if !ok {
			a.log.Error("id token not contained in token response")
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		var decodedIdToken []byte
		switch idToken := idToken.(type) {
		case string:
			idTokenSplit := strings.Split(idToken, ".")
			if len(idTokenSplit) != 3 {
				a.log.Error("unable to parse id token")
				return c.Status(500).JSON(ErrorResponse("something went wrong"))
			}
			decodedIdToken, err = b64.RawStdEncoding.DecodeString(idTokenSplit[1])
			if err != nil {
				a.log.Error("unable to decode id token", zap.Error(err))
				return c.Status(500).JSON(ErrorResponse("something went wrong"))
			}
		default:
			a.log.Error("unexpected idToken response type")
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		parsedIdToken := make(map[string]interface{})
		err = json.Unmarshal(decodedIdToken, &parsedIdToken)
		if err != nil {
			a.log.Error("unable to JSON decode id token")
		}
		userEmail, ok := parsedIdToken["email"]
		if !ok {
			a.log.Error("email not contained in decoded token response")
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		var userEmailStr string
		switch userEmail := userEmail.(type) {
		case string:
			userEmailStr = userEmail
		default:
			a.log.Error("unexpected email response type")
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		userToken, err := a.SignToken(userEmailStr)
		if err != nil {
			a.log.Error("could not sign user token")
			return c.Status(500).JSON(ErrorResponse("something went wrong"))
		}

		return c.SendString("signed in as user " + userEmailStr + " with token " + userToken)
	}
}
