package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

func main() {
	// Load config.json
	configData, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal("Failed to read config.json:", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatal("Failed to parse config.json:", err)
	}

	// Support @filepath syntax: load system prompt from external file
	if strings.HasPrefix(config.SystemPrompt, "@") {
		filePath := strings.TrimPrefix(config.SystemPrompt, "@")
		promptData, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to read system prompt file %s: %v", filePath, err)
		}
		config.SystemPrompt = string(promptData)
	}

	// Initialize store
	store, err := NewStore("../conversations")
	if err != nil {
		log.Fatal("Failed to initialize store:", err)
	}

	// Start handler
	handler := NewHandler(store, &config)
	handler.ServeHTTP()
}