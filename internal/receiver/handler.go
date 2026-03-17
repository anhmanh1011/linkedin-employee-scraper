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
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

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
