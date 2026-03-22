package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/siddharth/card-lens/internal/api"
	"github.com/siddharth/card-lens/internal/assets"
	"github.com/siddharth/card-lens/internal/auth"
	"github.com/siddharth/card-lens/internal/categorizer"
	"github.com/siddharth/card-lens/internal/crypto"
	gmailfetch "github.com/siddharth/card-lens/internal/gmail"
	"github.com/siddharth/card-lens/internal/store"
)

func main() {
	// Load .env file if present
	loadDotenv(".env")

	port := envOr("CARDLENS_PORT", "8080")
	dbPath := envOr("CARDLENS_DB_PATH", "data/cardlens.db")

	// Initialize store
	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer st.Close()

	// Load encryption key
	homeDir, _ := os.UserHomeDir()
	keyPath := envOr("CARDLENS_KEY_PATH", filepath.Join(homeDir, ".cardlens", "secret.key"))
	encKey, err := crypto.LoadOrCreateKey(keyPath)
	if err != nil {
		log.Fatalf("Failed to load encryption key: %v", err)
	}

	// Initialize Google OAuth (optional — works without it)
	var googleAuth *auth.GoogleAuth
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID != "" && clientSecret != "" {
		redirectURL := fmt.Sprintf("http://localhost:%s/auth/google/callback", port)
		googleAuth = auth.NewGoogleAuth(clientID, clientSecret, redirectURL, st, encKey)
		log.Println("Google OAuth configured")
	} else {
		log.Println("Google OAuth not configured (set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET)")
	}

	// Initialize categorizer with custom rules from DB
	customRules, _ := st.ListCategoryRules()
	cat := categorizer.New(customRules)

	// Initialize Gmail fetcher
	fetcher := gmailfetch.NewFetcher(st, cat, "data/statements")

	// Embedded frontend — skip in dev mode (when CARDLENS_DEV is set or Vite is running)
	var frontendFS fs.FS
	if os.Getenv("CARDLENS_DEV") == "" {
		sub, err := fs.Sub(assets.FrontendFS, "dist")
		if err != nil {
			log.Fatalf("Failed to load frontend assets: %v", err)
		}
		frontendFS = sub
	} else {
		log.Println("Dev mode: frontend served by Vite, not embedded")
	}

	// Create server
	server := api.NewServer(st, googleAuth, cat, fetcher, frontendFS)
	router := server.Router()

	addr := ":" + port
	log.Printf("CardLens starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotenv reads a .env file and sets environment variables (won't override existing ones).
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
