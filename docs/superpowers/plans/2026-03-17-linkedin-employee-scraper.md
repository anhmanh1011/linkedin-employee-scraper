# LinkedIn Employee Scraper Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a two-binary Go application that sends Google SERP queries to DataForSEO API to find LinkedIn profiles of employees by company name, and extracts their names from postback results.

**Architecture:** Two binaries (sender + receiver) sharing state via JSON file. Sender reads input file, batches tasks to DataForSEO. Receiver is an HTTP server that receives async postback results and writes extracted employee names to output file via a channel-based writer.

**Tech Stack:** Go 1.25, chi router, godotenv, DataForSEO SERP API v3

**Spec:** `docs/superpowers/specs/2026-03-17-linkedin-employee-scraper-design.md`

---

## File Structure

```
linkedin-employee-scraper/
├── cmd/
│   ├── sender/
│   │   └── main.go              # Sender entry point
│   └── receiver/
│       └── main.go              # Receiver entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Load .env, expose Config struct
│   ├── models/
│   │   └── models.go            # DfsPostBack, DfsTaskPostRequest, State structs
│   ├── store/
│   │   └── store.go             # Read/write state.json (channel-based writer)
│   ├── extractor/
│   │   ├── extractor.go         # Extract employee name from SERP title
│   │   └── extractor_test.go    # Tests for extractor
│   ├── sender/
│   │   ├── client.go            # DataForSEO API client (batch send)
│   │   └── client_test.go       # Tests for API client
│   └── receiver/
│       ├── handler.go           # Postback handler + writer goroutine
│       └── handler_test.go      # Tests for handler
├── data/
│   └── input.txt                # domain|company (user provides)
├── .env.example                 # Template config
├── go.mod
└── go.sum
```

---

## Chunk 1: Project Setup + Config + Models

### Task 1: Initialize Go module and dependencies

**Files:**
- Create: `go.mod`
- Create: `.env.example`
- Create: `.gitignore`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd C:/Users/DAO\ MANH/GolandProjects/linkedin-employee-scraper
go mod init linkedin-employee-scraper
```

- [ ] **Step 2: Add dependencies**

Run:
```bash
go get github.com/go-chi/chi/v5
go get github.com/go-chi/chi/v5/middleware
go get github.com/joho/godotenv
```

- [ ] **Step 3: Create .env.example**

Create `.env.example`:
```
DFS_LOGIN=your_login
DFS_PASSWORD=your_password
POSTBACK_URL=http://your-server:8080/postback
DEPTH=700
BATCH_SIZE=100
BATCH_DELAY_MS=500
MAX_CONCURRENT=30
RETRY_COUNT=3
INPUT_FILE=data/input.txt
OUTPUT_FILE=data/output.txt
STATE_FILE=data/state.json
RECEIVER_PORT=8080
```

- [ ] **Step 4: Create .gitignore**

Create `.gitignore`:
```
.env
data/output.txt
data/state.json
*.exe
*.log
```

- [ ] **Step 5: Create data directory with sample input**

Create `data/input.txt`:
```
talentpool.co.id|Talentpool Indonesia
```

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum .env.example .gitignore data/input.txt
git commit -m "feat: initialize project with dependencies and config template"
```

---

### Task 2: Config loader

**Files:**
- Create: `internal/config/config.go`

- [ ] **Step 1: Write config.go**

