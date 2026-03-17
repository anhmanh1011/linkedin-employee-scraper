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

	resultCh := make(chan string, 10000)
	var namesWritten atomic.Int64

	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		writeResults(cfg.OutputFile, resultCh, &namesWritten)
	}()

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

	srv := &http.Server{
		Addr:         ":" + cfg.ReceiverPort,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
