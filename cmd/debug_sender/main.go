package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"linkedin-employee-scraper/internal/config"
	"linkedin-employee-scraper/internal/models"
	"linkedin-employee-scraper/internal/sender"
)

const taskGetURL = "https://api.dataforseo.com/v3/serp/google/organic/task_get/regular/"

type taskGetResponse struct {
	StatusCode    int     `json:"status_code"`
	StatusMessage string  `json:"status_message"`
	Cost          float64 `json:"cost"`
	Tasks         []struct {
		ID            string     `json:"id"`
		StatusCode    int        `json:"status_code"`
		StatusMessage string     `json:"status_message"`
		Cost          float64    `json:"cost"`
		ResultCount   int        `json:"result_count"`
		Result        []struct {
			Keyword        string `json:"keyword"`
			SeResultsCount int    `json:"se_results_count"`
			ItemsCount     int    `json:"items_count"`
			Items          []struct {
				Type  string `json:"type"`
				Title string `json:"title"`
				URL   string `json:"url"`
			} `json:"items"`
		} `json:"result"`
	} `json:"tasks"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[ERROR] Failed to load config: %v", err)
	}

	entries, err := readAllInput(cfg.InputFile)
	if err != nil {
		log.Fatalf("[ERROR] Failed to read input: %v", err)
	}
	totalEntries := len(entries)
	log.Printf("[INFO] Total entries in input: %d", totalEntries)

	if totalEntries == 0 {
		log.Println("[INFO] No entries found. Exiting.")
		return
	}

	// Change this to control how many entries to debug
	maxDebug := 3
	if len(entries) < maxDebug {
		maxDebug = len(entries)
	}
	entries = entries[:maxDebug]

	httpClient := &http.Client{Timeout: 60 * time.Second}

	var totalPostCost float64
	var totalGetCost float64
	var totalPeople int

	for i, entry := range entries {
		log.Printf("========== [%d/%d] Domain: %s | Company: %s ==========", i+1, len(entries), entry.Domain, entry.Company)

		keyword := fmt.Sprintf(`"linkedin.com/in/" "%s"`, entry.Company)
		log.Printf("[DEBUG] Keyword: %s", keyword)
		log.Printf("[DEBUG] Depth: %d", cfg.Depth)

		// --- Step 1: POST task (with postback_url to satisfy API validation) ---
		items := []models.DfsTaskPostItem{{
			Keyword:      keyword,
			LocationCode: 2840,
			LanguageCode: "en",
			Depth:        cfg.Depth,
			Tag:          entry.Domain,
			PostbackURL:  cfg.PostbackURL,
			PostbackData: "regular",
		}}

		postResp, err := postTask(httpClient, cfg.DfsLogin, cfg.DfsPassword, items)
		if err != nil {
			log.Printf("[ERROR] POST failed: %v", err)
			continue
		}

		log.Printf("[POST] Cost: $%.6f | TasksCount: %d | TasksError: %d",
			postResp.Cost, postResp.TasksCount, postResp.TasksError)

		if len(postResp.Tasks) == 0 || postResp.Tasks[0].StatusCode != 20100 {
			status := 0
			msg := "no tasks"
			if len(postResp.Tasks) > 0 {
				status = postResp.Tasks[0].StatusCode
				msg = postResp.Tasks[0].StatusMessage
			}
			log.Printf("[ERROR] Task not created: %d %s", status, msg)
			continue
		}

		taskID := postResp.Tasks[0].ID
		log.Printf("[POST] TaskID: %s | TaskCost: $%.6f", taskID, postResp.Tasks[0].Cost)
		totalPostCost += postResp.Cost

		// --- Step 2: Poll task_get until ready ---
		log.Printf("[INFO] Polling for result...")
		var getResp *taskGetResponse
		for attempt := 1; attempt <= 60; attempt++ {
			time.Sleep(3 * time.Second)
			getResp, err = getTask(httpClient, cfg.DfsLogin, cfg.DfsPassword, taskID)
			if err != nil {
				log.Printf("[POLL %d] Error: %v", attempt, err)
				getResp = nil
				continue
			}
			if len(getResp.Tasks) > 0 && getResp.Tasks[0].StatusCode == 20000 {
				log.Printf("[POLL %d] Task ready!", attempt)
				break
			}
			log.Printf("[POLL %d] Not ready (status: %d)", attempt, getResp.Tasks[0].StatusCode)
			getResp = nil
		}

		if getResp == nil {
			log.Printf("[ERROR] Task %s did not complete in time", taskID)
			fmt.Println()
			continue
		}

		task := getResp.Tasks[0]
		log.Printf("[GET] Cost: $%.6f | ResultCount: %d", task.Cost, task.ResultCount)
		totalGetCost += getResp.Cost

		peopleCount := 0
		for _, r := range task.Result {
			log.Printf("[RESULT] SE Results: %d | Items returned: %d", r.SeResultsCount, r.ItemsCount)
			for k, item := range r.Items {
				if item.Type != "organic" {
					continue
				}
				name := extractName(item.Title)
				log.Printf("  [%d] %s | %s", k+1, name, item.URL)
				peopleCount++
			}
		}
		log.Printf("[PEOPLE] Found %d LinkedIn profiles", peopleCount)
		totalPeople += peopleCount

		log.Printf("[COST] POST: $%.6f | GET: $%.6f | Running POST total: $%.6f",
			postResp.Cost, getResp.Cost, totalPostCost)
		fmt.Println()

		if i < len(entries)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Printf("========== SUMMARY ==========")
	log.Printf("Requests sent: %d", len(entries))
	log.Printf("Total POST cost: $%.6f", totalPostCost)
	log.Printf("Total GET cost:  $%.6f", totalGetCost)
	log.Printf("Combined cost:   $%.6f", totalPostCost+totalGetCost)
	if len(entries) > 0 {
		avgPost := totalPostCost / float64(len(entries))
		log.Printf("Avg POST cost per task: $%.6f", avgPost)
		log.Printf("Estimated cost for all %d entries: $%.2f (POST only)", totalEntries, avgPost*float64(totalEntries))
	}
	log.Printf("Total people found: %d", totalPeople)
}

func postTask(client *http.Client, login, password string, items []models.DfsTaskPostItem) (*models.DfsTaskPostResponse, error) {
	body, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", sender.DfsTaskPostURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(login, password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result models.DfsTaskPostResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func getTask(client *http.Client, login, password, taskID string) (*taskGetResponse, error) {
	req, err := http.NewRequest("GET", taskGetURL+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(login, password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result taskGetResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Tasks) == 0 {
		return nil, fmt.Errorf("no tasks in response")
	}
	return &result, nil
}

func extractName(title string) string {
	if idx := strings.Index(title, " - "); idx > 0 {
		return strings.TrimSpace(title[:idx])
	}
	if idx := strings.Index(title, " | "); idx > 0 {
		return strings.TrimSpace(title[:idx])
	}
	return title
}

func readAllInput(path string) ([]models.CompanyEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []models.CompanyEntry
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		domain := strings.TrimSpace(parts[0])
		company := strings.TrimSpace(parts[1])
		if domain == "" || company == "" {
			continue
		}
		entries = append(entries, models.CompanyEntry{Domain: domain, Company: company})
	}
	return entries, scanner.Err()
}