Create `internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DfsLogin     string
	DfsPassword  string
	PostbackURL  string
	Depth        int
	BatchSize    int
	BatchDelayMs int
	MaxConcurrent int
	RetryCount   int
	InputFile    string
	OutputFile   string
	StateFile    string
	ReceiverPort string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // ignore error if .env not found, fall back to env vars

	cfg := &Config{
		DfsLogin:     os.Getenv("DFS_LOGIN"),
		DfsPassword:  os.Getenv("DFS_PASSWORD"),
		PostbackURL:  os.Getenv("POSTBACK_URL"),
		InputFile:    getEnvDefault("INPUT_FILE", "data/input.txt"),
		OutputFile:   getEnvDefault("OUTPUT_FILE", "data/output.txt"),
		StateFile:    getEnvDefault("STATE_FILE", "data/state.json"),
		ReceiverPort: getEnvDefault("RECEIVER_PORT", "8080"),
	}

	var err error
	cfg.Depth, err = getEnvInt("DEPTH", 700)
	if err != nil {
		return nil, fmt.Errorf("invalid DEPTH: %w", err)
	}
	cfg.BatchSize, err = getEnvInt("BATCH_SIZE", 100)
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_SIZE: %w", err)
	}
	cfg.BatchDelayMs, err = getEnvInt("BATCH_DELAY_MS", 500)
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_DELAY_MS: %w", err)
	}
	cfg.MaxConcurrent, err = getEnvInt("MAX_CONCURRENT", 30)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_CONCURRENT: %w", err)
	}
	cfg.RetryCount, err = getEnvInt("RETRY_COUNT", 3)
	if err != nil {
		return nil, fmt.Errorf("invalid RETRY_COUNT: %w", err)
	}

	if cfg.DfsLogin == "" || cfg.DfsPassword == "" {
		return nil, fmt.Errorf("DFS_LOGIN and DFS_PASSWORD are required")
	}
	if cfg.PostbackURL == "" {
		return nil, fmt.Errorf("POSTBACK_URL is required")
	}

	return cfg, nil
}

func getEnvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	return strconv.Atoi(v)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add config loader from .env"
```

---

### Task 3: Models

**Files:**
- Create: `internal/models/models.go`

- [ ] **Step 1: Write models.go**

Create `internal/models/models.go`:
```go
package models

import "time"

// --- Input ---

type CompanyEntry struct {
	Domain  string
	Company string
}

// --- DataForSEO Task Post Request ---

type DfsTaskPostItem struct {
	Keyword      string `json:"keyword"`
	LocationCode int    `json:"location_code"`
	LanguageCode string `json:"language_code"`
	Depth        int    `json:"depth"`
	Tag          string `json:"tag"`
	PostbackURL  string `json:"postback_url"`
	PostbackData string `json:"postback_data"`
}

// --- DataForSEO Task Post Response ---

type DfsTaskPostResponse struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	TasksCount    int    `json:"tasks_count"`
	TasksError    int    `json:"tasks_error"`
	Tasks         []struct {
		ID            string `json:"id"`
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
	} `json:"tasks"`
}

// --- DataForSEO Postback Payload ---

type DfsPostBack struct {
	Version       string `json:"version"`
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	Time          string `json:"time"`
	Cost          float64 `json:"cost"`
	TasksCount    int    `json:"tasks_count"`
	TasksError    int    `json:"tasks_error"`
	Tasks         []DfsTask `json:"tasks"`
}

type DfsTask struct {
	ID            string    `json:"id"`
	StatusCode    int       `json:"status_code"`
	StatusMessage string    `json:"status_message"`
	Time          string    `json:"time"`
	Cost          float64   `json:"cost"`
	ResultCount   int       `json:"result_count"`
	Path          []string  `json:"path"`
	Data          DfsTaskData `json:"data"`
	Result        []DfsResult `json:"result"`
}

type DfsTaskData struct {
	API          string `json:"api"`
	Function     string `json:"function"`
	Se           string `json:"se"`
	SeType       string `json:"se_type"`
	LanguageCode string `json:"language_code"`
	LocationCode int    `json:"location_code"`
	Keyword      string `json:"keyword"`
	Tag          string `json:"tag"`
}

type DfsResult struct {
	Keyword        string    `json:"keyword"`
	Type           string    `json:"type"`
	SeDomain       string    `json:"se_domain"`
	LocationCode   int       `json:"location_code"`
	LanguageCode   string    `json:"language_code"`
	CheckURL       string    `json:"check_url"`
	Datetime       string    `json:"datetime"`
	SeResultsCount int       `json:"se_results_count"`
	ItemsCount     int       `json:"items_count"`
	Items          []DfsItem `json:"items"`
}

type DfsItem struct {
	Type         string `json:"type"`
	RankGroup    int    `json:"rank_group"`
	RankAbsolute int    `json:"rank_absolute"`
	Domain       string `json:"domain"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	Breadcrumb   string `json:"breadcrumb"`
}

// --- State ---

type SentDomain struct {
	Company string    `json:"company"`
	TaskIDs []string  `json:"task_ids"`
	SentAt  time.Time `json:"sent_at"`
}

type State struct {
	SentTasks   map[string]SentDomain `json:"sent_tasks"`
	TotalSent   int                   `json:"total_sent"`
	LastBatchAt time.Time             `json:"last_batch_at"`
}

