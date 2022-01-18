// (C) Copyright 2021 Hewlett Packard Enterprise Development LP

package identitytoken

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	tokenutil "github.com/hewlettpackard/hpegl-provider-lib/pkg/token/token-util"
)

const (
	retryLimit = 3
)

type GenerateTokenInput struct {
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type TokenResponse struct {
	TokenType       string    `json:"token_type"`
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	Expiry          time.Time `json:"expiry"`
	ExpiresIn       int       `json:"expires_in"`
	Scope           string    `json:"scope"`
	AccessTokenOnly bool      `json:"accessTokenOnly"`
}

func GenerateToken(
	ctx context.Context,
	tenantID,
	clientID,
	clientSecret string,
	identityServiceURL string,
	httpClient tokenutil.HttpClient,
) (string, error) {
	params := GenerateTokenInput{
		TenantID:     tenantID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		GrantType:    "client_credentials",
	}

	url := fmt.Sprintf("%s/v1/token", identityServiceURL)

	b, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tokenutil.DoRetries(func() (*http.Response, error) {
		return httpClient.Do(req)
	}, retryLimit)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	err = tokenutil.ManageHTTPErrorCodes(resp, clientID)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var token TokenResponse

	err = json.Unmarshal(body, &token)
	if err != nil {
		return "", err
	}

	return token.AccessToken, nil
}
