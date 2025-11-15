package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Config struct with the settings
type Config struct {
	Model        string `json:"model"`
	CopilotToken string `json:"copilot_token"`
}

const modelCopilot = "copilot"
const modelTwinkle = "twinkle"

// configCmd is the root command for configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure settings for smart-testify, such as the model to be used or init Copilot token",
}

// configUseCmd sets the model to be used for test case generation
var configUseCmd = &cobra.Command{
	Use:   "use",
	Short: "Set the model to be used for test case generation (copilot or twinkle), by default it is set to twinkle.",
	Args:  cobra.ExactArgs(1), // Ensure exactly one argument is provided
	Run: func(cmd *cobra.Command, args []string) {
		// Validate and set model
		model := args[0]
		if model != modelCopilot && model != modelTwinkle {
			log.Errorf("Invalid model: %s, must be one of %v", model, []string{
				modelCopilot,
				modelTwinkle,
			})
			return
		}

		// Load config, set model, and save it
		config, err := loadConfig()
		if err != nil {
			log.Errorf("Failed to load config: %v", err)
			return
		}

		config.Model = model
		if err := saveConfig(config); err != nil {
			log.Errorf("Failed to save config: %v", err)
			return
		}

		fmt.Printf("Config updated: model set to %s\n", model)
		if config.Model == modelCopilot && config.CopilotToken == "" {
			log.Warn("Copilot token is not set. You must init the Copilot token manually before using it. Please run `smart-testify config copilot init-token` and follow the instructions.")
		}
	},
}

// configShowCmd displays the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the current configuration settings",
	Run: func(cmd *cobra.Command, args []string) {
		// Load the configuration
		config, err := loadConfig()
		if err != nil {
			log.Errorf("Failed to load config: %v", err)
			return
		}

		// Display the current settings
		log.Println("Current Configuration:")
		log.Printf("\tModel: %s\n", config.Model)
		if config.CopilotToken != "" {
			log.Printf("\tCopilot Token: %s\n", config.CopilotToken)
		} else {
			log.Println("\tCopilot Token: Not set")
		}
	},
}

func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting user home directory: %v\n", err)
		return ""
	}
	return filepath.Join(homeDir, ".smart-testify", "config.json")
}

func saveConfig(config *Config) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(getConfigPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Serialize the config into JSON
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write the config to the file
	if err := ioutil.WriteFile(getConfigPath(), configData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	return nil
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()

	// If the file doesn't exist, create a default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &Config{Model: modelTwinkle} // Set default model to "twinkle"
		if err := saveConfig(defaultConfig); err != nil {
			return nil, err
		}
		return defaultConfig, nil
	}

	// Read the config file
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Deserialize JSON into Config struct
	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return &config, nil
}

func init() {
	// Add the 'use', 'show' subcommands to the 'config' command
	configCmd.AddCommand(configShowCmd) // Add the new 'show' subcommand
	configCmd.AddCommand(configUseCmd)
	configCmd.AddCommand(promptCmd)
	configCmd.AddCommand(copilotCmd)
}

var globalConfig *Config

func getGlobalConfig() *Config {
	if globalConfig == nil {
		var err error
		globalConfig, err = loadConfig()
		if err != nil {
			log.Fatal("Failed to load config: %v", err)
		}
	}
	return globalConfig
}
