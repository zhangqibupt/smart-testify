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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SetHelpFunc(nil)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true // 禁用 completion 命令
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
}

func main() {
	cobra.EnableCommandSorting = false

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
