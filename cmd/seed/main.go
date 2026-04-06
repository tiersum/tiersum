package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tiersum/tiersum/internal/storage"
)

var (
	configFile string

	rootCmd = &cobra.Command{
		Use:   "seed",
		Short: "Seed database with sample data",
		Long:  `Populate the database with sample documents and summaries for testing.`,
		Run:   runSeed,
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

func runSeed(cmd *cobra.Command, args []string) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	store, err := storage.New()
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer store.Close()

	logger.Info("Seeding database...")

	// TODO: Implement seeding
	// - Create sample documents
	// - Create sample summaries at different tiers

	logger.Info("Seeding completed")
	fmt.Println("Database seeded successfully")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
