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
