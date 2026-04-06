package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/app"
	"github.com/tiersum/tiersum/internal/storage"
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

	// Initialize storage (infrastructure layer)
	store, err := storage.New()
	if err != nil {
		logger.Fatal("Failed to initialize storage", zap.Error(err))
	}
	defer store.Close()

	// Wire all dependencies using clean architecture
	// This is the composition root where interfaces are bound to implementations
	deps, err := app.NewDependencies(store.DB.DB, store.DB.Driver(), logger)
	if err != nil {
		logger.Fatal("Failed to wire dependencies", zap.Error(err))
	}

	// Set up Gin router
	if viper.GetString("logging.level") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": Version,
		})
	})

	// API routes - handler depends only on service interfaces
	apiV1 := router.Group("/api/v1")
	deps.Handler.RegisterRoutes(apiV1)

	// MCP routes
	if viper.GetBool("mcp.enabled") {
		// MCP server can also use service interfaces
		// mcpServer := mcp.NewServer(deps.QueryService, logger)
		// router.GET("/mcp/sse", mcpServer.SSEHandler())
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

	logger.Info("Server started", zap.Int("port", port))

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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
