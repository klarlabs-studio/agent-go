# Web Scraper Agent Example

Demonstrates the agent-go runtime with HTTP-based web scraping tools.

## What It Shows

- HTTP tools for fetching and parsing web pages
- Content extraction with CSS selectors and link following
- State-driven exploration: intake -> explore (fetch) -> act (extract) -> validate -> done
- LLM planner integration for autonomous scraping decisions

## Tools

| Tool | State | Description |
|------|-------|-------------|
| `fetch_page` | explore | Fetch a URL and return HTML content |
| `extract_text` | act | Extract text content from HTML |
| `extract_links` | explore | Extract all links from a page |
| `save_content` | act | Save extracted content to output |

## Running

```bash
# Requires an LLM provider API key (e.g., OPENAI_API_KEY)
go run ./example/webscraper
```

## Architecture

The agent follows the canonical state graph:

1. **intake** — Receives the scraping goal (URL + what to extract)
2. **explore** — Fetches pages and discovers links
3. **decide** — LLM chooses which links to follow or content to extract
4. **act** — Extracts and saves content
5. **validate** — Confirms extraction completeness
6. **done** — Returns aggregated results
