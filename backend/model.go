package main

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

type ChatRequest struct {
	ConversationID string  `json:"conversation_id"`
	Message        string  `json:"message"`
	Temperature    float64 `json:"temperature"`
	TopP           float64 `json:"top_p"`
	MaxTokens      int     `json:"max_tokens"`
	PresencePenalty float64 `json:"presence_penalty"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
}

type Config struct {
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
	BaseURL      string `json:"base_url"`
}