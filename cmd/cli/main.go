package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile string

	rootCmd = &cobra.Command{
		Use:   "tiersum-cli",
		Short: "TierSum CLI Tools",
		Long:  `Command-line interface for TierSum operations.`,
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "config.yaml", "config file path")

	// Add subcommands
	rootCmd.AddCommand(documentCmd())
	rootCmd.AddCommand(queryCmd())
	rootCmd.AddCommand(indexCmd())
}

func initConfig() {
	viper.SetConfigFile(configFile)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: config file not found: %v\n", err)
	}
}

func documentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doc",
		Short: "Document management commands",
	}
}

func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query the knowledge base",
		Run: func(cmd *cobra.Command, args []string) {
			question, _ := cmd.Flags().GetString("question")
			fmt.Printf("Querying: %s\n", question)
			// TODO: Implement query
		},
	}
	cmd.Flags().StringP("question", "q", "", "Query question")
	cmd.MarkFlagRequired("question")
	return cmd
}

func indexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "index",
		Short: "Index management commands",
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