func NewState() *State {
	return &State{
		SentTasks: make(map[string]SentDomain),
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/models/models.go
git commit -m "feat: add shared models for DataForSEO API and state"
```

---

## Chunk 2: Extractor + Store

### Task 4: Name extractor with TDD

**Files:**
- Create: `internal/extractor/extractor.go`
- Create: `internal/extractor/extractor_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/extractor/extractor_test.go`:
```go
package extractor

import "testing"

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "standard linkedin title",
			title:    "Moch Ichlil - Managing Director of Talentpool Indonesia",
			expected: "Moch Ichlil",
		},
		{
			name:     "title with multiple dashes",
			title:    "Jean-Pierre Dupont - Senior Engineer - Google",
			expected: "Jean-Pierre Dupont",
		},
		{
			name:     "title with no dash",
			title:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "empty title",
			title:    "",
			expected: "",
		},
		{
			name:     "title with only spaces around dash",
			title:    "  Alice Smith  -  Product Manager  ",
			expected: "Alice Smith",
		},
		{
			name:     "title with LinkedIn suffix",
			title:    "Bob Jones - LinkedIn",
			expected: "Bob Jones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractName(tt.title)
			if got != tt.expected {
				t.Errorf("ExtractName(%q) = %q, want %q", tt.title, got, tt.expected)
			}
		})
	}
}

func TestIsLinkedInProfileURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://www.linkedin.com/in/moch-ichlil-123", true},
		{"https://linkedin.com/in/john-doe", true},
		{"https://www.linkedin.com/company/talentpool", false},
		{"https://www.google.com/search?q=test", false},
		{"https://vn.linkedin.com/in/someone", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := IsLinkedInProfileURL(tt.url)
			if got != tt.expected {
				t.Errorf("IsLinkedInProfileURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
cd C:/Users/DAO\ MANH/GolandProjects/linkedin-employee-scraper
go test ./internal/extractor/ -v
```
Expected: FAIL — functions not defined.

- [ ] **Step 3: Write implementation**

Create `internal/extractor/extractor.go`:
```go
package extractor

import "strings"

// ExtractName extracts the person's name from a LinkedIn SERP title.
// Title format: "Name - Job Title at Company"
// Returns the part before the first " - ", trimmed.
func ExtractName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	parts := strings.SplitN(title, " - ", 2)
	return strings.TrimSpace(parts[0])
}

// IsLinkedInProfileURL checks if a URL is a LinkedIn personal profile.
func IsLinkedInProfileURL(url string) bool {
	return strings.Contains(url, "linkedin.com/in/")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/extractor/ -v
```
Expected: PASS — all tests green.

- [ ] **Step 5: Commit**

```bash
git add internal/extractor/
git commit -m "feat: add name extractor with tests"
```

---

### Task 5: State store

**Files:**
- Create: `internal/store/store.go`

- [ ] **Step 1: Write store.go**

Create `internal/store/store.go`:
```go
package store

import (
	"encoding/json"
	"linkedin-employee-scraper/internal/models"
	"log"
	"os"
	"sync"
	"time"
)

// MarkSentCmd is sent to the state writer goroutine.
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

// Load reads state from disk. If file doesn't exist, starts with empty state.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh state
		}
		return err
	}

	return json.Unmarshal(data, s.state)
}

// StartWriter starts the dedicated state writer goroutine.
// Call Close() to stop it and flush remaining writes.
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

// MarkSent sends a mark-sent command to the writer goroutine.
func (s *Store) MarkSent(domain string, entry models.SentDomain) {
	s.writeCh <- MarkSentCmd{Domain: domain, Entry: entry}
}

// Close stops the writer goroutine and waits for it to drain.
func (s *Store) Close() {
	close(s.writeCh)
	<-s.done
}

// IsSent checks if a domain was already sent.
func (s *Store) IsSent(domain string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.state.SentTasks[domain]
	return ok
}

// TotalSent returns total sent count.
func (s *Store) TotalSent() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.TotalSent
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/store/store.go
git commit -m "feat: add file-based state store with mutex protection"
```

---

## Chunk 3: Sender Binary

### Task 6: DataForSEO API client

**Files:**
- Create: `internal/sender/client.go`
- Create: `internal/sender/client_test.go`

- [ ] **Step 1: Write client_test.go with unit test for request building**

Create `internal/sender/client_test.go`:
```go
package sender

import (
	"linkedin-employee-scraper/internal/models"
	"testing"
)

