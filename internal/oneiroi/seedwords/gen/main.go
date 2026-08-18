// Command gen is a one-off, offline generator for internal/oneiroi/seedwords/words.json.
//
// It is NOT part of the dreamland command tree, is NOT run in CI, and is NOT invoked at
// runtime by `dreamland oneiroi seed`/`revise`/`fork` — see design.md's "Word list is
// generated once, offline, and checked in" decision. Run it manually, from the repo root,
// only when the checked-in word pool needs a refresh from the source Wikipedia page:
//
//	go run ./internal/oneiroi/seedwords/gen
//
// It fetches the raw wikitext of
// https://en.wikipedia.org/wiki/List_of_U.S._Department_of_Defense_and_partner_code_names,
// parses the "List of code names" section's bulleted entries, extracts each entry's code
// name (the phrase before its first " – "/" — "/" - " dash separator), splits multi-word
// code names on whitespace, and writes the deduplicated, lowercased set of tokens to
// internal/oneiroi/seedwords/words.json as {"words": [...]}, sorted for deterministic diffs.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const sourceURL = "https://en.wikipedia.org/w/index.php?title=List_of_U.S._Department_of_Defense_and_partner_code_names&action=raw"

func main() {
	body, err := fetch(sourceURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen: fetch:", err)
		os.Exit(1)
	}

	words := extractWords(body)
	if len(words) == 0 {
		fmt.Fprintln(os.Stderr, "gen: extracted zero words; source page structure may have changed")
		os.Exit(1)
	}

	out := struct {
		Words []string `json:"words"`
	}{Words: words}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen: marshal:", err)
		os.Exit(1)
	}

	target := outputPath()
	if err := os.WriteFile(target, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen: write:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "gen: wrote %d words to %s\n", len(words), target)
}

// outputPath resolves words.json relative to this source file's own directory, so the
// generator writes to the correct location regardless of the caller's working directory.
func outputPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "words.json")
}

func fetch(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	// Wikimedia's edge rejects requests with no descriptive User-Agent (its API etiquette
	// policy); Go's default "Go-http-client" UA gets a 403.
	req.Header.Set("User-Agent", "dreamland-oneiroi-seedwords-gen/1.0 (one-off offline generator; https://github.com/x86ed/dreamland)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, url)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var (
	reListStart   = regexp.MustCompile(`(?m)^==List of code names==\s*$`)
	reSectionEnd  = regexp.MustCompile(`(?m)^==(See also|Notes|References|External links)==\s*$`)
	reBulletLine  = regexp.MustCompile(`^[*:#]+\s*(.*)$`)
	rePipedLink   = regexp.MustCompile(`\[\[[^\]|]*\|([^\]]*)\]\]`)
	rePlainLink   = regexp.MustCompile(`\[\[([^\]]*)\]\]`)
	reTemplate    = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	reRefPair     = regexp.MustCompile(`(?s)<ref[^>]*>.*?</ref>`)
	reRefSelf     = regexp.MustCompile(`<ref[^>]*/>`)
	reHTMLTag     = regexp.MustCompile(`<[^>]+>`)
	reParenthetic = regexp.MustCompile(`\([^()]*\)`)
	reBoldItalic  = regexp.MustCompile(`'{2,}`)
	reDashSplit   = regexp.MustCompile(`\s[–—-]\s`)
	reOpPrefix    = regexp.MustCompile(`(?i)^(operation|operations|exercise|exercises)\s+`)
	reNonLetter   = regexp.MustCompile(`[^a-zA-Z]`)
)

// extractWords parses the raw wikitext body and returns the deduplicated, lowercased,
// sorted set of individual word tokens split from every multi-word code name found in
// the "List of code names" section's bulleted entries.
func extractWords(body string) []string {
	loc := reListStart.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	section := body[loc[1]:]
	if end := reSectionEnd.FindStringIndex(section); end != nil {
		section = section[:end[0]]
	}

	seen := make(map[string]bool)
	for _, line := range strings.Split(section, "\n") {
		m := reBulletLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		codeName := extractCodeName(m[1])
		tokens := strings.Fields(codeName)
		if len(tokens) < 2 {
			continue // only multi-word code names contribute tokens
		}
		for _, tok := range tokens {
			word := strings.ToLower(reNonLetter.ReplaceAllString(tok, ""))
			if len(word) < 2 {
				continue
			}
			seen[word] = true
		}
	}

	words := make([]string, 0, len(seen))
	for w := range seen {
		words = append(words, w)
	}
	sort.Strings(words)
	return words
}

// extractCodeName cleans one bulleted line's wikitext and returns the code name phrase —
// the text before the entry's first dash separator, with wiki-link/template/ref/HTML
// markup, parenthetical asides, and leading "Operation"/"Exercise" labels stripped.
// Returns "" for a line with no dash separator at all: without one, there is no
// reliable boundary between the code name and its free-text description, and treating
// the whole line as the "code name" would pollute the pool with prose (e.g. "was the
// code name of a ... warrantless surveillance program ...").
func extractCodeName(raw string) string {
	s := raw
	s = reRefPair.ReplaceAllString(s, "")
	s = reRefSelf.ReplaceAllString(s, "")
	s = reTemplate.ReplaceAllString(s, "")
	s = rePipedLink.ReplaceAllString(s, "$1")
	s = rePlainLink.ReplaceAllString(s, "$1")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = reBoldItalic.ReplaceAllString(s, "")

	idx := reDashSplit.FindStringIndex(s)
	if idx == nil {
		return ""
	}
	s = s[:idx[0]]

	s = reParenthetic.ReplaceAllString(s, "")
	s = reOpPrefix.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
