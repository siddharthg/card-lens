package api

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/siddharth/card-lens/internal/auth"
	"github.com/siddharth/card-lens/internal/categorizer"
	gmailfetch "github.com/siddharth/card-lens/internal/gmail"
	"github.com/siddharth/card-lens/internal/store"
)

type Server struct {
	store       *store.Store
	auth        *auth.GoogleAuth
	categorizer *categorizer.Categorizer
	fetcher     *gmailfetch.Fetcher
	frontendFS  fs.FS

	syncMu     sync.Mutex
	syncStatus SyncStatus
}

type SyncStatus struct {
	Status    string `json:"status"`
	LastSync  string `json:"last_sync,omitempty"`
	Message   string `json:"message,omitempty"`
	Processed int    `json:"processed,omitempty"`
}

func NewServer(s *store.Store, a *auth.GoogleAuth, c *categorizer.Categorizer, f *gmailfetch.Fetcher, frontendFS fs.FS) *Server {
	return &Server{
		store:       s,
		auth:        a,
		categorizer: c,
		fetcher:     f,
		frontendFS:  frontendFS,
		syncStatus:  SyncStatus{Status: "idle"},
	}
}

func (s *Server) Router() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Auth routes (not under /api because Google OAuth redirect)
	r.Get("/auth/google/login", s.handleGoogleLogin)
	r.Get("/auth/google/callback", s.handleGoogleCallback)

	r.Route("/api", func(r chi.Router) {
		// Auth
		r.Get("/auth/accounts", s.handleListAccounts)
		r.Delete("/auth/accounts/{id}", s.handleDeleteAccount)

		// Cards
		r.Get("/cards", s.handleListCards)
		r.Post("/cards", s.handleCreateCard)
		r.Get("/cards/{id}", s.handleGetCard)
		r.Put("/cards/{id}", s.handleUpdateCard)
		r.Delete("/cards/{id}", s.handleDeleteCard)

		// Transactions
		r.Get("/transactions", s.handleListTransactions)
		r.Put("/transactions/{id}", s.handleUpdateTransaction)
		r.Put("/transactions/bulk", s.handleBulkUpdateTransactions)
		r.Get("/transactions/export", s.handleExportTransactions)

		// Statements
		r.Get("/statements", s.handleListStatements)
		r.Get("/statements/{id}/pdf", s.handleDownloadStatementPDF)
		r.Get("/statements/{id}/text", s.handleStatementText)
		r.Get("/statements/{id}/transactions", s.handleStatementTransactions)
		r.Post("/statements/upload", s.handleUploadStatement)
		r.Post("/statements/upload-bulk", s.handleBulkUploadStatements)
		r.Delete("/statements/{id}", s.handleDeleteStatement)

		// Analytics
		r.Get("/analytics/summary", s.handleAnalyticsSummary)
		r.Get("/analytics/trends", s.handleAnalyticsTrends)
		r.Get("/analytics/calendar", s.handleAnalyticsCalendar)
		r.Get("/analytics/recurring", s.handleAnalyticsRecurring)

		// Sync
		r.Post("/sync", s.handleSync)
		r.Post("/sync/{accountId}", s.handleSyncAccount)
		r.Get("/sync/status", s.handleSyncStatus)
		r.Get("/sync/errors", s.handleListSyncErrors)
		r.Delete("/sync/errors/{id}", s.handleDeleteSyncError)

		// Settings
		r.Get("/settings", s.handleGetSettings)
		r.Put("/settings", s.handleUpdateSettings)

		// Categories & Rules
		r.Get("/categories", s.handleListCategories)
		r.Get("/merchants/rules", s.handleListMerchantRules)
		r.Post("/merchants/rules", s.handleCreateMerchantRule)
		r.Put("/merchants/rules/{id}", s.handleUpdateMerchantRule)
		r.Delete("/merchants/rules/{id}", s.handleDeleteMerchantRule)
	})

	// SPA fallback — serve frontend
	s.serveFrontend(r)

	return r
}

func (s *Server) serveFrontend(r chi.Router) {
	if s.frontendFS == nil {
		return
	}

	fileServer := http.FileServer(http.FS(s.frontendFS))

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file
		f, err := s.frontendFS.Open(path)
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/index.html"
		} else {
			f.Close()
		}

		fileServer.ServeHTTP(w, r)
	})
}
