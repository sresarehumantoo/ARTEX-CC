package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrConversationNotFound = errors.New("conversation not found")

type ChatStore struct {
	files *FileStore
	mu    sync.Mutex
	seqs  map[string]int64
}

func NewChatStore(files *FileStore) *ChatStore {
	return &ChatStore{files: files, seqs: make(map[string]int64)}
}

func convDir(id string) (string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return "", ErrConversationNotFound
	}
	return "conversations/" + id, nil
}

func (s *ChatStore) List() ([]Conversation, error) {
	names, err := s.files.ListDir("conversations")
	if errors.Is(err, os.ErrNotExist) {
		return []Conversation{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Conversation{}
	for _, name := range names {
		c, err := s.Get(name)
		if err != nil {
			continue
		}
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

func (s *ChatStore) Get(id string) (*Conversation, error) {
	dir, err := convDir(id)
	if err != nil {
		return nil, err
	}
	var c Conversation
	if err := s.files.ReadJSON(dir+"/meta.json", &c); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (s *ChatStore) Create(req CreateConversationRequest) (*Conversation, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	c := &Conversation{
		ID:           uuid.NewString(),
		Title:        req.Title,
		Model:        req.Model,
		Agent:        req.Agent,
		SystemPrompt: req.SystemPrompt,
		Status:       ChatStatusIdle,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if c.Title == "" {
		c.Title = "New conversation"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.files.WriteJSON("conversations/"+c.ID+"/meta.json", c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *ChatStore) Update(id string, fn func(*Conversation)) (*Conversation, error) {
	dir, err := convDir(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var c Conversation
	if err := s.files.ReadJSON(dir+"/meta.json", &c); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}
	fn(&c)
	c.ID = id
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.files.WriteJSON(dir+"/meta.json", &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *ChatStore) Delete(id string) error {
	dir, err := convDir(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.seqs, id)
	return s.files.Delete(dir)
}

// AppendEvent assigns the next sequence number, persists the event, and returns it.
func (s *ChatStore) AppendEvent(id string, ev ChatEvent) (ChatEvent, error) {
	dir, err := convDir(id)
	if err != nil {
		return ev, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	seq, ok := s.seqs[id]
	if !ok {
		lines, err := s.files.ReadJSONL(dir+"/events.jsonl", 0)
		if err != nil {
			return ev, err
		}
		seq = int64(len(lines))
	}
	seq++
	ev.Seq = seq
	if ev.Time == "" {
		ev.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := s.files.AppendJSONL(dir+"/events.jsonl", ev); err != nil {
		return ev, err
	}
	s.seqs[id] = seq
	return ev, nil
}

func (s *ChatStore) Events(id string, afterSeq int64) ([]ChatEvent, error) {
	dir, err := convDir(id)
	if err != nil {
		return nil, err
	}
	lines, err := s.files.ReadJSONL(dir+"/events.jsonl", afterSeq)
	if err != nil {
		return nil, err
	}
	out := make([]ChatEvent, 0, len(lines))
	for _, l := range lines {
		var ev ChatEvent
		if err := json.Unmarshal(l, &ev); err != nil {
			return nil, fmt.Errorf("decode event: %w", err)
		}
		out = append(out, ev)
	}
	return out, nil
}
