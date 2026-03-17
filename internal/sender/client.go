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
