// Package scrape provides advanced web scraping tools for agent-go.
//
// The pack uses an interface-based approach, allowing any scraping engine
// (headless browser, HTTP client, etc.) to be plugged in. It builds on
// basic browser capabilities to provide structured data extraction.
package scrape

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Scraper provides web scraping and data extraction capabilities.
type Scraper interface {
	// ExtractStructured extracts structured data from a URL using selectors.
	ExtractStructured(ctx context.Context, url string, schema ExtractionSchema) (*ExtractionResult, error)

	// FollowPagination navigates paginated content and collects results.
	FollowPagination(ctx context.Context, url string, opts PaginationOptions) (*PaginationResult, error)

	// ExtractLinks extracts all links from a page with optional filtering.
	ExtractLinks(ctx context.Context, url string, opts LinkOptions) ([]Link, error)

	// ExtractText extracts clean text content from a URL.
	ExtractText(ctx context.Context, url string) (*TextResult, error)
}

// AuthProvider handles authentication for protected pages.
type AuthProvider interface {
	// Authenticate performs authentication and returns session state.
	Authenticate(ctx context.Context, url string, credentials Credentials) (*Session, error)
}

// RobotsChecker verifies robots.txt compliance.
type RobotsChecker interface {
	// IsAllowed checks if scraping the URL is permitted by robots.txt.
	IsAllowed(ctx context.Context, url, userAgent string) (bool, error)
}

// ExtractionSchema defines what data to extract.
type ExtractionSchema struct {
	Fields []FieldSelector `json:"fields"`
}

// FieldSelector defines how to extract a specific field.
type FieldSelector struct {
	Name     string `json:"name"`
	Selector string `json:"selector"` // CSS selector
	Attribute string `json:"attribute,omitempty"` // e.g., "href", "src"; empty means text content
	Multiple bool   `json:"multiple,omitempty"` // extract all matching elements
}

// ExtractionResult contains extracted structured data.
type ExtractionResult struct {
	URL    string           `json:"url"`
	Data   map[string]any   `json:"data"`
	Items  []map[string]any `json:"items,omitempty"`
	Errors []string         `json:"errors,omitempty"`
}

// PaginationOptions configures pagination following.
type PaginationOptions struct {
	NextSelector string          `json:"next_selector"` // CSS selector for next page link
	ItemSelector string          `json:"item_selector"` // CSS selector for items
	Schema       ExtractionSchema `json:"schema"`
	MaxPages     int             `json:"max_pages,omitempty"`
	Delay        int             `json:"delay_ms,omitempty"`
}

// PaginationResult contains paginated extraction results.
type PaginationResult struct {
	URL        string           `json:"url"`
	Pages      int              `json:"pages"`
	TotalItems int              `json:"total_items"`
	Items      []map[string]any `json:"items"`
}

// LinkOptions configures link extraction.
type LinkOptions struct {
	Selector   string   `json:"selector,omitempty"`
	MatchHost  bool     `json:"match_host,omitempty"`
	Patterns   []string `json:"patterns,omitempty"`
}

// Link represents an extracted hyperlink.
type Link struct {
	URL    string `json:"url"`
	Text   string `json:"text,omitempty"`
	Rel    string `json:"rel,omitempty"`
}

// TextResult contains extracted text content.
type TextResult struct {
	URL     string `json:"url"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text"`
	WordCount int  `json:"word_count"`
}

// Credentials holds authentication credentials.
type Credentials struct {
	Type     string         `json:"type"` // "basic", "form", "oauth", "cookie"
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`
	Token    string         `json:"token,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
}

