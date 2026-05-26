package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	conversations map[string]*Conversation
	mu            sync.RWMutex
	basePath      string
}

func NewStore(basePath string) (*Store, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	s := &Store{
		conversations: make(map[string]*Conversation),
		basePath:      basePath,
	}

	files, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(basePath, f.Name()))
			if err != nil {
				continue
			}
			var conv Conversation
			if err := json.Unmarshal(data, &conv); err == nil {
				s.conversations[conv.ID] = &conv
			}
		}
	}

	return s, nil
}

func (s *Store) save(conversation *Conversation) error {
	filename := filepath.Join(s.basePath, conversation.ID+".json")
	data, err := json.MarshalIndent(conversation, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func (s *Store) CreateConversation() *Conversation {
	conv := &Conversation{
		ID:        uuid.New().String(),
		Title:     "客服小祥，为您服务",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []Message{},
	}

	s.mu.Lock()
	s.conversations[conv.ID] = conv
	s.mu.Unlock()

	s.save(conv)
	return conv
}

func (s *Store) GetConversation(id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if conv, ok := s.conversations[id]; ok {
		return conv, nil
	}
	return nil, fmt.Errorf("conversation not found")
}

func (s *Store) ListConversations() []*Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	convs := make([]*Conversation, 0, len(s.conversations))
	for _, conv := range s.conversations {
		convs = append(convs, conv)
	}

	// Sort by UpdatedAt descending
	for i := 0; i < len(convs)-1; i++ {
		for j := i + 1; j < len(convs); j++ {
			if convs[j].UpdatedAt.After(convs[i].UpdatedAt) {
				convs[i], convs[j] = convs[j], convs[i]
			}
		}
	}

	return convs
}

func (s *Store) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.conversations[id]; !ok {
		return fmt.Errorf("conversation not found")
	}

	delete(s.conversations, id)

	filename := filepath.Join(s.basePath, id+".json")
	os.Remove(filename)

	return nil
}

func (s *Store) AddMessage(conversationID string, role, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		return fmt.Errorf("conversation not found")
	}

	conv.Messages = append(conv.Messages, Message{Role: role, Content: content})
	conv.UpdatedAt = time.Now()

	// Update title from first user message
	if role == "user" && len(conv.Messages) == 2 {
		if len(content) > 30 {
			conv.Title = content[:30] + "..."
		} else {
			conv.Title = content
		}
	}

	return s.save(conv)
}

func (s *Store) GetMessages(conversationID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		return nil, fmt.Errorf("conversation not found")
	}

	return conv.Messages, nil
}