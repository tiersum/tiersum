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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/tiersum/tiersum/internal/api"
	"github.com/tiersum/tiersum/internal/config"
	"github.com/tiersum/tiersum/internal/di"
	"github.com/tiersum/tiersum/internal/job"
	"github.com/tiersum/tiersum/internal/storage/coldindex"
	"github.com/tiersum/tiersum/internal/storage/db"
	"github.com/tiersum/tiersum/internal/telemetry"
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

	// When features.web_ui is omitted, keep the embedded UI enabled (backward compatible with older configs).
	viper.SetDefault("features.web_ui", true)

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
	Logger         *zap.Logger
	SQLDB          *sql.DB
	ColdIndex      *coldindex.Index
	TextEmbedder   coldindex.IColdTextEmbedder
	DI             *di.Dependencies
	TracerShutdown func(context.Context) error
}

// setupServerDeps initializes all server dependencies
func setupServerDeps() (*ServerDeps, error) {
	logger, err := newLoggerFromViper()
	if err != nil {
		log.Printf("logging: init from config failed (%v), falling back to zap production", err)
		logger, _ = zap.NewProduction()
	}

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
	deps, err := di.NewDependencies(sqlDB, getDriver(), coldIndex, logger, Version)
	if err != nil {
		_ = textEmbedder.Close()
		sqlDB.Close()
		coldIndex.Close()
		logger.Fatal("Failed to wire dependencies", zap.Error(err))
	}

	tracerShutdown := telemetry.InitGlobalTracer(deps.OtelSpans, logger)

	return &ServerDeps{
		Logger:         logger,
		SQLDB:          sqlDB,
		ColdIndex:      coldIndex,
		TextEmbedder:   textEmbedder,
		DI:             deps,
		TracerShutdown: tracerShutdown,
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
	applyCORS(router, deps.Logger)
	if maxBody := config.ServerMaxRequestBodyBytes(); maxBody > 0 {
		router.MaxMultipartMemory = maxBody
		router.Use(maxRequestBodyMiddleware(maxBody))
	}

	// Root probes: never gated by program auth (same idea as /metrics).
	registerPublicInfraRoutes(router, deps)

	var traceMw gin.HandlerFunc
	if telemetry.GlobalTracerActive() {
		traceMw = api.NewTracingMiddleware()
	}
	registerAPIRoutes(router, deps, traceMw)
	registerBFFRoutes(router, deps, traceMw, Version)

	// Register MCP routes
	registerMCPRoutes(router, deps, traceMw)

	// Embedded Vue UI (only when enabled; REST/MCP/health always available)
	if viper.GetBool("features.web_ui") {
		registerStaticRoutes(router, deps)
	} else {
		deps.Logger.Info("Web UI disabled (features.web_ui=false); serving /health, /metrics, /api/v1/*, /bff/v1/* (auth required when initialized), and optional /mcp/* only")
		registerWebUIDisabledRoot(router)
	}

	return router
}

// registerWebUIDisabledRoot avoids serving the SPA when features.web_ui is false.
func registerWebUIDisabledRoot(r *gin.Engine) {
	const msg = "web UI is disabled (features.web_ui: false); use /api/v1 or /bff/v1, /health, /metrics, or enable the flag in config"
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
	})
	r.HEAD("/", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})
}

// registerPublicInfraRoutes registers GET /health and GET /metrics on the engine root.
// Neither is wrapped by program/browser auth middleware so probes and scrapers stay public.
func registerPublicInfraRoutes(r *gin.Engine, deps *ServerDeps) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"version":        Version,
			"cold_doc_count": deps.ColdIndex.ApproxEntries(),
		})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// registerAPIRoutes registers programmatic REST under /api/v1 (DB API keys + scope).
func registerAPIRoutes(r *gin.Engine, deps *ServerDeps, traceMw gin.HandlerFunc) {
	apiV1 := r.Group("/api/v1")
	apiV1.Use(api.ProgramAuthMiddleware(deps.DI.AuthService))
	deps.DI.RESTHandler.RegisterRoutes(apiV1, traceMw)
}

// registerBFFRoutes registers the same REST surface under /bff/v1 for the embedded UI (browser session).
func registerBFFRoutes(r *gin.Engine, deps *ServerDeps, traceMw gin.HandlerFunc, serverVersion string) {
	authBFF := api.NewAuthBFFHandler(deps.DI.AuthService, deps.DI.AdminConfigView, deps.Logger, serverVersion)
	bff := r.Group("/bff/v1")
	authBFF.RegisterPublicRoutes(bff)
	bff.Use(api.BFFSessionMiddleware(deps.DI.AuthService))
	deps.DI.RESTHandler.RegisterRoutes(bff, traceMw)
	me := bff.Group("/me")
	authBFF.RegisterMeRoutes(me)
	admin := bff.Group("/admin", api.BFFRequireAdmin())
	authBFF.RegisterAdminRoutes(admin)
}