func TestBuildTaskPostBody(t *testing.T) {
	entries := []models.CompanyEntry{
		{Domain: "example.com", Company: "Example Corp"},
		{Domain: "test.io", Company: "Test Inc"},
	}

	items := BuildTaskPostBody(entries, "http://localhost:8080/postback", 700)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	item := items[0]
	expectedKeyword := `site:linkedin.com/in "Example Corp"`
	if item.Keyword != expectedKeyword {
		t.Errorf("keyword = %q, want %q", item.Keyword, expectedKeyword)
	}
	if item.Tag != "example.com" {
		t.Errorf("tag = %q, want %q", item.Tag, "example.com")
	}
	if item.Depth != 700 {
		t.Errorf("depth = %d, want 700", item.Depth)
	}
	if item.PostbackURL != "http://localhost:8080/postback" {
		t.Errorf("postback_url = %q, want %q", item.PostbackURL, "http://localhost:8080/postback")
	}
	if item.PostbackData != "advanced" {
		t.Errorf("postback_data = %q, want %q", item.PostbackData, "advanced")
	}
	if item.LocationCode != 2840 {
		t.Errorf("location_code = %d, want 2840", item.LocationCode)
	}
	if item.LanguageCode != "en" {
		t.Errorf("language_code = %q, want %q", item.LanguageCode, "en")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/sender/ -v
```
Expected: FAIL — `BuildTaskPostBody` not defined.

- [ ] **Step 3: Write client.go**

Create `internal/sender/client.go`:
```go
package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	"linkedin-employee-scraper/internal/models"
)

const dfsTaskPostURL = "https://api.dataforseo.com/v3/serp/google/organic/task_post"

type Client struct {
	login      string
	password   string
	httpClient *http.Client
	retryCount int
}

func NewClient(login, password string, retryCount int) *Client {
	return &Client{
		login:      login,
		password:   password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		retryCount: retryCount,
	}
}

// BuildTaskPostBody creates the request body for DataForSEO task_post.
func BuildTaskPostBody(entries []models.CompanyEntry, postbackURL string, depth int) []models.DfsTaskPostItem {
	items := make([]models.DfsTaskPostItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, models.DfsTaskPostItem{
			Keyword:      fmt.Sprintf(`site:linkedin.com/in "%s"`, e.Company),
			LocationCode: 2840,
			LanguageCode: "en",
			Depth:        depth,
			Tag:          e.Domain,
			PostbackURL:  postbackURL,
			PostbackData: "advanced",
		})
	}
	return items
}

// SendBatch sends a batch of tasks to DataForSEO and returns task IDs.
func (c *Client) SendBatch(items []models.DfsTaskPostItem) (*models.DfsTaskPostResponse, error) {
	body, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			if delay > 8*time.Second {
				delay = 8 * time.Second
			}
			log.Printf("[INFO] Retry attempt %d/%d after %v", attempt, c.retryCount, delay)
			time.Sleep(delay)
		}

		resp, err := c.doRequest(body)
		if err != nil {
			lastErr = err
			log.Printf("[WARN] Batch send attempt %d failed: %v", attempt+1, err)
			continue
		}
		return resp, nil
	}

	return nil, fmt.Errorf("all %d attempts failed, last error: %w", c.retryCount+1, lastErr)
}

