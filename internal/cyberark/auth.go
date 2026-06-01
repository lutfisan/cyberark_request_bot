package cyberark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type AuthManager struct {
	client            *Client
	username          string
	password          string
	sessionTTLMinutes int

	token string
	mu    sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

func NewAuthManager(client *Client, username, password string, ttl int) *AuthManager {
	ctx, cancel := context.WithCancel(context.Background())
	am := &AuthManager{
		client:            client,
		username:          username,
		password:          password,
		sessionTTLMinutes: ttl,
		ctx:               ctx,
		cancel:            cancel,
	}
	client.Auth = am // wire auth back to client
	return am
}

func (a *AuthManager) Token() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.token
}

func (a *AuthManager) setToken(t string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = t
}

func (a *AuthManager) Logon() error {
	payload := map[string]interface{}{
		"username":          a.username,
		"password":          a.password,
		"concurrentSession": true,
	}

	req, err := a.client.newRequest("POST", "/PasswordVault/API/auth/CyberArk/Logon", payload)
	if err != nil {
		return fmt.Errorf("failed to create logon request: %w", err)
	}
	
	// Temporary bypass authorization header for logon
	req.Header.Del("Authorization")

	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("logon request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read logon response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("logon failed with status %d: %s", resp.StatusCode, string(body))
	}

	// CyberArk returns the token as a quoted string, strip quotes
	token := strings.Trim(string(body), "\"")
	a.setToken(token)

	slog.Info("cyberark authenticated successfully")
	return nil
}

func (a *AuthManager) Logoff() error {
	a.cancel() // Stop refresh goroutine
	
	req, err := a.client.newRequest("POST", "/PasswordVault/API/auth/Logoff", nil)
	if err != nil {
		return err
	}
	
	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	a.setToken("")
	slog.Info("cyberark session logged off")
	return nil
}

func (a *AuthManager) StartRefreshLoop() {
	// PRD: Refresh TTL-2 minutes before expiry
	refreshInterval := time.Duration(a.sessionTTLMinutes-2) * time.Minute
	if refreshInterval <= 0 {
		refreshInterval = 1 * time.Minute
	}

	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				slog.Debug("refreshing cyberark session token")
				err := a.Logon()
				if err != nil {
					slog.Error("failed to refresh cyberark session", "error", err)
				}
			}
		}
	}()
}

// ReAuth handles the single retry re-authentication flow when receiving a 401
func (a *AuthManager) ReAuth() error {
	slog.Warn("received 401 from cyberark, attempting re-authentication")
	return a.Logon()
}

// Helper to execute requests and handle 401 re-auth
func (a *AuthManager) DoRequestWithReAuth(method, endpoint string, body interface{}, out interface{}) error {
	for attempt := 1; attempt <= 2; attempt++ {
		req, err := a.client.newRequest(method, endpoint, body)
		if err != nil {
			return err
		}

		resp, err := a.client.httpClient.Do(req)
		if err != nil {
			return err
		}
		
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("failed to read response: %w", readErr)
		}

		if resp.StatusCode == 401 {
			if attempt == 1 {
				if err := a.ReAuth(); err != nil {
					return fmt.Errorf("re-auth failed: %w", err)
				}
				continue // retry request
			}
			return fmt.Errorf("unauthorized after re-auth")
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
		}

		if out != nil {
			// Some endpoints might return empty body for success (like 204 or empty 200)
			if len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return fmt.Errorf("failed to decode response: %w, body: %s", err, string(respBody))
				}
			}
		}
		return nil
	}
	return fmt.Errorf("request failed after retries")
}
