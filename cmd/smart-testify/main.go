package main

import (
	"github.com/spf13/cobra"
	"log"
	"smart-testify/internal/copilot"
)

var (
	client   = copilot.NewCopilotClient() // Global Copilot client
	pathFlag string
	modeFlag string
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
