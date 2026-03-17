# LinkedIn Employee Scraper — Design Spec

## Overview

Go application that integrates with DataForSEO SERP API to extract employee names from LinkedIn search results. Given a list of companies (with domains), it searches Google for `site:linkedin.com/in "Company Name"` and extracts employee names from the results.

## Architecture: Two Separate Binaries

Two independent binaries sharing state via a JSON file:

- **Sender** (`cmd/sender/main.go`): Reads input file, sends tasks to DataForSEO API in batches with concurrency.
- **Receiver** (`cmd/receiver/main.go`): HTTP server that receives postback results from DataForSEO and writes extracted names to output file.

## Project Structure

```
linkedin-employee-scraper/
├── cmd/
│   ├── sender/
│   │   └── main.go
│   └── receiver/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go        # Load config from .env
│   ├── models/
│   │   └── models.go        # Shared structs (DfsPostBack, Task state)
│   ├── store/
│   │   └── store.go         # File-based shared state (JSON)
│   └── extractor/
│       └── extractor.go     # Extract employee names from SERP title
├── data/
│   ├── input.txt            # Input: domain|company name
│   └── output.txt           # Output: domain|employee name
├── .env                     # API credentials + config
├── go.mod
└── go.sum
```

## Input / Output Format

**Input** (`data/input.txt`):
```
talentpool.co.id|Talentpool Indonesia
example.com|Example Corp
```

**Output** (`data/output.txt`):
```
talentpool.co.id|Moch Ichlil
talentpool.co.id|Anita Ratnaningsih
example.com|John Doe
```

## Config (`.env`)

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

## Sender Flow

1. Load `.env` config.
2. Read `state.json` to get previously sent domains (for resume).
3. Read `input.txt` line by line using `bufio.Scanner`.
4. For each line, parse `domain|company`. Skip if domain already in state.
5. Accumulate into batch buffer (max 100 tasks per batch).
6. When batch is full, send via goroutine (max 30 concurrent goroutines using semaphore).
7. Each DataForSEO task_post request contains:
   - `keyword`: `site:linkedin.com/in "Company Name"`
   - `depth`: 700
   - `postback_url`: receiver URL
   - `tag`: domain (so receiver can map results back)
8. On successful batch send, update `state.json` with sent domains.
9. On failure, retry up to 3 times with exponential backoff. If still fails, log error and continue.
10. After all batches sent, print summary.

### DataForSEO API Limits

- 100 tasks per POST request
- 2,000 requests per minute
- 30 concurrent requests max
- Theoretical max: 200,000 tasks/minute

## Receiver Flow

1. Load `.env` config.
2. Start HTTP server (chi router) on configured port.
3. Endpoints:
   - `POST /postback` — receive DataForSEO results
   - `GET /ping` — health check
   - `GET /stats` — count of tasks received and names extracted

### Postback Handler

1. Decode JSON body (support gzip Content-Encoding).
2. Validate top-level `status_code == 20000`. If not, log warning and return OK.
3. For each task in payload:
   - Get `domain` from task's `tag` field.
   - Iterate through `result[].items[]`.
   - For each item where URL contains `linkedin.com/in/`:
     - Extract name: take the part before ` - ` in the `title` field.
     - Trim whitespace.
     - Skip if name is empty.
   - Send `domain|name` pairs to a buffered channel.
4. Return `{"status": "ok"}`.

### Writer Goroutine

A single dedicated goroutine reads from the buffered channel and writes to output file:

```
Handler 1 ──┐
Handler 2 ──┼──→ channel (buffered 10000) ──→ Writer goroutine ──→ file.Write + Sync
Handler 3 ──┘
```

- Only 1 goroutine writes to file — no locks needed.
- Calls `file.Sync()` after each write to flush to disk.
- On graceful shutdown: close channel, writer drains remaining data, closes file.

### Name Extraction Logic

```
Title:  "Moch Ichlil - Managing Director of Talentpool Indonesia"
URL:    "https://www.linkedin.com/in/moch-ichlil-..."

→ Split by " - ", take first part
→ Trim whitespace
→ Result: "Moch Ichlil"
→ Output line: "talentpool.co.id|Moch Ichlil"
```

If title has no ` - `, use the entire title as the name.

## Shared State (`data/state.json`)

```json
{
  "sent_tasks": {
    "example.com": {
      "company": "Example Corp",
      "task_ids": ["task-abc-123"],
      "sent_at": "2026-03-17T10:00:00Z"
    }
  },
  "total_sent": 150,
  "last_batch_at": "2026-03-17T10:05:00Z"
}
```

Sender writes to this file after each successful batch. Receiver uses `tag` field from postback, so it does not need to read state.

## Error Handling

| Scenario | Handling |
|---|---|
| DataForSEO returns status_code != 20000 | Log warning, skip that task |
| Batch send fails (network error) | Retry 3 times with exponential backoff, then log and continue |
| Postback body invalid JSON | Return HTTP 400, log error |
| Title has no ` - ` separator | Use entire title as name |
| Title empty or URL not linkedin.com/in | Skip that item |

## Data Flow Diagram

```
input.txt                          DataForSEO
    │                                  │
    ▼                                  │
 [Sender]                             │
    │                                  │
    ├─ read line by line               │
    ├─ batch 100 tasks                 │
    ├─ POST task_post ─────────────►   │
    │   (keyword: site:linkedin.com/in "Company")
    │   (tag: domain)                  │
    │   (postback_url: receiver)       │
    ├─ update state.json               │
    │                                  │
    │         DataForSEO processes     │
    │                                  │
    │               POST /postback ──────────► [Receiver]
    │                                              │
    │                                   ├─ decode JSON
    │                                   ├─ get domain from tag
    │                                   ├─ extract name from title
    │                                   ├─ channel → writer goroutine
    │                                   ▼
    │                               output.txt
    │                           (domain|employee name)
```

## Dependencies

- `github.com/go-chi/chi/v5` — HTTP router (receiver)
- `github.com/joho/godotenv` — .env file loading
- Standard library for everything else
