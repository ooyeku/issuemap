package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"compress/gzip"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/ooyeku/issuemap/internal/app"
	"github.com/ooyeku/issuemap/internal/app/services"
	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
	"github.com/ooyeku/issuemap/internal/infrastructure/git"
	"github.com/ooyeku/issuemap/internal/infrastructure/storage"
	"github.com/ooyeku/issuemap/web"
)

// Server represents the IssueMap HTTP server
type Server struct {
	httpServer    *http.Server
	port          int
	basePath      string
	issueService  *services.IssueService
	memoryStorage *entities.IssueLinkedList
	syncService   *SyncService
	pidFile       string
	logFile       string
	logFileHandle *os.File
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port     int    `json:"port"`
	BasePath string `json:"base_path"`
	LogLevel string `json:"log_level"`
}

// NewServer creates a new IssueMap server instance
func NewServer(basePath string) (*Server, error) {
	// Initialize services
	issueRepo := storage.NewFileIssueRepository(basePath)
	configRepo := storage.NewFileConfigRepository(basePath)

	// Git client should use the parent directory (actual git repo root)
	gitRepoPath := filepath.Dir(basePath) // Go up one level from .issuemap to git root
	gitRepo, err := git.NewGitClient(gitRepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git client: %w", err)
	}

	issueService := services.NewIssueService(issueRepo, configRepo, gitRepo)
	memoryStorage := entities.NewIssueLinkedList()

	// Find available port
	port, err := findAvailablePort(app.ServerPortRangeStart, app.ServerPortRangeEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	server := &Server{
		port:          port,
		basePath:      basePath,
		issueService:  issueService,
		memoryStorage: memoryStorage,
		pidFile:       filepath.Join(basePath, app.ServerPIDFile),
		logFile:       filepath.Join(basePath, app.ServerLogFile),
	}

	// Create sync service
	syncService, err := NewSyncService(server, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync service: %w", err)
	}
	server.syncService = syncService

	return server, nil
}

// Start begins the HTTP server
func (s *Server) Start() error {
	// Load issues into memory
	if err := s.loadIssuesIntoMemory(); err != nil {
		return fmt.Errorf("failed to load issues into memory: %w", err)
	}

	// Start sync service to watch for file changes
	if err := s.syncService.Start(); err != nil {
		return fmt.Errorf("failed to start sync service: %w", err)
	}

	// Setup router and middleware
	router := s.setupRouter()

	// Configure server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Write PID file
	if err := s.writePIDFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Setup logging
	s.setupLogging()

	log.Printf("IssueMap server starting on port %d", s.port)
	log.Printf("API endpoints available at http://localhost:%d%s", s.port, app.APIBasePath)

	// Start server
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return fmt.Errorf("server is not running")
	}

	log.Println("Shutting down server...")

	// Stop sync service first
	if s.syncService != nil {
		s.syncService.Stop()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(app.ServerShutdownTimeout)*time.Second)
	defer cancel()

	// Shutdown server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	// Close log file if open
	if s.logFileHandle != nil {
		if err := s.logFileHandle.Close(); err != nil {
			log.Printf("Warning: failed to close log file: %v", err)
		}
		s.logFileHandle = nil
	}

	// Remove PID file
	if err := os.Remove(s.pidFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove PID file: %v", err)
	}

	log.Println("Server stopped successfully")
	return nil
}

// GetPort returns the server port
func (s *Server) GetPort() int {
	return s.port
}

// IsRunning checks if the server is currently running
func (s *Server) IsRunning() bool {
	if _, err := os.Stat(s.pidFile); os.IsNotExist(err) {
		return false
	}

	// Read PID and check if process is running
	pidBytes, err := ioutil.ReadFile(s.pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return false
	}

	// Check if process exists (simple check)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Try to signal the process (non-destructive check)
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// gzipResponseWriter wraps http.ResponseWriter to support gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (grw *gzipResponseWriter) WriteHeader(statusCode int) {
	// Remove Content-Length when compressing
	grw.ResponseWriter.Header().Del("Content-Length")
	grw.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	grw.ResponseWriter.WriteHeader(statusCode)
}

func (grw *gzipResponseWriter) Write(b []byte) (int, error) {
	return grw.writer.Write(b)
}

func acceptsGzip(r *http.Request) bool {
	enc := r.Header.Get("Accept-Encoding")
	return strings.Contains(enc, "gzip")
}

func isCompressiblePath(path string) bool {
	return strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".txt") || path == "/"
}

