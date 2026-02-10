package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// OAuthCredentials holds OAuth 2.1 client credentials loaded from JSON file.
type OAuthCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// TokenStore manages issued access tokens.
type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time // token -> expiry
	expiry time.Duration
}

// NewTokenStore creates a new token store with the given token expiry duration.
func NewTokenStore(expiry time.Duration) *TokenStore {
	return &TokenStore{
		tokens: make(map[string]time.Time),
		expiry: expiry,
	}
}

// IssueToken generates a new access token.
func (ts *TokenStore) IssueToken() (string, time.Duration) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	token := generateToken()
	ts.tokens[token] = time.Now().Add(ts.expiry)
	return token, ts.expiry
}

// ValidateToken checks if a token is valid and not expired.
func (ts *TokenStore) ValidateToken(token string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	expiry, ok := ts.tokens[token]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

// LoadCredentials reads OAuth credentials from a JSON file.
// The file is expected to follow Google OAuth JSON format with a "web" key.
func LoadCredentials(path string) (*OAuthCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	// Try Google OAuth format first: {"web": {"client_id": ..., "client_secret": ...}}
	var googleFormat struct {
		Web struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"web"`
	}

	if err := json.Unmarshal(data, &googleFormat); err == nil && googleFormat.Web.ClientID != "" {
		return &OAuthCredentials{
			ClientID:     googleFormat.Web.ClientID,
			ClientSecret: googleFormat.Web.ClientSecret,
		}, nil
	}

	// Try flat format: {"client_id": ..., "client_secret": ...}
	var creds OAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials JSON: %w", err)
	}

	if creds.ClientID == "" || creds.ClientSecret == "" {
		return nil, fmt.Errorf("credentials file missing client_id or client_secret")
	}

	return &creds, nil
}

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random token: %v", err))
	}
	return hex.EncodeToString(b)
}
