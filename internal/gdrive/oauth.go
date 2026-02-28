package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"
)

// tokenCachePath is the default location for the cached OAuth token.
var tokenCachePath = filepath.Join(os.Getenv("HOME"), ".localfiles-index", "gdrive-token.json")

// Scopes required for Drive read + Sheets read.
var oauthScopes = []string{
	drive.DriveReadonlyScope,
	sheets.SpreadsheetsReadonlyScope,
}

// LoadOAuthConfig reads a Google OAuth credentials JSON file and returns an oauth2.Config.
// Supports "installed", "web", and flat credential formats.
func LoadOAuthConfig(credentialsPath string) (*oauth2.Config, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	// Try standard Google format first (installed/web wrapper)
	config, err := google.ConfigFromJSON(data, oauthScopes...)
	if err == nil {
		return config, nil
	}

	// Fallback: try flat JSON with client_id/client_secret
	var flat struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if jsonErr := json.Unmarshal(data, &flat); jsonErr == nil && flat.ClientID != "" {
		return &oauth2.Config{
			ClientID:     flat.ClientID,
			ClientSecret: flat.ClientSecret,
			Scopes:       oauthScopes,
			Endpoint:     google.Endpoint,
			RedirectURL:  "http://localhost",
		}, nil
	}

	return nil, fmt.Errorf("unsupported credentials format: %w", err)
}

// GetToken loads a cached token or triggers the browser OAuth flow.
func GetToken(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Try loading cached token
	token, err := loadCachedToken()
	if err == nil {
		// Check if token needs refresh
		if token.Valid() {
			return token, nil
		}
		// Try to refresh
		src := config.TokenSource(ctx, token)
		newToken, err := src.Token()
		if err == nil {
			if err := saveCachedToken(newToken); err != nil {
				slog.Warn("failed to cache refreshed token", "error", err)
			}
			return newToken, nil
		}
		slog.Warn("token refresh failed, re-authenticating", "error", err)
	}

	// No valid cached token — run browser auth flow
	token, err = browserAuthFlow(ctx, config)
	if err != nil {
		return nil, err
	}

	if err := saveCachedToken(token); err != nil {
		slog.Warn("failed to cache token", "error", err)
	}

	return token, nil
}

// GetCachedToken loads a cached token without triggering browser auth.
// Returns an error if no valid cached token exists.
func GetCachedToken(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	token, err := loadCachedToken()
	if err != nil {
		return nil, fmt.Errorf("no cached Google Drive token (run CLI auth first): %w", err)
	}

	if token.Valid() {
		return token, nil
	}

	// Try to refresh
	src := config.TokenSource(ctx, token)
	newToken, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("cached token expired and refresh failed (run CLI auth again): %w", err)
	}

	if err := saveCachedToken(newToken); err != nil {
		slog.Warn("failed to cache refreshed token", "error", err)
	}

	return newToken, nil
}

// browserAuthFlow starts a local HTTP server, opens the browser for OAuth consent,
// and waits for the callback with the authorization code.
func browserAuthFlow(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("finding free port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	config.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errCh <- fmt.Errorf("OAuth error: %s: %s", errParam, r.URL.Query().Get("error_description"))
			fmt.Fprintf(w, "<html><body><h2>Authentication failed</h2><p>%s</p></body></html>", errParam)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			fmt.Fprint(w, "<html><body><h2>No code received</h2></body></html>")
			return
		}
		codeCh <- code
		fmt.Fprint(w, "<html><body><h2>Authentication successful!</h2><p>You can close this tab.</p></body></html>")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()
	defer server.Shutdown(context.Background())

	// Open browser
	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Opening browser for Google authentication...\n")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n", authURL)
	openBrowser(authURL)

	// Wait for callback with timeout
	select {
	case code := <-codeCh:
		token, err := config.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("exchanging code for token: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("OAuth flow timed out after 5 minutes")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func loadCachedToken() (*oauth2.Token, error) {
	data, err := os.ReadFile(tokenCachePath)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func saveCachedToken(token *oauth2.Token) error {
	dir := filepath.Dir(tokenCachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return os.WriteFile(tokenCachePath, data, 0600)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Start()
}
