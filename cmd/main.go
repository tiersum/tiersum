package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/di"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/storage/coldindex"
	"github.com/tiersum/tiersum/internal/storage/db"
	"github.com/tiersum/tiersum/pkg/types"
)

//go:embed web/*
var webFS embed.FS

var (
	Version    = "dev"
	configFile string

	rootCmd = &cobra.Command{
		Use:   "tiersum",
		Short: "TierSum - Hierarchical Summary Knowledge Base",
		Long:  `A RAG-free document retrieval system powered by multi-layer abstraction.`,
		Run:   runServer,
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "configs/config.yaml", "config file path")
}

func initConfig() {
	viper.SetConfigFile(configFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: config file not found: %v", err)
	}

	// Expand environment variables in config values (e.g., ${VAR} -> value)
	expandEnvVars()
}

// expandEnvVars replaces ${VAR} or $VAR patterns with actual environment variable values
func expandEnvVars() {
	for _, key := range viper.AllKeys() {
		val := viper.GetString(key)
		expanded := os.ExpandEnv(val)
		if expanded != val {
			viper.Set(key, expanded)
		}
	}
}

// ServerDeps holds all server dependencies
type ServerDeps struct {
	Logger       *zap.Logger
	SQLDB        *sql.DB
	ColdIndex    *coldindex.Index
	TextEmbedder coldindex.IColdTextEmbedder
	DI           *di.Dependencies
}

// setupServerDeps initializes all server dependencies
func setupServerDeps() (*ServerDeps, error) {
	logger, _ := zap.NewProduction()

	// Initialize database
	sqlDB, err := initDB()
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Run database migrations
	if err := runMigrations(sqlDB, getDriver()); err != nil {
		logger.Error("Failed to run migrations, continuing anyway", zap.Error(err))
	}

	// Cold document index (Bleve + HNSW, in-process)
	coldIndex, err := coldindex.NewIndex(logger)
	if err != nil {
		sqlDB.Close()
		logger.Fatal("Failed to create cold index", zap.Error(err))
	}
	coldIndex.SetColdChapterMaxTokens(viper.GetInt("cold_index.markdown.chapter_max_tokens"))
	coldindex.SetColdMarkdownSlidingStrideTokens(viper.GetInt("cold_index.markdown.sliding_stride_tokens"))
	coldIndex.SetColdSearchRecall(
		viper.GetInt("cold_index.search.branch_recall_multiplier"),
		viper.GetInt("cold_index.search.branch_recall_floor"),
		viper.GetInt("cold_index.search.branch_recall_ceiling"),
	)

	textEmbedder, err := coldindex.NewTextEmbedderFromViper(logger)
	if err != nil {
		sqlDB.Close()
		coldIndex.Close()
		return nil, fmt.Errorf("text embedder: %w", err)
	}
	coldIndex.SetTextEmbedder(textEmbedder)

	// Load cold documents into cold index
	if err := loadColdDocuments(sqlDB, getDriver(), coldIndex, logger); err != nil {
		logger.Error("Failed to load cold documents into cold index", zap.Error(err))
	}

	// Wire all dependencies
	deps, err := di.NewDependencies(sqlDB, getDriver(), coldIndex, logger)
	if err != nil {
		_ = textEmbedder.Close()
		sqlDB.Close()
		coldIndex.Close()
		logger.Fatal("Failed to wire dependencies", zap.Error(err))
	}

	return &ServerDeps{
		Logger:       logger,
		SQLDB:        sqlDB,
		ColdIndex:    coldIndex,
		TextEmbedder: textEmbedder,
		DI:           deps,
	}, nil
}

// setupRouter configures the Gin router with all routes
func setupRouter(deps *ServerDeps) *gin.Engine {
	// Set up Gin router
	if viper.GetString("logging.level") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Register health check
	registerHealthRoute(router, deps)

	// Register API routes
	registerAPIRoutes(router, deps)

	// Register MCP routes
	registerMCPRoutes(router, deps)

	// Register static file routes
	registerStaticRoutes(router, deps)

	return router
}

// registerHealthRoute registers the health check endpoint
func registerHealthRoute(r *gin.Engine, deps *ServerDeps) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"version":        Version,
			"cold_doc_count": deps.ColdIndex.ApproxEntries(),
		})
	})
}

// registerAPIRoutes registers all API v1 routes
func registerAPIRoutes(r *gin.Engine, deps *ServerDeps) {
	apiV1 := r.Group("/api/v1")
	apiV1.Use(api.APIKeyAuth(viper.GetString("security.api_key")))
	deps.DI.RESTHandler.RegisterRoutes(apiV1)
}

// registerMCPRoutes registers MCP protocol routes
func registerMCPRoutes(r *gin.Engine, deps *ServerDeps) {
	if viper.GetBool("mcp.enabled") {
		r.GET("/mcp/sse", deps.DI.MCPServer.SSEHandler())
		r.POST("/mcp/message", deps.DI.MCPServer.MessageHandler())
	}
}

