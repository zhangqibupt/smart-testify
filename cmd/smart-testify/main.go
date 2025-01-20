package main

import (
	"github.com/spf13/cobra"
	"smart-testify/internal/logger"
)

var (
	log             = logger.GetLogger() // Global logger
	pathFlag        string
	modeFlag        string
	functionFilter  string
	ignoreErrorFlag bool
	granularity     string
)

// Initialize root command
var rootCmd = &cobra.Command{
	Use:   "smart-testify",
	Short: "A tool to generate unit tests for Go file using AI",
}

func init() {
	// Add flags for the generate command
	rootCmd.PersistentFlags().BoolVarP(&ignoreErrorFlag, "ignore-error", "c", false, "Continue handling next file if error occurs")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(generateCmd)
}

func main() {
	cobra.EnableCommandSorting = false

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