func (c *Client) doRequest(body []byte) (*models.DfsTaskPostResponse, error) {
	req, err := http.NewRequest("POST", dfsTaskPostURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.SetBasicAuth(c.login, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result models.DfsTaskPostResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.StatusCode != 20000 {
		return nil, fmt.Errorf("API error %d: %s", result.StatusCode, result.StatusMessage)
	}

	return &result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/sender/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sender/
git commit -m "feat: add DataForSEO API client with retry and batch support"
```

---

### Task 7: Sender main binary

**Files:**
- Create: `cmd/sender/main.go`

- [ ] **Step 1: Write cmd/sender/main.go**

Create `cmd/sender/main.go`:
```go
package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"linkedin-employee-scraper/internal/config"
	"linkedin-employee-scraper/internal/models"
	"linkedin-employee-scraper/internal/sender"
	"linkedin-employee-scraper/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[ERROR] Failed to load config: %v", err)
	}

	// Load state
	st := store.New(cfg.StateFile)
	if err := st.Load(); err != nil {
		log.Fatalf("[ERROR] Failed to load state: %v", err)
	}
	st.StartWriter() // start channel-based state writer
	defer st.Close()  // drain and stop writer on exit

	log.Printf("[INFO] Loaded state: %d domains already sent", st.TotalSent())

	// Read input file
	entries, err := readInput(cfg.InputFile, st)
	if err != nil {
		log.Fatalf("[ERROR] Failed to read input: %v", err)
	}
	log.Printf("[INFO] Found %d new companies to process", len(entries))

	if len(entries) == 0 {
		log.Println("[INFO] Nothing to send. Exiting.")
		return
	}

	// Create API client
	client := sender.NewClient(cfg.DfsLogin, cfg.DfsPassword, cfg.RetryCount)

	// Split into batches
	batches := makeBatches(entries, cfg.BatchSize)
	log.Printf("[INFO] Split into %d batches of max %d tasks", len(batches), cfg.BatchSize)

	// Send batches with concurrency control
	sem := make(chan struct{}, cfg.MaxConcurrent)
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failCount atomic.Int64

	for i, batch := range batches {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(batchIdx int, entries []models.CompanyEntry) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore

			log.Printf("[INFO] Sending batch %d/%d (%d tasks)", batchIdx+1, len(batches), len(entries))

			items := sender.BuildTaskPostBody(entries, cfg.PostbackURL, cfg.Depth)
			resp, err := client.SendBatch(items)
			if err != nil {
				log.Printf("[ERROR] Batch %d failed: %v", batchIdx+1, err)
				failCount.Add(int64(len(entries)))
				return
			}

			// Record sent domains in state via channel (safe for concurrent goroutines)
			for j, entry := range entries {
				taskID := ""
				if j < len(resp.Tasks) {
					taskID = resp.Tasks[j].ID
				}
				st.MarkSent(entry.Domain, models.SentDomain{
					Company: entry.Company,
					TaskIDs: []string{taskID},
					SentAt:  time.Now(),
				})
			}

			successCount.Add(int64(len(entries)))
			log.Printf("[INFO] Batch %d sent successfully (%d tasks)", batchIdx+1, len(entries))
		}(i, batch)

		// Delay between launching batches
		if i < len(batches)-1 {
			time.Sleep(time.Duration(cfg.BatchDelayMs) * time.Millisecond)
		}
	}

	wg.Wait()

	log.Printf("[INFO] Done. Sent: %d, Failed: %d, Total in state: %d", successCount.Load(), failCount.Load(), st.TotalSent())
}

func readInput(path string, st *store.Store) ([]models.CompanyEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []models.CompanyEntry
	scanner := bufio.NewScanner(file)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			log.Printf("[WARN] Skipping malformed line %d: %q", lineNum, line)
			continue
		}

		domain := strings.TrimSpace(parts[0])
		company := strings.TrimSpace(parts[1])

		if domain == "" || company == "" {
			log.Printf("[WARN] Skipping empty domain/company at line %d", lineNum)
			continue
		}

		if st.IsSent(domain) {
			continue
		}

		entries = append(entries, models.CompanyEntry{
			Domain:  domain,
			Company: company,
		})
	}

	return entries, scanner.Err()
}

func makeBatches(entries []models.CompanyEntry, batchSize int) [][]models.CompanyEntry {
	var batches [][]models.CompanyEntry
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		batches = append(batches, entries[i:end])
	}
	return batches
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
go build ./cmd/sender/
```
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/sender/main.go
git commit -m "feat: add sender binary with batch sending and concurrency"
```

---

## Chunk 4: Receiver Binary

### Task 8: Postback handler + writer

**Files:**
- Create: `internal/receiver/handler.go`
- Create: `internal/receiver/handler_test.go`

- [ ] **Step 1: Write handler_test.go**