// Session holds authenticated session state.
type Session struct {
	Cookies map[string]string `json:"cookies,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Token   string            `json:"token,omitempty"`
}

// Config holds scrape pack configuration.
type Config struct {
	// Scraper provides scraping capabilities (required).
	Scraper Scraper

	// Auth is an optional authentication provider.
	Auth AuthProvider

	// Robots is an optional robots.txt checker.
	Robots RobotsChecker

	// RespectRobots enables robots.txt compliance checking.
	RespectRobots bool

	// DefaultUserAgent is the user agent string.
	DefaultUserAgent string

	// MaxPagesPerRequest limits pagination depth.
	MaxPagesPerRequest int
}

// Pack returns the web scraping tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &scrapePack{cfg: cfg}
	if p.cfg.DefaultUserAgent == "" {
		p.cfg.DefaultUserAgent = "AgentGoBot/1.0"
	}
	if p.cfg.MaxPagesPerRequest == 0 {
		p.cfg.MaxPagesPerRequest = 50
	}

	tools := []tool.Tool{
		p.extractStructuredTool(),
		p.followPaginationTool(),
		p.extractLinksTool(),
		p.extractTextTool(),
		p.checkRobotsTool(),
	}

	if cfg.Auth != nil {
		tools = append(tools, p.authenticateTool())
	}

	return pack.NewBuilder("scrape").
		WithDescription("Advanced web scraping tools: structured extraction, pagination, authentication, robots.txt compliance").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type scrapePack struct {
	cfg Config
}

func (p *scrapePack) extractStructuredTool() tool.Tool {
	return tool.NewBuilder("scrape_extract_structured").
		WithDescription("Extract structured data from a web page using CSS selectors").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL    string          `json:"url"`
				Fields []FieldSelector `json:"fields"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}
			if len(in.Fields) == 0 {
				return tool.Result{}, fmt.Errorf("fields is required")
			}

			if p.cfg.RespectRobots && p.cfg.Robots != nil {
				allowed, err := p.cfg.Robots.IsAllowed(ctx, in.URL, p.cfg.DefaultUserAgent)
				if err != nil {
					return tool.Result{}, fmt.Errorf("robots.txt check failed: %w", err)
				}
				if !allowed {
					return tool.Result{}, fmt.Errorf("scraping %s is not allowed by robots.txt", in.URL)
				}
			}

			result, err := p.cfg.Scraper.ExtractStructured(ctx, in.URL, ExtractionSchema{Fields: in.Fields})
			if err != nil {
				return tool.Result{}, fmt.Errorf("extraction failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *scrapePack) followPaginationTool() tool.Tool {
	return tool.NewBuilder("scrape_follow_pagination").
		WithDescription("Navigate paginated content and collect results across pages").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL          string          `json:"url"`
				NextSelector string          `json:"next_selector"`
				ItemSelector string          `json:"item_selector"`
				Fields       []FieldSelector `json:"fields"`
				MaxPages     int             `json:"max_pages,omitempty"`
				DelayMS      int             `json:"delay_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}
			if in.NextSelector == "" {
				return tool.Result{}, fmt.Errorf("next_selector is required")
			}

			maxPages := in.MaxPages
			if maxPages == 0 || maxPages > p.cfg.MaxPagesPerRequest {
				maxPages = p.cfg.MaxPagesPerRequest
			}

			result, err := p.cfg.Scraper.FollowPagination(ctx, in.URL, PaginationOptions{
				NextSelector: in.NextSelector,
				ItemSelector: in.ItemSelector,
				Schema:       ExtractionSchema{Fields: in.Fields},
				MaxPages:     maxPages,
				Delay:        in.DelayMS,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("pagination failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *scrapePack) extractLinksTool() tool.Tool {
	return tool.NewBuilder("scrape_extract_links").
		WithDescription("Extract all links from a web page with optional filtering").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL       string   `json:"url"`
				Selector  string   `json:"selector,omitempty"`
				MatchHost bool     `json:"match_host,omitempty"`
				Patterns  []string `json:"patterns,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}

			links, err := p.cfg.Scraper.ExtractLinks(ctx, in.URL, LinkOptions{
				Selector:  in.Selector,
				MatchHost: in.MatchHost,
				Patterns:  in.Patterns,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("link extraction failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"url":   in.URL,
				"count": len(links),
				"links": links,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *scrapePack) extractTextTool() tool.Tool {
	return tool.NewBuilder("scrape_extract_text").
		WithDescription("Extract clean text content from a web page").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}

			result, err := p.cfg.Scraper.ExtractText(ctx, in.URL)
			if err != nil {
				return tool.Result{}, fmt.Errorf("text extraction failed: %w", err)
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *scrapePack) checkRobotsTool() tool.Tool {
	return tool.NewBuilder("scrape_check_robots").
		WithDescription("Check if scraping a URL is allowed by robots.txt").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL       string `json:"url"`
				UserAgent string `json:"user_agent,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}

			ua := in.UserAgent
			if ua == "" {
				ua = p.cfg.DefaultUserAgent
			}

			if p.cfg.Robots == nil {
				output, _ := json.Marshal(map[string]any{
					"url":     in.URL,
					"allowed": true,
					"note":    "no robots checker configured, assuming allowed",
				})
				return tool.Result{Output: output}, nil
			}

			allowed, err := p.cfg.Robots.IsAllowed(ctx, in.URL, ua)
			if err != nil {
				return tool.Result{}, fmt.Errorf("robots check failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"url":        in.URL,
				"user_agent": ua,
				"allowed":    allowed,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (p *scrapePack) authenticateTool() tool.Tool {
	return tool.NewBuilder("scrape_authenticate").
		WithDescription("Authenticate with a website for scraping protected content").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				URL         string            `json:"url"`
				Type        string            `json:"type"`
				Username    string            `json:"username,omitempty"`
				Password    string            `json:"password,omitempty"`
				Token       string            `json:"token,omitempty"`
				Fields      map[string]string `json:"fields,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}
			if in.Type == "" {
				return tool.Result{}, fmt.Errorf("type is required (basic, form, oauth, cookie)")
			}

			session, err := p.cfg.Auth.Authenticate(ctx, in.URL, Credentials{
				Type:     in.Type,
				Username: in.Username,
				Password: in.Password,
				Token:    in.Token,
				Fields:   in.Fields,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("authentication failed: %w", err)
			}

			output, _ := json.Marshal(map[string]any{
				"url":     in.URL,
				"success": true,
				"session": session,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
