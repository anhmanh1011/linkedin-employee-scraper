package receiver

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
		// good
	}
}

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
