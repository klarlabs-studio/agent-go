// Package main demonstrates the agent-go runtime with web scraping tools.
//
// This example shows:
// - HTTP-based tools for web scraping
// - Content extraction patterns
// - State-driven exploration and data gathering
// - LLM integration with multiple providers
//
// Run with: go run ./example/webscraper
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/logging"
	api "go.klarlabs.de/agent/interfaces/api"
)

func main() {
	// Initialize logging
	logging.Init(logging.Config{
		Level:   "info",
		Format:  "console",
		NoColor: false,
	})

	fmt.Printf("=== Web Scraper Agent Example ===\n\n")

	// Run the example
	if err := runExample(); err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Example completed successfully! ===\n")
}

func runExample() error {
	// Create tool registry with web scraping tools
	registry := api.NewToolRegistry()
	if err := registry.Register(NewFetchURLTool()); err != nil {
		return fmt.Errorf("failed to register fetch_url: %w", err)
	}
	if err := registry.Register(NewExtractTextTool()); err != nil {
		return fmt.Errorf("failed to register extract_text: %w", err)
	}
	if err := registry.Register(NewExtractLinksTool()); err != nil {
		return fmt.Errorf("failed to register extract_links: %w", err)
	}
	if err := registry.Register(NewSearchContentTool()); err != nil {
		return fmt.Errorf("failed to register search_content: %w", err)
	}

	// Configure tool eligibility per state
	eligibility := api.NewToolEligibility()
	// All scraping tools are read-only, allow in explore and validate
	eligibility.Allow(agent.StateExplore, "fetch_url")
	eligibility.Allow(agent.StateExplore, "extract_text")
	eligibility.Allow(agent.StateExplore, "extract_links")
	eligibility.Allow(agent.StateExplore, "search_content")
	// Act state for processing
	eligibility.Allow(agent.StateAct, "fetch_url")
	eligibility.Allow(agent.StateAct, "extract_text")
	eligibility.Allow(agent.StateAct, "extract_links")
	eligibility.Allow(agent.StateAct, "search_content")
	// Validate state for verification
	eligibility.Allow(agent.StateValidate, "fetch_url")
	eligibility.Allow(agent.StateValidate, "extract_text")
	eligibility.Allow(agent.StateValidate, "search_content")

	// Create a scripted planner that simulates a web scraping workflow:
	// 1. Fetch a webpage
	// 2. Extract links
	// 3. Search for specific content
	// 4. Compile results
	planner := api.NewScriptedPlanner(
		// Start: transition from intake to explore
		api.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    api.NewTransitionDecision(agent.StateExplore, "Begin web exploration"),
		},
		// Explore: fetch the main page
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("fetch_url",
				json.RawMessage(`{"url": "https://example.com"}`),
				"Fetch homepage content"),
		},
		// Explore: extract text content
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("extract_text",
				json.RawMessage(`{"html": "<html><body><h1>Example Domain</h1><p>This domain is for illustrative examples.</p></body></html>", "selector": ""}`),
				"Extract text from page"),
		},
		// Explore: extract links
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("extract_links",
				json.RawMessage(`{"html": "<html><body><a href='https://www.iana.org/domains/example'>More information...</a></body></html>", "base_url": "https://example.com"}`),
				"Extract links from page"),
		},
		// Explore: transition to decide
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    api.NewTransitionDecision(agent.StateDecide, "Evaluate gathered data"),
		},
		// Decide: transition to act
		api.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    api.NewTransitionDecision(agent.StateAct, "Process scraped content"),
		},
		// Act: search for specific content
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision: api.NewCallToolDecision("search_content",
				json.RawMessage(`{"text": "This domain is for use in illustrative examples in documents. You may use this domain in literature without prior coordination or asking for permission.", "pattern": "example"}`),
				"Search for keyword"),
		},
		// Act: transition to validate
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    api.NewTransitionDecision(agent.StateValidate, "Verify extraction results"),
		},
		// Validate: confirm content extraction
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewCallToolDecision("extract_text",
				json.RawMessage(`{"html": "<html><body><h1>Example Domain</h1></body></html>", "selector": "h1"}`),
				"Verify title extraction"),
		},
		// Finish
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewFinishDecision("Successfully scraped and analyzed web content",
				json.RawMessage(`{"url": "https://example.com", "title": "Example Domain", "links_found": 1, "status": "complete"}`)),
		},
	)

	// Create the engine
	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(planner),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithBudgets(map[string]int{
			"tool_calls":    15,
			"http_requests": 10,
		}),
		api.WithMaxSteps(25),
	)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	// Run the agent
	ctx := context.Background()
	run, err := engine.Run(ctx, "Scrape and analyze content from example.com")
	if err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	// Print results
	fmt.Printf("\n--- Run Results ---\n")
	fmt.Printf("Run ID: %s\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Final State: %s\n", run.CurrentState)
	fmt.Printf("Duration: %v\n", run.Duration())

	if run.Result != nil {
		var result map[string]interface{}
		if err := json.Unmarshal(run.Result, &result); err == nil {
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Printf("Result: %s\n", resultJSON)
		}
	}

	fmt.Printf("\n--- Evidence Trail ---\n")
	for i, ev := range run.Evidence {
		fmt.Printf("%d. [%s] %s\n", i+1, ev.Type, ev.Source)
		if len(ev.Content) > 0 {
			var content interface{}
			if err := json.Unmarshal(ev.Content, &content); err == nil {
				switch v := content.(type) {
				case string:
					if len(v) > 200 {
						v = v[:200] + "..."
					}
					fmt.Printf("   Content: %s\n", v)
				case map[string]interface{}:
					contentJSON, _ := json.MarshalIndent(v, "   ", "  ")
					fmt.Printf("   Content: %s\n", contentJSON)
				}
			}
		}
	}

	return nil
}
