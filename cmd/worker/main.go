package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
)

var (
	configFile string

	rootCmd = &cobra.Command{
		Use:   "worker",
		Short: "TierSum Background Worker",
		Long:  `Background job processor for document summarization and indexing.`,
		Run:   runWorker,
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
		fmt.Printf("Warning: config file not found: %v\n", err)
	}
}

func runWorker(cmd *cobra.Command, args []string) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize storage
	store, err := storage.New(storage.Config{
		DatabaseURL: viper.GetString("storage.database.dsn"),
	})
	if err != nil {
		logger.Fatal("Failed to initialize storage", zap.Error(err))
	}
	defer store.Close()

	logger.Info("Worker started")

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker loop
	go func() {
		// TODO: Implement job processing loop
		// - Process document summarization
		// - Update index
		logger.Info("Worker loop started")
		<-ctx.Done()
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down worker...")
	cancel()

	logger.Info("Worker exited")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
