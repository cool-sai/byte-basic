package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Token struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Prefix     string `json:"prefix"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

type TokenView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

type TokenStore struct {
	path  string
	mu    sync.Mutex
	items []Token
}

func NewTokenStore(path string) (*TokenStore, error) {
	s := &TokenStore{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *TokenStore) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.items = nil
			return nil
		}
		return err
	}
	var items []Token
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	s.items = items
	return nil
}

func (s *TokenStore) saveLocked() error {
	if s.items == nil {
		s.items = []Token{}
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.path, append(b, '\n'))
}

func (s *TokenStore) List() []TokenView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TokenView, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, TokenView{
			ID: t.ID, Name: t.Name, Prefix: t.Prefix,
			CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt,
		})
	}
	return out
}

func (s *TokenStore) Create(name string) (string, TokenView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", TokenView{}, fmt.Errorf("请填写名称")
	}
	if len([]rune(name)) > 64 {
		return "", TokenView{}, fmt.Errorf("名称太长")
	}
	raw, err := randomHex(16)
	if err != nil {
		return "", TokenView{}, err
	}
	id, err := randomHex(8)
	if err != nil {
		return "", TokenView{}, err
	}
	item := Token{
		ID:        id,
		Name:      name,
		Hash:      tokenHash(raw),
		Prefix:    raw[:4],
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	if err := s.saveLocked(); err != nil {
		s.items = s.items[:len(s.items)-1]
		return "", TokenView{}, err
	}
	return raw, TokenView{
		ID: item.ID, Name: item.Name, Prefix: item.Prefix, CreatedAt: item.CreatedAt,
	}, nil
}

func (s *TokenStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.items[:0]
	found := false
	for _, t := range s.items {
		if t.ID == id {
			found = true
			continue
		}
		next = append(next, t)
	}
	if !found {
		return os.ErrNotExist
	}
	s.items = next
	return s.saveLocked()
}

func (s *TokenStore) Match(raw string) bool {
	if raw == "" {
		return false
	}
	sum := tokenHash(raw)
	want := []byte(sum)
	s.mu.Lock()
	defer s.mu.Unlock()
	hit := -1
	for i := range s.items {
		if subtle.ConstantTimeCompare([]byte(s.items[i].Hash), want) == 1 {
			hit = i
		}
	}
	if hit < 0 {
		return false
	}
	s.items[hit].LastUsedAt = time.Now().Format(time.RFC3339)
	_ = s.saveLocked()
	return true
}

func tokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
