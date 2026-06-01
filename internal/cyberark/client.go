package cyberark

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type Client struct {
	baseURL    string
	httpClient *retryablehttp.Client
	Auth       *AuthManager
}

func NewClient(baseURL string, timeoutSecs int, maxRetries int, skipTLSVerify bool) *Client {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = maxRetries
	retryClient.HTTPClient.Timeout = time.Duration(timeoutSecs) * time.Second
	retryClient.Logger = nil // Disable retryablehttp's internal logging or wrap slog

	// Custom TLS config if needed
	if skipTLSVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		retryClient.HTTPClient.Transport = tr
	}

	c := &Client{
		baseURL:    baseURL,
		httpClient: retryClient,
	}
	return c
}

func (c *Client) newRequest(method, endpoint string, body interface{}) (*retryablehttp.Request, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := retryablehttp.NewRequest(method, c.baseURL+endpoint, bodyBytes)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Auth != nil {
		token := c.Auth.Token()
		if token != "" {
			req.Header.Set("Authorization", token)
		}
	}
	return req, nil
}
