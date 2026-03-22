package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/gmail/v1"

	"github.com/siddharth/card-lens/internal/crypto"
	"github.com/siddharth/card-lens/internal/models"
	"github.com/siddharth/card-lens/internal/store"
)

type GoogleAuth struct {
	config *oauth2.Config
	store  *store.Store
	encKey []byte
}

func NewGoogleAuth(clientID, clientSecret, redirectURL string, s *store.Store, encKey []byte) *GoogleAuth {
	return &GoogleAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				gmail.GmailReadonlyScope,
				"https://www.googleapis.com/auth/userinfo.email",
			},
			Endpoint: google.Endpoint,
		},
		store:  s,
		encKey: encKey,
	}
}

// AuthURL generates the Google OAuth2 authorization URL with a random state.
func (g *GoogleAuth) AuthURL() (string, string, error) {
	state := make([]byte, 16)
	if _, err := rand.Read(state); err != nil {
		return "", "", err
	}
	stateStr := hex.EncodeToString(state)
	url := g.config.AuthCodeURL(stateStr,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	return url, stateStr, nil
}

// Exchange exchanges the authorization code for a token and saves it.
func (g *GoogleAuth) Exchange(ctx context.Context, code string) (*models.OAuthAccount, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}

	// Get user email
	client := g.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("get userinfo: %w", err)
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	// Encrypt token
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("marshal token: %w", err)
	}

	encrypted, nonce, err := crypto.Encrypt(tokenJSON, g.encKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}

	acct := &models.OAuthAccount{
		ID:             userInfo.Email,
		Email:          userInfo.Email,
		EncryptedToken: encrypted,
		Nonce:          nonce,
	}

	if err := g.store.SaveOAuthAccount(acct); err != nil {
		return nil, fmt.Errorf("save oauth account: %w", err)
	}

	return acct, nil
}

// GmailService creates an authenticated Gmail service for the given account.
func (g *GoogleAuth) GmailService(ctx context.Context, accountID string) (*gmail.Service, error) {
	acct, err := g.store.GetOAuthAccount(accountID)
	if err != nil {
		return nil, fmt.Errorf("get oauth account: %w", err)
	}
	if acct == nil {
		return nil, fmt.Errorf("oauth account not found: %s", accountID)
	}

	tokenJSON, err := crypto.Decrypt(acct.EncryptedToken, acct.Nonce, g.encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	// Create a token source that auto-refreshes and saves the new token
	ts := g.config.TokenSource(ctx, &token)
	newToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	// If token was refreshed, save it
	if newToken.AccessToken != token.AccessToken {
		tokenJSON, _ := json.Marshal(newToken)
		encrypted, nonce, err := crypto.Encrypt(tokenJSON, g.encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt refreshed token: %w", err)
		}
		acct.EncryptedToken = encrypted
		acct.Nonce = nonce
		if err := g.store.SaveOAuthAccount(acct); err != nil {
			return nil, fmt.Errorf("save refreshed token: %w", err)
		}
	}

	client := oauth2.NewClient(ctx, ts)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}

	return srv, nil
}

// SetStateCookie sets the OAuth state in an HTTP-only cookie.
func SetStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ValidateStateCookie checks the state parameter against the cookie.
func ValidateStateCookie(r *http.Request, state string) bool {
	cookie, err := r.Cookie("oauth_state")
	if err != nil {
		return false
	}
	return cookie.Value == state
}
