package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/di"
	"github.com/tiersum/tiersum/internal/storage/db"
	"github.com/tiersum/tiersum/internal/storage/memory"
	"github.com/tiersum/tiersum/pkg/types"
)

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
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "config.yaml", "config file path")
}

func initConfig() {
	viper.SetConfigFile(configFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: config file not found: %v", err)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database
	sqlDB, err := initDB()
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer sqlDB.Close()

	// Run database migrations
	if err := runMigrations(sqlDB, getDriver()); err != nil {
		logger.Error("Failed to run migrations, continuing anyway", zap.Error(err))
		// Continue anyway - the schema might already exist
	}

	// Create memory index for cold documents
	memIndex, err := memory.NewIndex(logger)
	if err != nil {
		logger.Fatal("Failed to create memory index", zap.Error(err))
	}
	defer memIndex.Close()

	// Load cold documents into memory index
	if err := loadColdDocuments(sqlDB, getDriver(), memIndex, logger); err != nil {
		logger.Error("Failed to load cold documents into memory index", zap.Error(err))
		// Continue anyway - the system can still work without cold doc search
	}

	// Wire all dependencies
	deps, err := di.NewDependencies(sqlDB, getDriver(), memIndex, logger)
	if err != nil {
		logger.Fatal("Failed to wire dependencies", zap.Error(err))
	}

	// Start job scheduler
	deps.JobScheduler.Start()
	defer deps.JobScheduler.Stop()

	// Set up Gin router
	if viper.GetString("logging.level") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Static files configuration
	webDir := viper.GetString("server.web_dir")
	if webDir == "" {
		webDir = "./web/dist"
	}
	webDirExists := false
	if _, err := os.Stat(webDir); err == nil {
		webDirExists = true
		logger.Info("Serving static files", zap.String("dir", webDir))
	} else {
		logger.Warn("Web directory not found, API-only mode", zap.String("dir", webDir))
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"version":        Version,
			"cold_doc_count": memIndex.GetDocumentCount(),
		})
	})

	// API routes - handler depends only on service interfaces
	apiV1 := router.Group("/api/v1")
	deps.RESTHandler.RegisterRoutes(apiV1)

	// MCP routes
	if viper.GetBool("mcp.enabled") {
		router.GET("/mcp/sse", deps.MCPServer.SSEHandler())
	}

	// Static files - serve after API routes to avoid conflicts
	if webDirExists {
		// Serve static files from /static path
		router.Static("/static", webDir)
		// Use NoRoute for SPA fallback - serve index.html for non-API routes
		router.NoRoute(func(c *gin.Context) {
			c.File(webDir + "/index.html")
		})
	}

	// Start server
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
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Server started",
		zap.Int("port", port),
		zap.Int("cold_docs_indexed", memIndex.GetDocumentCount()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func initDB() (*sql.DB, error) {
	driver := viper.GetString("storage.database.driver")
	if driver == "" {
		driver = "sqlite3"
	}

	switch driver {
	case "sqlite3", "sqlite":
		dsn := viper.GetString("storage.database.dsn")
		if dsn == "" {
			dsn = "./data/tiersum.db"
		}
		return sql.Open("sqlite3", dsn+"?_journal_mode=WAL")
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func runMigrations(sqlDB *sql.DB, driver string) error {
	migrator := db.NewMigrator(sqlDB, driver)
	return migrator.MigrateUpSimple()
}

func loadColdDocuments(sqlDB *sql.DB, driver string, memIndex *memory.Index, logger *zap.Logger) error {
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

	logger.Info("Loading cold documents into memory index", zap.Int("count", len(coldDocs)))

	// Build index from cold documents
	getEmbedding := func(doc *types.Document) []float32 {
		// Try to parse embedding from document if available
		// For now, generate simple embedding
		return memory.GenerateSimpleEmbedding(doc.Content)
	}

	if err := memIndex.RebuildFromDocuments(ctx, coldDocs, getEmbedding); err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	logger.Info("Cold documents loaded successfully",
		zap.Int("count", len(coldDocs)),
		zap.Duration("duration", time.Since(start)))

	return nil
}

func getDriver() string {
	driver := viper.GetString("storage.database.driver")
	if driver == "" {
		return "sqlite3"
	}
	return driver
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
