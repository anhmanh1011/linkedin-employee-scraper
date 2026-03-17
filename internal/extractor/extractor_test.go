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
