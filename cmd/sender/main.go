package main

import (
	"bufio"
	"context"
	"errors"
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

	st := store.New(cfg.StateFile)
	if err := st.Load(); err != nil {
		log.Fatalf("[ERROR] Failed to load state: %v", err)
	}
	st.StartWriter()
	defer st.Close()

	log.Printf("[INFO] Loaded state: %d domains already sent", st.TotalSent())

	entries, err := readInput(cfg.InputFile, st)
	if err != nil {
		log.Fatalf("[ERROR] Failed to read input: %v", err)
	}
	log.Printf("[INFO] Found %d new companies to process", len(entries))

	if len(entries) == 0 {
		log.Println("[INFO] Nothing to send. Exiting.")
		return
	}

	client := sender.NewClient(cfg.DfsLogin, cfg.DfsPassword, cfg.RetryCount)

	batches := makeBatches(entries, cfg.BatchSize)
	log.Printf("[INFO] Split into %d batches of max %d tasks", len(batches), cfg.BatchSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sem := make(chan struct{}, cfg.MaxConcurrent)
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failCount atomic.Int64

	for i, batch := range batches {
		if ctx.Err() != nil {
			log.Printf("[INFO] Stopping: context cancelled, skipping remaining batches")
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(batchIdx int, entries []models.CompanyEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			log.Printf("[INFO] Sending batch %d/%d (%d tasks)", batchIdx+1, len(batches), len(entries))

			items := sender.BuildTaskPostBody(entries, cfg.PostbackURL, cfg.Depth)
			resp, err := client.SendBatch(items)
			if err != nil {
				if errors.Is(err, sender.ErrInsufficientFunds) {
					log.Printf("[FATAL] Insufficient funds! Stopping all batches.")
					cancel()
					return
				}
				log.Printf("[ERROR] Batch %d failed: %v", batchIdx+1, err)
				failCount.Add(int64(len(entries)))
				return
			}

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
