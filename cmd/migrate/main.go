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
		Use:   "migrate",
		Short: "Database migration tool",
		Long:  `Run database migrations for TierSum.`,
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "config.yaml", "config file path")

	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(downCmd())
	rootCmd.AddCommand(versionCmd())
}

func initConfig() {
	viper.SetConfigFile(configFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: config file not found: %v\n", err)
	}
}

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Run all pending migrations",
		Run: func(cmd *cobra.Command, args []string) {
			logger, _ := zap.NewProduction()
			defer logger.Sync()

			store, err := storage.New(storage.Config{
				DatabaseURL: viper.GetString("storage.database.dsn"),
			})
			if err != nil {
				logger.Fatal("Failed to connect to database", zap.Error(err))
			}
			defer store.Close()

			// TODO: Implement migration up
			logger.Info("Running migrations up...")
			fmt.Println("Migrations completed")
		},
	}
}

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Rollback last migration",
		Run: func(cmd *cobra.Command, args []string) {
			logger, _ := zap.NewProduction()
			defer logger.Sync()

			store, err := storage.New(storage.Config{
				DatabaseURL: viper.GetString("storage.database.dsn"),
			})
			if err != nil {
				logger.Fatal("Failed to connect to database", zap.Error(err))
			}
			defer store.Close()

			// TODO: Implement migration down
			logger.Info("Rolling back migration...")
			fmt.Println("Rollback completed")
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show current migration version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Current migration version: 0")
		},
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