// registerStaticRoutes registers static file serving routes (embedded web assets)
func registerStaticRoutes(r *gin.Engine, deps *ServerDeps) {
	deps.Logger.Info("Registering static file routes with embedded files")

	// Use embedded static files via NoRoute handler
	// Must be registered after all API routes
	r.NoRoute(StaticFileServer())
}

// StaticFileServer returns a gin handler for serving embedded static files.
func StaticFileServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/health") ||
			strings.HasPrefix(path, "/mcp/") {
			c.Next()
			return
		}

		if path == "/" {
			path = "/index.html"
		}

		filePath := "web" + path
		data, err := webFS.ReadFile(filePath)
		if err != nil {
			data, err = webFS.ReadFile("web/index.html")
			if err != nil {
				c.Next()
				return
			}
			filePath = "web/index.html"
		}

		contentType := staticContentType(filePath)
		c.Header("Content-Type", contentType)
		c.Data(http.StatusOK, contentType, data)
		c.Abort()
	}
}

func staticContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// WebFS returns the embedded filesystem for the web UI assets.
func WebFS() fs.FS {
	return webFS
}

// startServer starts the HTTP server with graceful shutdown
func startServer(router *gin.Engine, deps *ServerDeps) error {
	port := viper.GetInt("server.port")
	if port == 0 {
		port = 8080
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			deps.Logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	deps.Logger.Info("Server started",
		zap.Int("port", port),
		zap.Int("cold_docs_indexed", deps.ColdIndex.ApproxEntries()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	deps.Logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		deps.Logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	deps.Logger.Info("Server exited")
	return nil
}

func runServer(cmd *cobra.Command, args []string) {
	// Setup dependencies
	deps, err := setupServerDeps()
	if err != nil {
		log.Fatal(err)
	}
	defer deps.SQLDB.Close()
	defer deps.ColdIndex.Close()
	if deps.TextEmbedder != nil {
		defer func() { _ = deps.TextEmbedder.Close() }()
	}

	// Start job scheduler
	deps.DI.JobScheduler.Start()
	defer deps.DI.JobScheduler.Stop()

	promoteCtx, promoteCancel := context.WithCancel(context.Background())
	defer promoteCancel()
	job.StartPromoteQueueConsumer(promoteCtx, deps.DI.DocumentMaintenance, deps.Logger)

	// Setup and start server
	router := setupRouter(deps)
	if err := startServer(router, deps); err != nil {
		deps.Logger.Fatal("Server error", zap.Error(err))
	}
}

func initDB() (*sql.DB, error) {
	driver := normalizeDBDriver(viper.GetString("storage.database.driver"))

	switch driver {
	case "sqlite3":
		dsn := viper.GetString("storage.database.dsn")
		if dsn == "" {
			dsn = "./data/tiersum.db"
		}
		return sql.Open("sqlite3", dsn+"?_journal_mode=WAL")
	case "postgres":
		dsn := viper.GetString("storage.database.dsn")
		if dsn == "" {
			return nil, fmt.Errorf("storage.database.dsn is required for postgres driver")
		}
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, err
		}
		if mc := viper.GetInt("storage.database.max_connections"); mc > 0 {
			db.SetMaxOpenConns(mc)
		}
		if mic := viper.GetInt("storage.database.min_connections"); mic > 0 {
			db.SetMaxIdleConns(mic)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func runMigrations(sqlDB *sql.DB, driver string) error {
	migrator := db.NewMigrator(sqlDB, driver)
	return migrator.MigrateUpSimple()
}

func loadColdDocuments(sqlDB *sql.DB, driver string, coldIndex *coldindex.Index, logger *zap.Logger) error {
	start := time.Now()

	// Create a simple cache for repository
	cacheStore := &noopCache{}

	// Create document repository
	docRepo := db.NewDocumentRepo(sqlDB, driver, cacheStore)

	// Query all cold documents
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	coldDocs, err := docRepo.ListByStatus(ctx, types.DocStatusCold, 0)
	if err != nil {
		return fmt.Errorf("failed to list cold documents: %w", err)
	}

	if len(coldDocs) == 0 {
		logger.Info("No cold documents to load")
		return nil
	}

	logger.Info("Loading cold documents into cold index", zap.Int("count", len(coldDocs)))

	if err := coldIndex.RebuildFromDocuments(ctx, coldDocs); err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	logger.Info("Cold documents loaded successfully",
		zap.Int("count", len(coldDocs)),
		zap.Duration("duration", time.Since(start)))

	return nil
}

func getDriver() string {
	return normalizeDBDriver(viper.GetString("storage.database.driver"))
}

func normalizeDBDriver(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "", "sqlite", "sqlite3":
		return "sqlite3"
	case "postgres", "postgresql":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(d))
	}
}

// noopCache is a no-op cache implementation for use during startup
type noopCache struct{}

func (n *noopCache) Get(key string) (interface{}, bool) { return nil, false }
func (n *noopCache) Set(key string, value interface{})  {}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
