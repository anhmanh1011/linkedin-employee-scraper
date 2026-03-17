package store

import (
	"encoding/json"
	"linkedin-employee-scraper/internal/models"
	"log"
	"os"
	"sync"
	"time"
)

type MarkSentCmd struct {
	Domain string
	Entry  models.SentDomain
}

type Store struct {
	path    string
	mu      sync.RWMutex
	state   *models.State
	writeCh chan MarkSentCmd
	done    chan struct{}
}

func New(path string) *Store {
	return &Store{
		path:    path,
		state:   models.NewState(),
		writeCh: make(chan MarkSentCmd, 1000),
		done:    make(chan struct{}),
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, s.state)
}

func (s *Store) StartWriter() {
	go func() {
		defer close(s.done)
		for cmd := range s.writeCh {
			s.mu.Lock()
			s.state.SentTasks[cmd.Domain] = cmd.Entry
			s.state.TotalSent++
			s.state.LastBatchAt = time.Now()

			data, err := json.MarshalIndent(s.state, "", "  ")
			s.mu.Unlock()

			if err != nil {
				log.Printf("[ERROR] Failed to marshal state: %v", err)
				continue
			}
			if err := os.WriteFile(s.path, data, 0644); err != nil {
				log.Printf("[ERROR] Failed to write state file: %v", err)
			}
		}
	}()
}

func (s *Store) MarkSent(domain string, entry models.SentDomain) {
	s.writeCh <- MarkSentCmd{Domain: domain, Entry: entry}
}

func (s *Store) Close() {
	close(s.writeCh)
	<-s.done
}

func (s *Store) IsSent(domain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.state.SentTasks[domain]
	return ok
}

func (s *Store) TotalSent() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.TotalSent
}