Create `internal/receiver/handler_test.go`:
```go
package receiver

import (
	"bytes"
	"encoding/json"
	"linkedin-employee-scraper/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPostbackHandler_ExtractsNames(t *testing.T) {
	resultCh := make(chan string, 100)
	handler := NewPostbackHandler(resultCh)

	payload := models.DfsPostBack{
		StatusCode:    20000,
		StatusMessage: "Ok.",
		TasksCount:    1,
		Tasks: []models.DfsTask{
			{
				ID:         "test-task-1",
				StatusCode: 20000,
				Data: models.DfsTaskData{
					Tag: "example.com",
				},
				Result: []models.DfsResult{
					{
						Items: []models.DfsItem{
							{
								Type:  "organic",
								Title: "John Doe - Software Engineer",
								URL:   "https://www.linkedin.com/in/john-doe",
							},
							{
								Type:  "organic",
								Title: "Jane Smith - Product Manager",
								URL:   "https://www.linkedin.com/in/jane-smith",
							},
							{
								Type:  "organic",
								Title: "Some Company Page",
								URL:   "https://www.linkedin.com/company/example",
							},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/postback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Drain channel with timeout
	var results []string
	timeout := time.After(1 * time.Second)
	for {
		select {
		case r := <-resultCh:
			results = append(results, r)
		case <-timeout:
			goto done
		}
	}
done:

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}
	if results[0] != "example.com|John Doe" {
		t.Errorf("result[0] = %q, want %q", results[0], "example.com|John Doe")
	}
	if results[1] != "example.com|Jane Smith" {
		t.Errorf("result[1] = %q, want %q", results[1], "example.com|Jane Smith")
	}
}

func TestPostbackHandler_SkipsErrorTasks(t *testing.T) {
	resultCh := make(chan string, 100)
	handler := NewPostbackHandler(resultCh)

	payload := models.DfsPostBack{
		StatusCode: 20000,
		Tasks: []models.DfsTask{
			{
				ID:         "error-task",
				StatusCode: 40000,
				Data:       models.DfsTaskData{Tag: "fail.com"},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/postback", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	select {
	case r := <-resultCh:
		t.Fatalf("expected no results, got %q", r)
	case <-time.After(100 * time.Millisecond):
		// good, no results
	}
}
```

func TestPostbackHandler_GzipBody(t *testing.T) {
	resultCh := make(chan string, 100)
	handler := NewPostbackHandler(resultCh)

	payload := models.DfsPostBack{
		StatusCode: 20000,
		Tasks: []models.DfsTask{
			{
				ID:         "gzip-task",
				StatusCode: 20000,
				Data:       models.DfsTaskData{Tag: "gzip.com"},
				Result: []models.DfsResult{
					{
						Items: []models.DfsItem{
							{
								Type:  "organic",
								Title: "Alice Gzip - Engineer",
								URL:   "https://www.linkedin.com/in/alice-gzip",
							},
						},
					},
				},
			},
		},
	}

	raw, _ := json.Marshal(payload)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write(raw)
	gw.Close()

	req := httptest.NewRequest("POST", "/postback", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	select {
	case r := <-resultCh:
		if r != "gzip.com|Alice Gzip" {
			t.Errorf("result = %q, want %q", r, "gzip.com|Alice Gzip")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected result, got none")
	}
}

func TestPostbackHandler_InvalidJSON(t *testing.T) {
	resultCh := make(chan string, 100)
	handler := NewPostbackHandler(resultCh)

	req := httptest.NewRequest("POST", "/postback", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
```

Also add these imports at the top of the test file:
```go
import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"linkedin-employee-scraper/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/receiver/ -v
```
Expected: FAIL — `NewPostbackHandler` not defined.

- [ ] **Step 3: Write handler.go**

Create `internal/receiver/handler.go`:
```go
package receiver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"sync/atomic"

	"linkedin-employee-scraper/internal/extractor"
	"linkedin-employee-scraper/internal/models"
)

type PostbackHandler struct {
	resultCh      chan<- string
	tasksReceived atomic.Int64
}

func NewPostbackHandler(resultCh chan<- string) *PostbackHandler {
	return &PostbackHandler{resultCh: resultCh}
}

func (h *PostbackHandler) TasksReceived() int64 {
	return h.tasksReceived.Load()
}

func (h *PostbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 50MB limit

	var reader io.Reader = r.Body
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			log.Printf("[ERROR] Invalid gzip body: %v", err)
			http.Error(w, "invalid gzip body", http.StatusBadRequest)
			return
		}
		defer gr.Close()
		reader = gr
	}
	defer r.Body.Close()

	var payload models.DfsPostBack
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		log.Printf("[ERROR] Invalid JSON: %v", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Check top-level status
	if payload.StatusCode != 20000 {
		log.Printf("[WARN] Non-20000 top-level status: %d %s", payload.StatusCode, payload.StatusMessage)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	namesExtracted := 0
	h.tasksReceived.Add(int64(len(payload.Tasks)))

	for _, task := range payload.Tasks {
		// Check per-task status
		if task.StatusCode != 20000 {
			log.Printf("[WARN] Task %s error: %d %s", task.ID, task.StatusCode, task.StatusMessage)
			continue
		}

		domain := task.Data.Tag
		if domain == "" {
			log.Printf("[WARN] Task %s has empty tag, skipping", task.ID)
			continue
		}

		for _, result := range task.Result {
			for _, item := range result.Items {
				if !extractor.IsLinkedInProfileURL(item.URL) {
					continue
				}

				name := extractor.ExtractName(item.Title)
				if name == "" {
					continue
				}

				line := fmt.Sprintf("%s|%s", domain, name)

				// Non-blocking send to channel
				select {
				case h.resultCh <- line:
					namesExtracted++
				default:
					log.Printf("[WARN] Channel full, dropping result: %s", line)
				}
			}
		}
	}

	log.Printf("[INFO] Processed postback: %d tasks, %d names extracted", len(payload.Tasks), namesExtracted)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/receiver/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/receiver/
git commit -m "feat: add postback handler with name extraction and channel writer"
```

---

### Task 9: Receiver main binary

**Files:**
- Create: `cmd/receiver/main.go`

- [ ] **Step 1: Write cmd/receiver/main.go**

Create `cmd/receiver/main.go`:
```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"linkedin-employee-scraper/internal/config"
	"linkedin-employee-scraper/internal/receiver"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[ERROR] Failed to load config: %v", err)
	}

	// Result channel and writer
	resultCh := make(chan string, 10000)
	var namesWritten atomic.Int64

	// Start writer goroutine
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		writeResults(cfg.OutputFile, resultCh, &namesWritten)
	}()

	// Setup router
	handler := receiver.NewPostbackHandler(resultCh)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tasks_received":%d,"names_written":%d}`,
			handler.TasksReceived(), namesWritten.Load())
	})

	r.Post("/postback", func(w http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(w, req)
	})

	// HTTP server with timeouts
	srv := &http.Server{
		Addr:         ":" + cfg.ReceiverPort,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] Receiver listening on :%s", cfg.ReceiverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[ERROR] Server failed: %v", err)
		}
	}()

	<-done
	log.Println("[INFO] Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	// Close channel and wait for writer to drain
	close(resultCh)
	writerWg.Wait()

	log.Printf("[INFO] Shutdown complete. Total names written: %d", namesWritten.Load())
}

