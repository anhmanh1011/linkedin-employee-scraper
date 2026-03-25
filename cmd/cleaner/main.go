package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"unicode"
)

var (
	// Credentials/titles after name: ", CPA", ", Ph.D", ", LCSW", ", PA-C", etc.
	credentialRe = regexp.MustCompile(`[,]\s*[A-Z][A-Za-z.®]{1,10}(\s+[A-Za-z.®]{1,10})*\s*$`)
	// "Dr. ", "Prof. " prefix
	honorificRe = regexp.MustCompile(`^(Dr\.|Prof\.|Mr\.|Mrs\.|Ms\.|Ing\.|Dra\.)\s+`)
	// Only letters, spaces, hyphens, apostrophes, periods in names
	validNameRe = regexp.MustCompile(`^[A-Za-zÀ-ÿ\s\-'.]+$`)
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: cleaner <input_file> <output_file>")
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	in, err := os.Open(inputFile)
	if err != nil {
		log.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	out, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Failed to create output: %v", err)
	}
	defer out.Close()

	writer := bufio.NewWriter(out)
	defer writer.Flush()

	scanner := bufio.NewScanner(in)

	var total, kept, dropped int

	for scanner.Scan() {
		total++
		line := scanner.Text()

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			dropped++
			continue
		}

		domain := parts[0]
		name := parts[1]

		cleaned := cleanName(name)
		if cleaned == "" {
			dropped++
			continue
		}

		fmt.Fprintf(writer, "%s|%s\n", domain, cleaned)
		kept++
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Scanner error: %v", err)
	}

	log.Printf("Total: %d, Kept: %d, Dropped: %d", total, kept, dropped)
}

func cleanName(raw string) string {
	name := raw

	// Remove zero-width chars, RTL/LTR marks
	name = strings.Map(func(r rune) rune {
		if r == 0x200F || r == 0x200E || r == 0x200B || r == 0x200C || r == 0x200D || r == 0xFEFF {
			return -1
		}
		return r
	}, name)

	name = strings.TrimSpace(name)

	// Split on en-dash or em-dash, take first part (name before title/description)
	for _, sep := range []string{" – ", " — ", " - "} {
		if idx := strings.Index(name, sep); idx > 0 {
			name = name[:idx]
		}
	}

	// Split on " at ", " bei ", " chez ", " @" (job descriptions)
	for _, sep := range []string{" at ", " bei ", " chez ", " @ "} {
		if idx := strings.Index(strings.ToLower(name), sep); idx > 0 {
			name = name[:idx]
		}
	}

	// Remove trailing credentials: ", CPA", ", Ph.D", etc.
	name = credentialRe.ReplaceAllString(name, "")

	// Remove known credential tokens anywhere in name
	credTokens := regexp.MustCompile(`(?i)\b(PA-C|LPC-IT|LCSW|CRPC|MSW|MSL|MBA|CFP®?|CPA|SPHR|Ph\.?D|B\.A\.A|Pl\.?\s*Fin)\b\.?`)
	name = credTokens.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)

	// Remove honorific prefixes
	name = honorificRe.ReplaceAllString(name, "")

	// Remove "..." at end
	name = strings.TrimRight(name, ".")
	name = strings.TrimSpace(name)

	// Handle "Lastname, Firstname" format -> "Firstname Lastname"
	if parts := strings.SplitN(name, ", ", 2); len(parts) == 2 {
		first := strings.TrimSpace(parts[1])
		last := strings.TrimSpace(parts[0])
		if isSimpleName(first) && isSimpleName(last) {
			name = first + " " + last
		}
	}

	// Remove parenthesized parts: "(Desbordes)", "(TX)"
	parenRe := regexp.MustCompile(`\s*\([^)]*\)\s*`)
	name = parenRe.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Reject if not valid characters
	if !validNameRe.MatchString(name) {
		return ""
	}

	// Split into words
	words := strings.Fields(name)

	// Reject single word names
	if len(words) < 2 {
		return ""
	}

	// Reject if more than 5 words (likely not a name)
	if len(words) > 5 {
		return ""
	}

	// Reject if any word is too long (>20 chars, likely not a name)
	for _, w := range words {
		if len([]rune(w)) > 20 {
			return ""
		}
	}

	// Reject if contains common non-name indicators
	lower := strings.ToLower(name)
	rejectPhrases := []string{
		"llc", "inc", "ltd", "corp", "gmbh", "pvt", "pllc",
		"company", "service", "group", "associate", "partner",
		"design", "studio", "agency", "solution", "consulting",
		"looking for", "actively", "passionate",
		"senior", "junior", "manager", "director", "ceo", "cto", "cfo",
		"engineer", "developer", "specialist", "consultant",
		"immobilier", "directeur", "président",
	}
	for _, phrase := range rejectPhrases {
		if strings.Contains(lower, phrase) {
			return ""
		}
	}

	// Reject if starts with lowercase (likely not a name)
	firstRune := []rune(words[0])[0]
	if unicode.IsLower(firstRune) {
		return ""
	}

	// Capitalize properly
	for i, w := range words {
		words[i] = capitalizeWord(w)
	}

	return strings.Join(words, " ")
}

func capitalizeWord(w string) string {
	runes := []rune(strings.ToLower(w))
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	// Handle hyphenated names: "De-rooij" -> "De-Rooij"
	for i, r := range runes {
		if r == '-' && i+1 < len(runes) {
			runes[i+1] = unicode.ToUpper(runes[i+1])
		}
	}
	return string(runes)
}

func isSimpleName(s string) bool {
	words := strings.Fields(s)
	return len(words) >= 1 && len(words) <= 3 && validNameRe.MatchString(s)
}
