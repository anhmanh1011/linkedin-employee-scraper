package extractor

import "strings"

func ExtractName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	parts := strings.SplitN(title, " - ", 2)
	return strings.TrimSpace(parts[0])
}

func IsLinkedInProfileURL(url string) bool {
	return strings.Contains(url, "linkedin.com/in/")
}
