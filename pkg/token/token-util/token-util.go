// (C) Copyright 2021 Hewlett Packard Enterprise Development LP

package tokenutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/errors"
	"gopkg.in/square/go-jose.v2"
)

// Token a jwt token format
type Token struct {
	Issuer           string `json:"iss"`
	Subject          string `json:"sub"`
	Expiry           int64  `json:"exp"`
	IssuedAt         int64  `json:"iat"`
	Type             string `json:"typ"`
	Nonce            string `json:"nonce"`
	AtHash           string `json:"at_hash"`
	ClientID         string `json:"cid,omitempty"`
	UserID           string `json:"uid,omitempty"`
	TenantID         string `json:"tenantId"`
	AuthorizedParty  string `json:"azp"`
	KeycloakClientID string `json:"clientId"`
	IsHPE            bool   `json:"isHPE"`
}

//nolint:stylecheck,golint,revive
type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DecodeAccessToken decodes the accessToken offline
//nolint:gocritic
func DecodeAccessToken(rawToken string) (Token, error) {
	_, err := jose.ParseSigned(rawToken)
	if err != nil {
		return Token{}, fmt.Errorf("oidc: malformed jwt: %w", err)
	}

	// Throw out tokens with invalid claims before trying to verify the token. This lets
	// us do cheap checks before possibly re-syncing keys.
	payload, err := parseJWT(rawToken)
	if err != nil {
		log.Fatal(fmt.Sprintf("oidc: malformed jwt: %v", err))

		return Token{}, fmt.Errorf("oidc: malformed jwt: %w", err)
	}
	var token Token
	if err := json.Unmarshal(payload, &token); err != nil {
		log.Fatal(fmt.Sprintf("oidc: failed to unmarshal claims: %v", err))

		return Token{}, fmt.Errorf("oidc: failed to unmarshal claims: %w", err)
	}

	if token.UserID != "" {
		// User token
		token.Subject = "users/" + token.UserID
	} else if token.ClientID != "" || token.KeycloakClientID != "" {
		token.Subject = "clients/" + token.Subject
	} else {
		// TODO This is just so that Keycloak tokens continue to work. Remove after keycloak is gone
		token.Subject = "users/" + token.Subject
	}

	return token, nil
}

func DoRetries(call func() (*http.Response, error), retries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	for {
		resp, err = call()
		if err != nil {
			return nil, err
		}

		if !isStatusRetryable(resp.StatusCode) || retries == 0 {
			break
		}

		time.Sleep(3 * time.Second)
		retries--
	}

	return resp, nil
}

func ManageHTTPErrorCodes(resp *http.Response, clientID string) error {
	var err error

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		msg := fmt.Sprintf("Bad request: %v", string(body))
		err = errors.MakeErrBadRequest(errors.ErrorResponse{
			ErrorCode: "ErrGenerateTokenBadRequest",
			Message:   msg,
		})

		return err
	case http.StatusForbidden:
		err = errors.MakeErrForbidden(clientID)

		return err
	case http.StatusUnauthorized:
		err = errors.MakeErrUnauthorized(clientID)

		return err
	default:
		msg := fmt.Sprintf("Unexpected status code %v", resp.StatusCode)
		err = errors.MakeErrInternalError(errors.ErrorResponse{
			ErrorCode: "ErrGenerateTokenUnexpectedResponseCode",
			Message:   msg,
		})

		return err
	}
}

func isStatusRetryable(statusCode int) bool {
	if statusCode == http.StatusInternalServerError || statusCode == http.StatusTooManyRequests {
		return true
	}

	return false
}

func parseJWT(p string) ([]byte, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("oidc: malformed jwt, expected 3 parts got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: malformed jwt payload: %w", err)
	}

	return payload, nil
}
