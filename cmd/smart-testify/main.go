package main

import (
	"github.com/spf13/cobra"
	"smart-testify/internal/logger"
)

var (
	log = logger.GetLogger() // Global logger
)

// Initialize root command
var rootCmd = &cobra.Command{
	Use:   "smart-testify",
	Short: "A tool to generate unit tests for Go file using AI",
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(generateCmd)
}

func main() {
	cobra.EnableCommandSorting = false

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