func writeResults(outputFile string, resultCh <-chan string, counter *atomic.Int64) {
	file, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("[ERROR] Failed to open output file: %v", err)
	}
	defer file.Close()

	for line := range resultCh {
		if _, err := fmt.Fprintln(file, line); err != nil {
			log.Printf("[ERROR] Failed to write result: %v", err)
			continue
		}
		if err := file.Sync(); err != nil {
			log.Printf("[ERROR] Failed to sync file: %v", err)
		}
		counter.Add(1)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
go build ./cmd/receiver/
```
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/receiver/main.go
git commit -m "feat: add receiver binary with graceful shutdown and file writer"
```

---

## Chunk 5: Integration Test + Final

### Task 10: Run all tests

- [ ] **Step 1: Run all unit tests**

Run:
```bash
go test ./... -v
```
Expected: All PASS.

- [ ] **Step 2: Build both binaries**

Run:
```bash
go build -o sender.exe ./cmd/sender/
go build -o receiver.exe ./cmd/receiver/
```
Expected: Both compile without errors.

- [ ] **Step 3: Commit if any outstanding changes**

```bash
git status
# Only add specific files if there are changes
git commit -m "feat: complete linkedin-employee-scraper with sender and receiver"
```

---

## Usage

### 1. Start receiver first:
```bash
# Create .env from template
cp .env.example .env
# Edit .env with your DataForSEO credentials

# Start receiver
go run ./cmd/receiver/
```

### 2. Prepare input file:
```
# data/input.txt
talentpool.co.id|Talentpool Indonesia
example.com|Example Corp
```

### 3. Run sender:
```bash
go run ./cmd/sender/
```

### 4. Check results:
```bash
cat data/output.txt
# talentpool.co.id|Moch Ichlil
# talentpool.co.id|Anita Ratnaningsih
# ...
```

### 5. Monitor:
```bash
curl http://localhost:8080/ping
curl http://localhost:8080/stats
```
