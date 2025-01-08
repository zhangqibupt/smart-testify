package main

import (
	"github.com/spf13/cobra"
	"smart-testify/internal/copilot"
	"smart-testify/internal/logger"
)

var (
	log             = logger.GetLogger() // Global logger
	client          = copilot.NewCopilotClient(false)
	pathFlag        string
	modeFlag        string
	functionFilter  string
	ignoreErrorFlag bool
)

// Initialize root command
var rootCmd = &cobra.Command{
	Use:   "smart-testify",
	Short: "A tool to generate test files for Go code using AI",
}

func init() {
	// Add flags for the generate command
	generate.Flags().StringVarP(&pathFlag, "path", "p", "", "Path to the file or directory to generate tests for")
	generate.Flags().StringVarP(&modeFlag, "mode", "m", "overwrite", "Mode for test file generation: overwrite, append, or skip")
	generate.Flags().StringVarP(&functionFilter, "filter", "f", "", "Regex filter for functions to generate tests for")

	rootCmd.PersistentFlags().BoolVarP(&ignoreErrorFlag, "ignore-error", "c", false, "Continue execution even if an error occurs")

	// Add the init-token subcommand
	rootCmd.AddCommand(initTokenCmd)

	// Add the generate subcommand
	rootCmd.AddCommand(generate)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