// registerMCPRoutes registers MCP protocol routes (same OpenTelemetry Gin middleware as core REST when enabled).
func registerMCPRoutes(r *gin.Engine, deps *ServerDeps, traceMw gin.HandlerFunc) {
	if !viper.GetBool("mcp.enabled") {
		return
	}
	sse := deps.DI.MCPServer.SSEHandler()
	msg := deps.DI.MCPServer.MessageHandler()
	if traceMw != nil {
		r.GET("/mcp/sse", traceMw, sse)
		r.POST("/mcp/message", traceMw, msg)
	} else {
		r.GET("/mcp/sse", sse)
		r.POST("/mcp/message", msg)
	}
}

// registerStaticRoutes registers static file serving routes (embedded web assets)
func registerStaticRoutes(r *gin.Engine, deps *ServerDeps) {
	deps.Logger.Info("Registering static file routes with embedded files")

	h := StaticFileServer()
	// Explicit root + SPA fallback: some setups only hit NoRoute for unknown paths, not for "/".
	r.GET("/", h)
	r.HEAD("/", h)
	r.NoRoute(h)
}

// StaticFileServer returns a gin handler for serving embedded static files.
func StaticFileServer() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/bff/") ||
			path == "/metrics" ||
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
	addr := config.HTTPListenAddr(port)

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}
	if t := config.ServerReadTimeout(); t > 0 {
		srv.ReadTimeout = t
		srv.WriteTimeout = t
		srv.IdleTimeout = t
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			deps.Logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	deps.Logger.Info("Server started",
		zap.String("addr", addr),
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
	defer func() { _ = deps.Logger.Sync() }()
	defer deps.SQLDB.Close()
	defer func() {
		if deps.TracerShutdown == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := deps.TracerShutdown(ctx); err != nil {
			deps.Logger.Warn("Tracer shutdown", zap.Error(err))
		}
	}()
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
	job.StartHotIngestQueueConsumer(promoteCtx, deps.DI.HotIngestProcessor, deps.Logger)

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
	ctx := context.Background()
	migErr := migrator.MigrateUpSimple()
	if err := migrator.EnsureOtelSpansTable(ctx); err != nil {
		return fmt.Errorf("ensure otel_spans: %w", err)
	}
	if err := migrator.EnsureAuthTables(ctx); err != nil {
		return fmt.Errorf("ensure auth tables: %w", err)
	}
	return migErr
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

func parseZapLevel(s string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic", "panic", "fatal":
		return zapcore.InfoLevel
	default:
		return zapcore.InfoLevel
	}
}

// newLoggerFromViper builds zap from logging.* viper keys (level, format, output, file_path, caller).
// When output is "file", creates parent directories and appends to logging.file_path.
func newLoggerFromViper() (*zap.Logger, error) {
	level := parseZapLevel(viper.GetString("logging.level"))
	fmtStr := strings.ToLower(strings.TrimSpace(viper.GetString("logging.format")))
	if fmtStr == "" {
		fmtStr = "console"
	}
	outKind := strings.ToLower(strings.TrimSpace(viper.GetString("logging.output")))
	if outKind == "" {
		outKind = "stdout"
	}

	var enc zapcore.Encoder
	if fmtStr == "json" {
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		enc = zapcore.NewJSONEncoder(encCfg)
	} else {
		encCfg := zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		enc = zapcore.NewConsoleEncoder(encCfg)
	}

	var ws zapcore.WriteSyncer
	switch outKind {
	case "file":
		p := strings.TrimSpace(viper.GetString("logging.file_path"))
		if p == "" {
			p = "./logs/tiersum.log"
		}
		dir := filepath.Dir(p)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("logging.file_path mkdir: %w", err)
			}
		}
		f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("logging.file_path open: %w", err)
		}
		ws = zapcore.AddSync(f)
	default:
		ws = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(enc, ws, zap.NewAtomicLevelAt(level))
	opts := []zap.Option{zap.WithCaller(viper.GetBool("logging.caller"))}
	return zap.New(core, opts...), nil
}

// applyCORS registers Access-Control headers when server.cors.enabled is true.
func applyCORS(r *gin.Engine, logger *zap.Logger) {
	if !viper.GetBool("server.cors.enabled") {
		return
	}
	origins := viper.GetStringSlice("server.cors.allowed_origins")
	if len(origins) == 0 {
		logger.Warn("server.cors.enabled is true but server.cors.allowed_origins is empty; allowing any origin")
		origins = []string{"*"}
	}
	r.Use(func(c *gin.Context) {
		reqOrigin := strings.TrimSpace(c.GetHeader("Origin"))
		allow := ""
		for _, o := range origins {
			o = strings.TrimSpace(o)
			if o == "*" {
				allow = "*"
				break
			}
			if o != "" && o == reqOrigin {
				allow = reqOrigin
				break
			}
		}
		if allow != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allow)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept, Authorization, X-API-Key")
			c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
			if allow != "*" {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
}

func maxRequestBodyMiddleware(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
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
