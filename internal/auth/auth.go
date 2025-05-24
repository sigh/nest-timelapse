package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	oauthScope = "https://www.googleapis.com/auth/sdm.service"
)

type credentials struct {
	Installed struct {
		ClientID                string   `json:"client_id"`
		ProjectID               string   `json:"project_id"`
		AuthURI                 string   `json:"auth_uri"`
		TokenURI                string   `json:"token_uri"`
		AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
		ClientSecret            string   `json:"client_secret"`
		RedirectURIs            []string `json:"redirect_uris"`
	} `json:"installed"`
}

// TokenSource wraps an oauth2.TokenSource and handles token persistence
type TokenSource struct {
	tokenSource oauth2.TokenSource
	tokenFile   string
	config      *oauth2.Config
}

// Token implements oauth2.TokenSource interface
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	token, err := ts.tokenSource.Token()
	if err != nil {
		// Check if the error is due to an expired or invalid token
		if strings.Contains(err.Error(), "invalid_grant") {
			// Remove the expired token file
			if err := os.Remove(ts.tokenFile); err != nil {
				fmt.Printf("Warning: failed to remove expired token file: %v\n", err)
			}

			// Start a new OAuth flow
			newToken, err := handleOAuthFlow(ts.config)
			if err != nil {
				return nil, fmt.Errorf("failed to refresh token through OAuth flow: %w", err)
			}

			// Save the new token
			if err := saveJSON(newToken, ts.tokenFile); err != nil {
				fmt.Printf("Warning: failed to save new token: %v\n", err)
			}

			// Update the token source with the new token
			ts.tokenSource = ts.config.TokenSource(context.Background(), newToken)
			return newToken, nil
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Save the token if it has been refreshed
	if token.AccessToken != "" {
		if err := saveJSON(token, ts.tokenFile); err != nil {
			fmt.Printf("Warning: failed to save refreshed token: %v\n", err)
		}
	}

	return token, nil
}

// loadJSON is a generic function that loads and parses a JSON file into the specified type
func loadJSON[T any](filename string) (*T, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", filename, err)
	}
	return &result, nil
}

// saveJSON is a generic function that saves a value as JSON to the specified file
func saveJSON[T any](data *T, filename string) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON for %s: %w", filename, err)
	}
	return os.WriteFile(filename, jsonData, 0600)
}

// GetCredentials handles OAuth token management, including loading from cache,
// token refresh, and initiating the OAuth flow if needed. Returns a TokenSource
// that will automatically handle token refresh and persistence.
func GetCredentials(tokenFile, credentialsFile string) (*TokenSource, error) {
	creds, err := loadJSON[credentials](credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	config := &oauth2.Config{
		ClientID:     creds.Installed.ClientID,
		ClientSecret: creds.Installed.ClientSecret,
		Scopes:       []string{oauthScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080",
	}

	// Try to load existing token
	var token *oauth2.Token
	if savedToken, err := loadJSON[oauth2.Token](tokenFile); err == nil {
		token = savedToken
	} else {
		// No saved token, start OAuth flow
		token, err = handleOAuthFlow(config)
		if err != nil {
			return nil, fmt.Errorf("failed to complete OAuth flow: %w", err)
		}
		if err := saveJSON(token, tokenFile); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
	}

	// Create a token source that will handle refresh
	tokenSource := config.TokenSource(context.Background(), token)

	return &TokenSource{
		tokenSource: tokenSource,
		tokenFile:   tokenFile,
		config:      config,
	}, nil
}

// handleOAuthFlow implements the OAuth 2.0 authorization code flow, prompting
// the user to authorize the application in their browser
func handleOAuthFlow(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Go to the following link in your browser:\n%v\n", authURL)
	fmt.Print("Enter the authorization code or redirect URL: ")

	var input string
	if _, err := fmt.Scan(&input); err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// Try to extract code from redirect URL if it looks like a URL
	var authCode string
	if strings.HasPrefix(input, "http") {
		redirectURL, err := url.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redirect URL: %w", err)
		}
		authCode = redirectURL.Query().Get("code")
		if authCode == "" {
			return nil, fmt.Errorf("no authorization code found in redirect URL")
		}
	} else {
		authCode = input
	}

	token, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return token, nil
}
