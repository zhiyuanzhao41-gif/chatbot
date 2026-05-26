package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	store   *Store
	config  *Config
	apiKey  string
	baseURL string
}

func NewHandler(store *Store, config *Config) *Handler {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable is required")
	}

	return &Handler{
		store:   store,
		config:  config,
		apiKey:  apiKey,
		baseURL: baseURL,
	}
}

func (h *Handler) listConversations(c *gin.Context) {
	conversations := h.store.ListConversations()
	c.JSON(http.StatusOK, conversations)
}

func (h *Handler) createConversation(c *gin.Context) {
	conv := h.store.CreateConversation()
	c.JSON(http.StatusCreated, conv)
}

func (h *Handler) getConversation(c *gin.Context) {
	id := c.Param("id")
	conv, err := h.store.GetConversation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conv)
}

func (h *Handler) deleteConversation(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteConversation(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) updateSystemPrompt(c *gin.Context) {
	var body struct {
		SystemPrompt string `json:"system_prompt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.config.SystemPrompt = body.SystemPrompt
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *Handler) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"model":         h.config.Model,
		"system_prompt": h.config.SystemPrompt,
		"base_url":      h.baseURL,
	})
}

func (h *Handler) chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing messages
	messages, err := h.store.GetMessages(req.ConversationID)
	if err != nil {
		// Create new conversation if not found
		conv := h.store.CreateConversation()
		req.ConversationID = conv.ID
		messages = conv.Messages
	}

	// Build messages array with system prompt
	var allMessages []Message
	if h.config.SystemPrompt != "" {
		allMessages = append(allMessages, Message{Role: "system", Content: h.config.SystemPrompt})
	}
	allMessages = append(allMessages, messages...)
	allMessages = append(allMessages, Message{Role: "user", Content: req.Message})

	// Build request to DeepSeek
	deepseekReq := map[string]interface{}{
		"model":            h.config.Model,
		"messages":         allMessages,
		"stream":           true,
		"temperature":      req.Temperature,
		"top_p":            req.TopP,
		"max_tokens":       req.MaxTokens,
		"presence_penalty": req.PresencePenalty,
		"frequency_penalty": req.FrequencyPenalty,
	}

	reqBody, err := json.Marshal(deepseekReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create request to DeepSeek
	httpReq, err := http.NewRequest("POST", h.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Build assistant content incrementally while streaming
	var assistantContent strings.Builder
	saveDone := func() {
		h.store.AddMessage(req.ConversationID, "user", req.Message)
		h.store.AddMessage(req.ConversationID, "assistant", assistantContent.String())
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		content := strings.TrimPrefix(line, "data: ")
		if content == "[DONE]" {
			c.Writer.WriteString("data: [DONE]\n\n")
			c.Writer.Flush()
			saveDone()
			return
		}

		// Extract delta content incrementally
		var delta struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(content), &delta); err == nil {
			if len(delta.Choices) > 0 {
				assistantContent.WriteString(delta.Choices[0].Delta.Content)
			}
		}

		// Forward the raw data
		c.Writer.WriteString(line + "\n")
		c.Writer.Flush()
	}
	if err := scanner.Err(); err != nil {
		log.Printf("stream scan error: %v", err)
	}
	saveDone()
}

func (h *Handler) ServeHTTP() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Static files
	r.Static("/static", "../frontend")

	// API routes
	api := r.Group("/api")
	{
		api.GET("/conversations", h.listConversations)
		api.POST("/conversations", h.createConversation)
		api.GET("/conversations/:id", h.getConversation)
		api.DELETE("/conversations/:id", h.deleteConversation)
		api.PUT("/config/system-prompt", h.updateSystemPrompt)
		api.GET("/config", h.getConfig)
		api.POST("/chat", h.chat)
	}

	// Serve index.html or mobile.html based on User-Agent
	r.GET("/", func(c *gin.Context) {
		ua := c.GetHeader("User-Agent")
		mobile := false
		for _, kw := range []string{"Mobile", "Android", "iPhone", "iPad", "iPod", "webOS", "BlackBerry", "Windows Phone"} {
			if strings.Contains(ua, kw) {
				mobile = true
				break
			}
		}
		if mobile {
			c.File("../frontend/mobile.html")
		} else {
			c.File("../frontend/index.html")
		}
	})

	r.Run(":8080")
}
