package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.klarlabs.de/agent/domain/tool"
	api "go.klarlabs.de/agent/interfaces/api"
)

// FetchURLInput is the input schema for the fetch_url tool.
type FetchURLInput struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // seconds
}

// FetchURLOutput is the output schema for the fetch_url tool.
type FetchURLOutput struct {
	StatusCode  int               `json:"status_code"`
	Body        string            `json:"body"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
}

// ExtractTextInput is the input schema for the extract_text tool.
type ExtractTextInput struct {
	HTML     string `json:"html"`
	Selector string `json:"selector,omitempty"` // CSS-like selector (simplified)
}

// ExtractTextOutput is the output schema for the extract_text tool.
type ExtractTextOutput struct {
	Text       string   `json:"text"`
	Paragraphs []string `json:"paragraphs"`
	WordCount  int      `json:"word_count"`
}

// ExtractLinksInput is the input schema for the extract_links tool.
type ExtractLinksInput struct {
	HTML    string `json:"html"`
	BaseURL string `json:"base_url,omitempty"`
}

// ExtractLinksOutput is the output schema for the extract_links tool.
type ExtractLinksOutput struct {
	Links []LinkInfo `json:"links"`
	Count int        `json:"count"`
}

// LinkInfo represents extracted link information.
type LinkInfo struct {
	URL  string `json:"url"`
	Text string `json:"text"`
}

// SearchContentInput is the input schema for the search_content tool.
type SearchContentInput struct {
	Text    string `json:"text"`
	Pattern string `json:"pattern"`
}

// SearchContentOutput is the output schema for the search_content tool.
type SearchContentOutput struct {
	Matches    []MatchInfo `json:"matches"`
	MatchCount int         `json:"match_count"`
	Found      bool        `json:"found"`
}

// MatchInfo represents a search match.
type MatchInfo struct {
	Text     string `json:"text"`
	Position int    `json:"position"`
	Context  string `json:"context"` // surrounding text
}

// NewFetchURLTool creates a tool for fetching URL content.
func NewFetchURLTool() tool.Tool {
	return api.NewToolBuilder("fetch_url").
		WithDescription("Fetches content from a URL via HTTP GET request").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {"type": "string", "description": "URL to fetch"},
				"method": {"type": "string", "description": "HTTP method (default: GET)"},
				"headers": {"type": "object", "description": "Optional HTTP headers"},
				"timeout": {"type": "integer", "description": "Request timeout in seconds (default: 30)"}
			},
			"required": ["url"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"status_code": {"type": "integer", "description": "HTTP status code"},
				"body": {"type": "string", "description": "Response body"},
				"content_type": {"type": "string", "description": "Content-Type header"},
				"headers": {"type": "object", "description": "Response headers"}
			}
		}`))).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in FetchURLInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			// Validate URL
			parsedURL, err := url.Parse(in.URL)
			if err != nil {
				return tool.Result{}, fmt.Errorf("invalid URL: %w", err)
			}
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				return tool.Result{}, fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
			}

			// Set defaults
			method := in.Method
			if method == "" {
				method = http.MethodGet
			}
			timeout := in.Timeout
			if timeout <= 0 {
				timeout = 30
			}

			// Create request
			req, err := http.NewRequestWithContext(ctx, method, in.URL, nil)
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to create request: %w", err)
			}

			// Add headers
			req.Header.Set("User-Agent", "agent-go/1.0 webscraper")
			for key, value := range in.Headers {
				req.Header.Set(key, value)
			}

			// Execute request with timeout
			client := &http.Client{
				Timeout: time.Duration(timeout) * time.Second,
			}
			resp, err := client.Do(req)
			if err != nil {
				return tool.Result{}, fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			// Read body (limit to 1MB)
			body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to read response: %w", err)
			}

			// Extract headers
			headers := make(map[string]string)
			for key := range resp.Header {
				headers[key] = resp.Header.Get(key)
			}

			output := FetchURLOutput{
				StatusCode:  resp.StatusCode,
				Body:        string(body),
				ContentType: resp.Header.Get("Content-Type"),
				Headers:     headers,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewExtractTextTool creates a tool for extracting text from HTML.
func NewExtractTextTool() tool.Tool {
	return api.NewToolBuilder("extract_text").
		WithDescription("Extracts text content from HTML, optionally filtering by selector").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"html": {"type": "string", "description": "HTML content to extract text from"},
				"selector": {"type": "string", "description": "Simple tag selector (e.g., 'p', 'h1', 'div')"}
			},
			"required": ["html"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": {"type": "string", "description": "Extracted text content"},
				"paragraphs": {"type": "array", "description": "Text split by paragraphs"},
				"word_count": {"type": "integer", "description": "Word count"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in ExtractTextInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			html := in.HTML
			var text string

			if in.Selector != "" {
				// Simple tag extraction
				text = extractByTag(html, in.Selector)
			} else {
				// Extract all text
				text = stripHTMLTags(html)
			}

			// Clean up whitespace
			text = cleanWhitespace(text)

			// Split into paragraphs
			paragraphs := splitParagraphs(text)

			// Count words
			wordCount := len(strings.Fields(text))

			output := ExtractTextOutput{
				Text:       text,
				Paragraphs: paragraphs,
				WordCount:  wordCount,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewExtractLinksTool creates a tool for extracting links from HTML.
func NewExtractLinksTool() tool.Tool {
	return api.NewToolBuilder("extract_links").
		WithDescription("Extracts all hyperlinks from HTML content").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"html": {"type": "string", "description": "HTML content to extract links from"},
				"base_url": {"type": "string", "description": "Base URL for resolving relative links"}
			},
			"required": ["html"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"links": {"type": "array", "description": "Extracted links with URL and text"},
				"count": {"type": "integer", "description": "Number of links found"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in ExtractLinksInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			// Parse base URL if provided
			var baseURL *url.URL
			if in.BaseURL != "" {
				var err error
				baseURL, err = url.Parse(in.BaseURL)
				if err != nil {
					return tool.Result{}, fmt.Errorf("invalid base URL: %w", err)
				}
			}

			// Extract links using regex
			links := extractLinks(in.HTML, baseURL)

			output := ExtractLinksOutput{
				Links: links,
				Count: len(links),
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewSearchContentTool creates a tool for searching content.
func NewSearchContentTool() tool.Tool {
	return api.NewToolBuilder("search_content").
		WithDescription("Searches text content for a pattern (regex or plain text)").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"text": {"type": "string", "description": "Text content to search"},
				"pattern": {"type": "string", "description": "Search pattern (plain text or regex)"}
			},
			"required": ["text", "pattern"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"matches": {"type": "array", "description": "Found matches with position and context"},
				"match_count": {"type": "integer", "description": "Number of matches found"},
				"found": {"type": "boolean", "description": "Whether any matches were found"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in SearchContentInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if in.Pattern == "" {
				return tool.Result{}, fmt.Errorf("pattern cannot be empty")
			}

			// Try to compile as regex, fall back to literal search
			re, err := regexp.Compile("(?i)" + in.Pattern)
			if err != nil {
				// Fall back to literal string search
				re = regexp.MustCompile(regexp.QuoteMeta(in.Pattern))
			}

			// Find all matches
			matches := findMatches(in.Text, re)

			output := SearchContentOutput{
				Matches:    matches,
				MatchCount: len(matches),
				Found:      len(matches) > 0,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// Helper functions

// stripHTMLTags removes all HTML tags from content.
func stripHTMLTags(html string) string {
	// Remove script and style elements completely
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = reScript.ReplaceAllString(html, "")
	html = reStyle.ReplaceAllString(html, "")

	// Remove HTML tags
	reTags := regexp.MustCompile(`<[^>]+>`)
	text := reTags.ReplaceAllString(html, " ")

	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	return text
}

// extractByTag extracts content from specific HTML tags.
func extractByTag(html, tag string) string {
	pattern := fmt.Sprintf(`(?is)<%s[^>]*>(.*?)</%s>`, tag, tag)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(html, -1)

	var parts []string
	for _, match := range matches {
		if len(match) > 1 {
			text := stripHTMLTags(match[1])
			text = cleanWhitespace(text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// cleanWhitespace normalizes whitespace in text.
func cleanWhitespace(text string) string {
	// Replace multiple whitespace with single space
	reSpace := regexp.MustCompile(`\s+`)
	text = reSpace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// splitParagraphs splits text into paragraphs.
func splitParagraphs(text string) []string {
	// Split on double newlines or period followed by capital letter
	var paragraphs []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			paragraphs = append(paragraphs, line)
		}
	}
	if len(paragraphs) == 0 && text != "" {
		paragraphs = []string{text}
	}
	return paragraphs
}

// extractLinks extracts hyperlinks from HTML.
func extractLinks(html string, baseURL *url.URL) []LinkInfo {
	// Match <a> tags with href attribute
	re := regexp.MustCompile(`(?is)<a[^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(html, -1)

	var links []LinkInfo
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		href := strings.TrimSpace(match[1])
		text := stripHTMLTags(match[2])
		text = cleanWhitespace(text)

		// Skip empty hrefs, anchors, and dangerous URL schemes
		// CodeQL: go/incomplete-url-scheme-check - must check all dangerous schemes
		if href == "" || strings.HasPrefix(href, "#") ||
			strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "data:") ||
			strings.HasPrefix(href, "vbscript:") {
			continue
		}

		// Resolve relative URLs
		if baseURL != nil && !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
			resolved, err := baseURL.Parse(href)
			if err == nil {
				href = resolved.String()
			}
		}

		// Deduplicate
		if seen[href] {
			continue
		}
		seen[href] = true

		links = append(links, LinkInfo{
			URL:  href,
			Text: text,
		})
	}

	return links
}

// findMatches finds all regex matches with context.
func findMatches(text string, re *regexp.Regexp) []MatchInfo {
	indices := re.FindAllStringIndex(text, -1)
	var matches []MatchInfo

	for _, idx := range indices {
		start := idx[0]
		end := idx[1]
		matchText := text[start:end]

		// Extract context (50 chars before and after)
		contextStart := start - 50
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := end + 50
		if contextEnd > len(text) {
			contextEnd = len(text)
		}
		context := text[contextStart:contextEnd]
		context = cleanWhitespace(context)

		matches = append(matches, MatchInfo{
			Text:     matchText,
			Position: start,
			Context:  context,
		})
	}

	return matches
}
