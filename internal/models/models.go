package models

import "time"

type CompanyEntry struct {
	Domain  string
	Company string
}

type DfsTaskPostItem struct {
	Keyword      string `json:"keyword"`
	LocationCode int    `json:"location_code"`
	LanguageCode string `json:"language_code"`
	Depth        int    `json:"depth"`
	Tag          string `json:"tag"`
	PostbackURL  string `json:"postback_url"`
	PostbackData string `json:"postback_data"`
}

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

type DfsPostBack struct {
	Version       string    `json:"version"`
	StatusCode    int       `json:"status_code"`
	StatusMessage string    `json:"status_message"`
	Time          string    `json:"time"`
	Cost          float64   `json:"cost"`
	TasksCount    int       `json:"tasks_count"`
	TasksError    int       `json:"tasks_error"`
	Tasks         []DfsTask `json:"tasks"`
}

type DfsTask struct {
	ID            string      `json:"id"`
	StatusCode    int         `json:"status_code"`
	StatusMessage string      `json:"status_message"`
	Time          string      `json:"time"`
	Cost          float64     `json:"cost"`
	ResultCount   int         `json:"result_count"`
	Path          []string    `json:"path"`
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