// staticWithCachingAndGzip applies basic caching headers and gzip compression for static assets
func staticWithCachingAndGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		// Cache policy
		if strings.HasSuffix(p, ".html") || p == "/" || p == "" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if strings.HasSuffix(p, ".css") || strings.HasSuffix(p, ".js") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		w.Header().Add("Vary", "Accept-Encoding")

		// Gzip compression
		if acceptsGzip(r) && isCompressiblePath(p) {
			gz := gzip.NewWriter(w)
			defer gz.Close()
			grw := &gzipResponseWriter{ResponseWriter: w, writer: gz}
			next.ServeHTTP(grw, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// setupRouter configures the HTTP router with all endpoints
func (s *Server) setupRouter() http.Handler {
	router := mux.NewRouter()

	// API subrouter
	api := router.PathPrefix(app.APIBasePath).Subrouter()

	// Middleware
	api.Use(s.loggingMiddleware)
	api.Use(s.errorHandlingMiddleware)

	// Health check
	api.HandleFunc("/health", s.healthHandler).Methods("GET")

	// Server info
	api.HandleFunc("/info", s.infoHandler).Methods("GET")

	// Issue endpoints
	issues := api.PathPrefix("/issues").Subrouter()
	issues.HandleFunc("", s.listIssuesHandler).Methods("GET")
	issues.HandleFunc("", s.createIssueHandler).Methods("POST")
	issues.HandleFunc("/{id}", s.getIssueHandler).Methods("GET")
	issues.HandleFunc("/{id}", s.updateIssueHandler).Methods("PUT")
	issues.HandleFunc("/{id}", s.deleteIssueHandler).Methods("DELETE")
	issues.HandleFunc("/{id}/close", s.closeIssueHandler).Methods("POST")
	issues.HandleFunc("/{id}/reopen", s.reopenIssueHandler).Methods("POST")
	issues.HandleFunc("/{id}/assign", s.assignIssueHandler).Methods("POST")
	issues.HandleFunc("/{id}/comments", s.addCommentHandler).Methods("POST")

	// History endpoints
	history := api.PathPrefix("/history").Subrouter()
	history.HandleFunc("", s.listHistoryHandler).Methods("GET")
	history.HandleFunc("/{id}", s.getIssueHistoryHandler).Methods("GET")

	// Statistics endpoints
	stats := api.PathPrefix("/stats").Subrouter()
	stats.HandleFunc("", s.getStatsHandler).Methods("GET")
	stats.HandleFunc("/summary", s.getSummaryHandler).Methods("GET")

	// Git endpoints
	gitApi := api.PathPrefix("/git").Subrouter()
	gitApi.HandleFunc("/commit/{hash}/diff", s.getCommitDiffHandler).Methods("GET")

	// Static web UI (serve embedded assets at root) with caching and gzip
	uiFS := http.FileServer(http.FS(web.Static))
	router.PathPrefix("/").Handler(staticWithCachingAndGzip(uiFS))

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	return c.Handler(router)
}

// loadIssuesIntoMemory loads all issues from disk into the linked list
func (s *Server) loadIssuesIntoMemory() error {
	ctx := context.Background()

	// Use an empty filter to get all issues
	filter := repositories.IssueFilter{}
	issueList, err := s.issueService.ListIssues(ctx, filter)
	if err != nil {
		log.Printf("Warning: Failed to load issues from disk: %v", err)
		s.memoryStorage.Clear()
		return nil // Don't fail server startup if we can't load issues
	}

	// Clear existing memory and load fresh data
	s.memoryStorage.Clear()

	// Add each issue to memory storage
	for i := range issueList.Issues {
		issueCopy := issueList.Issues[i]
		s.memoryStorage.Add(&issueCopy)
	}

	log.Printf("Loaded %d issues into memory from disk", len(issueList.Issues))
	return nil
}

// writePIDFile writes the current process ID to a file
func (s *Server) writePIDFile() error {
	pid := os.Getpid()
	return ioutil.WriteFile(s.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// setupLogging configures server logging
func (s *Server) setupLogging() {
	logFile, err := os.OpenFile(s.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Warning: failed to open log file: %v", err)
		return
	}

	s.logFileHandle = logFile
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// findAvailablePort finds an available port in the given range
func findAvailablePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", start, end)
}

// Middleware functions

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("%s %s %v", r.Method, r.URL.Path, duration)
	})
}

func (s *Server) errorHandlingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic in handler: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Response helpers

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]interface{}{
		"error":   true,
		"message": message,
		"code":    statusCode,
	}
	s.jsonResponse(w, response, statusCode)
}
