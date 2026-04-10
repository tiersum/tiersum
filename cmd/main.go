package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
// ServerDeps 持有所有服务器依赖
type ServerDeps struct {
	Logger       *zap.Logger
	SQLDB        *sql.DB
	MemIndex     *memory.Index
	DI           *di.Dependencies
	WebDir       string
	WebDirExists bool
}

// setupServerDeps initializes all server dependencies
// setupServerDeps 初始化所有服务器依赖
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

	// Create memory index for cold documents
	memIndex, err := memory.NewIndex(logger)
	if err != nil {
		sqlDB.Close()
		logger.Fatal("Failed to create memory index", zap.Error(err))
	}

	// Load cold documents into memory index
	if err := loadColdDocuments(sqlDB, getDriver(), memIndex, logger); err != nil {
		logger.Error("Failed to load cold documents into memory index", zap.Error(err))
	}

	// Wire all dependencies
	deps, err := di.NewDependencies(sqlDB, getDriver(), memIndex, logger)
	if err != nil {
		sqlDB.Close()
		memIndex.Close()
		logger.Fatal("Failed to wire dependencies", zap.Error(err))
	}

	// Static files configuration
	webDir := viper.GetString("server.web_dir")
	if webDir == "" {
		webDir = "./web/dist"
	}
	
	// Try to resolve relative paths against multiple possible locations
	if _, err := os.Stat(webDir); err != nil {
		// If relative path doesn't work, try to find web/dist in common locations
		possiblePaths := []string{
			"./web/dist",
			"../web/dist",
			"../../web/dist",
		}
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				webDir = path
				break
			}
		}
	}
	webDirExists := false
	if _, err := os.Stat(webDir); err == nil {
		webDirExists = true
		logger.Info("Serving static files", zap.String("dir", webDir))
	} else {
		logger.Warn("Web directory not found, API-only mode", zap.String("dir", webDir))
	}

	return &ServerDeps{
		Logger:       logger,
		SQLDB:        sqlDB,
		MemIndex:     memIndex,
		DI:           deps,
		WebDir:       webDir,
		WebDirExists: webDirExists,
	}, nil
}

// setupRouter configures the Gin router with all routes
// setupRouter 配置 Gin 路由，注册所有路由
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
// registerHealthRoute 注册健康检查端点
func registerHealthRoute(r *gin.Engine, deps *ServerDeps) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"version":        Version,
			"cold_doc_count": deps.MemIndex.GetDocumentCount(),
		})
	})
}

// registerAPIRoutes registers all API v1 routes
// registerAPIRoutes 注册所有 API v1 路由
func registerAPIRoutes(r *gin.Engine, deps *ServerDeps) {
	apiV1 := r.Group("/api/v1")
	deps.DI.RESTHandler.RegisterRoutes(apiV1)
}

// registerMCPRoutes registers MCP protocol routes
// registerMCPRoutes 注册 MCP 协议路由
func registerMCPRoutes(r *gin.Engine, deps *ServerDeps) {
	if viper.GetBool("mcp.enabled") {
		r.GET("/mcp/sse", deps.DI.MCPServer.SSEHandler())
	}
}

// registerStaticRoutes registers static file serving routes
// registerStaticRoutes 注册静态文件服务路由
func registerStaticRoutes(r *gin.Engine, deps *ServerDeps) {
	if !deps.WebDirExists {
		deps.Logger.Warn("Web directory not found, static routes disabled", zap.String("web_dir", deps.WebDir))
		return
	}
	deps.Logger.Info("Registering static file routes", zap.String("web_dir", deps.WebDir))

	// SPA fallback: serve static files or index.html
	// Must be registered after all API routes
	r.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		deps.Logger.Debug("NoRoute handler triggered", zap.String("path", requestPath))

		// Don't serve static files for API routes - let API 404s be 404s
		if strings.HasPrefix(requestPath, "/api/") || strings.HasPrefix(requestPath, "/health") || strings.HasPrefix(requestPath, "/mcp/") {
			c.Status(http.StatusNotFound)
			return
		}

		// Clean the path (remove leading slash)
		cleanPath := strings.TrimPrefix(requestPath, "/")
		if cleanPath == "" {
			deps.Logger.Debug("Serving index.html for root path")
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.File(deps.WebDir + "/index.html")
			return
		}

		fullPath := deps.WebDir + "/" + cleanPath

		// 1. If it's a file, serve it directly
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			deps.Logger.Debug("Serving file", zap.String("path", fullPath))
			c.File(fullPath)
			return
		}

		// 2. Try .html file (e.g., /tags -> tags.html)
		htmlPath := fullPath + ".html"
		if _, err := os.Stat(htmlPath); err == nil {
			deps.Logger.Debug("Serving HTML file", zap.String("path", htmlPath))
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.File(htmlPath)
			return
		}

		// 3. If it's a directory with index.html, serve that
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			indexPath := fullPath + "/index.html"
			if _, err := os.Stat(indexPath); err == nil {
				deps.Logger.Debug("Serving index.html from directory", zap.String("path", indexPath))
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.File(indexPath)
				return
			}
		}

		// 4. Handle dynamic routes like /docs/[id] - serve the parent index.html for client-side routing
		// For Next.js static export with dynamic routes, we need to serve the index.html 
		// from the nearest parent directory that has one
		pathParts := strings.Split(cleanPath, "/")
		for i := len(pathParts); i > 0; i-- {
			parentPath := strings.Join(pathParts[:i], "/")
			parentIndex := deps.WebDir + "/" + parentPath + "/index.html"
			if _, err := os.Stat(parentIndex); err == nil {
				deps.Logger.Debug("Serving parent index.html for dynamic route", 
					zap.String("path", requestPath), 
					zap.String("parent_index", parentIndex))
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.File(parentIndex)
				return
			}
		}

		// 5. Fall back to root index.html for client-side routing
		deps.Logger.Debug("Falling back to index.html for SPA route", zap.String("original_path", requestPath))
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.File(deps.WebDir + "/index.html")
	})
}

// startServer starts the HTTP server with graceful shutdown
// startServer 启动 HTTP 服务器，支持优雅关闭
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
		zap.Int("cold_docs_indexed", deps.MemIndex.GetDocumentCount()))

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
	defer deps.MemIndex.Close()

	// Start job scheduler
	deps.DI.JobScheduler.Start()
	defer deps.DI.JobScheduler.Stop()

	// Setup and start server
	router := setupRouter(deps)
	if err := startServer(router, deps); err != nil {
		deps.Logger.Fatal("Server error", zap.Error(err))
	}
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
