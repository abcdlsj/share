package store

import (
	"sync"
	"time"

	"notionbot/internal/model"
)

type Phase int

const (
	PhaseIdle Phase = iota
	PhaseRecording
	PhaseAwaitTitle
)

type ChatContext struct {
	Phase   Phase
	Entries []model.Entry
	EndedAt time.Time
}

type StateStore struct {
	mu    sync.Mutex
	chats map[int64]*ChatContext
}

func NewStateStore() *StateStore {
	return &StateStore{chats: map[int64]*ChatContext{}}
}

func (s *StateStore) Get(chatID int64) *ChatContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.chats[chatID]
	if !ok {
		ctx = &ChatContext{Phase: PhaseIdle}
		s.chats[chatID] = ctx
	}
	return ctx
}

func (s *StateStore) Reset(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chats[chatID] = &ChatContext{Phase: PhaseIdle}
}
