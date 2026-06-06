// Package browser provides browser automation tools using Chrome DevTools Protocol.
package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Config holds browser pack configuration.
type Config struct {
	// Headless runs browser in headless mode (default: true)
	Headless bool
	// Timeout is the default timeout for operations
	Timeout time.Duration
	// UserAgent is the custom user agent string
	UserAgent string
	// ProxyURL is the proxy server URL
	ProxyURL string
	// WindowWidth is the browser window width
	WindowWidth int
	// WindowHeight is the browser window height
	WindowHeight int
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Headless:     true,
		Timeout:      30 * time.Second,
		WindowWidth:  1920,
		WindowHeight: 1080,
	}
}

type browserPack struct {
	cfg       Config
	allocator context.Context
	cancel    context.CancelFunc
}

// Pack creates a new browser tools pack.
func Pack(cfg Config) *pack.Pack {
	p := &browserPack{cfg: cfg}

	return pack.NewBuilder("browser").
		WithDescription("Browser automation tools for web scraping, testing, and interaction").
		WithVersion("1.0.0").
		AddTools(
			p.navigateTool(),
			p.getHTMLTool(),
			p.getTextTool(),
			p.screenshotTool(),
			p.clickTool(),
			p.typeTool(),
			p.submitTool(),
			p.waitForTool(),
			p.evaluateTool(),
			p.getAttributeTool(),
			p.setAttributeTool(),
			p.querySelectorTool(),
			p.querySelectorAllTool(),
			p.scrollTool(),
			p.hoverTool(),
			p.getCookiesTool(),
			p.setCookieTool(),
			p.clearCookiesTool(),
			p.getURLTool(),
			p.goBackTool(),
			p.goForwardTool(),
			p.reloadTool(),
			p.setViewportTool(),
			p.pdfTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func (p *browserPack) getContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(p.cfg.WindowWidth, p.cfg.WindowHeight),
	}

	if p.cfg.Headless {
		opts = append(opts, chromedp.Headless)
	}
	if p.cfg.UserAgent != "" {
		opts = append(opts, chromedp.UserAgent(p.cfg.UserAgent))
	}
	if p.cfg.ProxyURL != "" {
		opts = append(opts, chromedp.ProxyServer(p.cfg.ProxyURL))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	cancel := func() {
		browserCancel()
		allocCancel()
	}

	return browserCtx, cancel, nil
}

// navigateTool navigates to a URL.
func (p *browserPack) navigateTool() tool.Tool {
	return tool.NewBuilder("browser_navigate").
		WithDescription("Navigate to a URL in the browser").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL     string `json:"url"`
				Wait    string `json:"wait,omitempty"` // networkIdle, load, domContentLoaded
				Timeout int    `json:"timeout,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.URL == "" {
				return tool.Result{}, fmt.Errorf("url is required")
			}

			timeout := p.cfg.Timeout
			if params.Timeout > 0 {
				timeout = time.Duration(params.Timeout) * time.Millisecond
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			actions = append(actions, chromedp.Navigate(params.URL))

			if params.Wait == "networkIdle" {
				actions = append(actions, chromedp.WaitReady("body"))
			}

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("navigation failed: %w", err)
			}

			var currentURL string
			if err := chromedp.Run(browserCtx, chromedp.Location(&currentURL)); err != nil {
				return tool.Result{}, fmt.Errorf("failed to get URL: %w", err)
			}

			result := map[string]interface{}{
				"url":     currentURL,
				"success": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getHTMLTool gets HTML content from an element or the page.
func (p *browserPack) getHTMLTool() tool.Tool {
	return tool.NewBuilder("browser_get_html").
		WithDescription("Get HTML content of an element or the entire page").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector,omitempty"`
				Outer    bool   `json:"outer,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var html string
			selector := params.Selector
			if selector == "" {
				selector = "html"
			}

			if params.Outer {
				actions = append(actions, chromedp.OuterHTML(selector, &html))
			} else {
				actions = append(actions, chromedp.InnerHTML(selector, &html))
			}

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("failed to get HTML: %w", err)
			}

			result := map[string]interface{}{
				"html":     html,
				"selector": selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getTextTool gets text content from an element.
func (p *browserPack) getTextTool() tool.Tool {
	return tool.NewBuilder("browser_get_text").
		WithDescription("Get text content of an element").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var text string
			actions = append(actions, chromedp.Text(params.Selector, &text))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("failed to get text: %w", err)
			}

			result := map[string]interface{}{
				"text":     text,
				"selector": params.Selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// screenshotTool takes a screenshot of the page or element.
func (p *browserPack) screenshotTool() tool.Tool {
	return tool.NewBuilder("browser_screenshot").
		WithDescription("Take a screenshot of the page or a specific element").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector,omitempty"`
				FullPage bool   `json:"full_page,omitempty"`
				Quality  int    `json:"quality,omitempty"` // 0-100 for JPEG
				Format   string `json:"format,omitempty"`  // png or jpeg
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var buf []byte
			if params.Selector != "" {
				actions = append(actions, chromedp.Screenshot(params.Selector, &buf))
			} else if params.FullPage {
				actions = append(actions, chromedp.FullScreenshot(&buf, 90))
			} else {
				actions = append(actions, chromedp.CaptureScreenshot(&buf))
			}

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("screenshot failed: %w", err)
			}

			result := map[string]interface{}{
				"data":   base64.StdEncoding.EncodeToString(buf),
				"format": "png",
				"size":   len(buf),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// clickTool clicks on an element.
func (p *browserPack) clickTool() tool.Tool {
	return tool.NewBuilder("browser_click").
		WithDescription("Click on an element").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector"`
				Button   string `json:"button,omitempty"` // left, right, middle
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			actions = append(actions, chromedp.Click(params.Selector))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("click failed: %w", err)
			}

			result := map[string]interface{}{
				"clicked":  true,
				"selector": params.Selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// typeTool types text into an input field.
func (p *browserPack) typeTool() tool.Tool {
	return tool.NewBuilder("browser_type").
		WithDescription("Type text into an input field").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector"`
				Text     string `json:"text"`
				Clear    bool   `json:"clear,omitempty"` // clear existing value first
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			if params.Clear {
				actions = append(actions, chromedp.Clear(params.Selector))
			}
			actions = append(actions, chromedp.SendKeys(params.Selector, params.Text))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("type failed: %w", err)
			}

			result := map[string]interface{}{
				"typed":    true,
				"selector": params.Selector,
				"length":   len(params.Text),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// submitTool submits a form.
func (p *browserPack) submitTool() tool.Tool {
	return tool.NewBuilder("browser_submit").
		WithDescription("Submit a form").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			actions = append(actions, chromedp.Submit(params.Selector))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("submit failed: %w", err)
			}

			result := map[string]interface{}{
				"submitted": true,
				"selector":  params.Selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// waitForTool waits for an element to appear.
func (p *browserPack) waitForTool() tool.Tool {
	return tool.NewBuilder("browser_wait_for").
		WithDescription("Wait for an element to appear on the page").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Selector string `json:"selector"`
				Visible  bool   `json:"visible,omitempty"`
				Hidden   bool   `json:"hidden,omitempty"`
				Timeout  int    `json:"timeout,omitempty"` // ms
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			timeout := p.cfg.Timeout
			if params.Timeout > 0 {
				timeout = time.Duration(params.Timeout) * time.Millisecond
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, timeout)
			defer cancelTimeout()

			var action chromedp.Action
			if params.Hidden {
				action = chromedp.WaitNotPresent(params.Selector)
			} else if params.Visible {
				action = chromedp.WaitVisible(params.Selector)
			} else {
				action = chromedp.WaitReady(params.Selector)
			}

			if err := chromedp.Run(browserCtx, action); err != nil {
				return tool.Result{}, fmt.Errorf("wait failed: %w", err)
			}

			result := map[string]interface{}{
				"found":    true,
				"selector": params.Selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// evaluateTool executes JavaScript in the browser.
func (p *browserPack) evaluateTool() tool.Tool {
	return tool.NewBuilder("browser_evaluate").
		WithDescription("Execute JavaScript code in the browser").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL        string `json:"url,omitempty"`
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Expression == "" {
				return tool.Result{}, fmt.Errorf("expression is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var result interface{}
			actions = append(actions, chromedp.Evaluate(params.Expression, &result))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("evaluate failed: %w", err)
			}

			output, _ := json.Marshal(map[string]interface{}{
				"result": result,
			})
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getAttributeTool gets an attribute value from an element.
func (p *browserPack) getAttributeTool() tool.Tool {
	return tool.NewBuilder("browser_get_attribute").
		WithDescription("Get an attribute value from an element").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL       string `json:"url,omitempty"`
				Selector  string `json:"selector"`
				Attribute string `json:"attribute"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" || params.Attribute == "" {
				return tool.Result{}, fmt.Errorf("selector and attribute are required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var value string
			var ok bool
			actions = append(actions, chromedp.AttributeValue(params.Selector, params.Attribute, &value, &ok))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("get attribute failed: %w", err)
			}

			result := map[string]interface{}{
				"value":     value,
				"exists":    ok,
				"selector":  params.Selector,
				"attribute": params.Attribute,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setAttributeTool sets an attribute value on an element.
func (p *browserPack) setAttributeTool() tool.Tool {
	return tool.NewBuilder("browser_set_attribute").
		WithDescription("Set an attribute value on an element").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Selector  string `json:"selector"`
				Attribute string `json:"attribute"`
				Value     string `json:"value"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" || params.Attribute == "" {
				return tool.Result{}, fmt.Errorf("selector and attribute are required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			script := fmt.Sprintf(`document.querySelector(%q).setAttribute(%q, %q)`,
				params.Selector, params.Attribute, params.Value)

			if err := chromedp.Run(browserCtx, chromedp.Evaluate(script, nil)); err != nil {
				return tool.Result{}, fmt.Errorf("set attribute failed: %w", err)
			}

			result := map[string]interface{}{
				"set":       true,
				"selector":  params.Selector,
				"attribute": params.Attribute,
				"value":     params.Value,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// querySelectorTool finds a single element matching a selector.
func (p *browserPack) querySelectorTool() tool.Tool {
	return tool.NewBuilder("browser_query_selector").
		WithDescription("Find a single element matching a CSS selector").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var nodes []*cdp.Node
			actions = append(actions, chromedp.Nodes(params.Selector, &nodes, chromedp.ByQuery))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("query failed: %w", err)
			}

			if len(nodes) == 0 {
				result := map[string]interface{}{
					"found":    false,
					"selector": params.Selector,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			node := nodes[0]
			result := map[string]interface{}{
				"found":     true,
				"selector":  params.Selector,
				"tag_name":  strings.ToLower(node.NodeName),
				"node_type": node.NodeType,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// querySelectorAllTool finds all elements matching a selector.
func (p *browserPack) querySelectorAllTool() tool.Tool {
	return tool.NewBuilder("browser_query_selector_all").
		WithDescription("Find all elements matching a CSS selector").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL      string `json:"url,omitempty"`
				Selector string `json:"selector"`
				Limit    int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var nodes []*cdp.Node
			actions = append(actions, chromedp.Nodes(params.Selector, &nodes, chromedp.ByQueryAll))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("query failed: %w", err)
			}

			limit := len(nodes)
			if params.Limit > 0 && params.Limit < limit {
				limit = params.Limit
			}

			elements := make([]map[string]interface{}, 0, limit)
			for i := 0; i < limit; i++ {
				node := nodes[i]
				elements = append(elements, map[string]interface{}{
					"tag_name":  strings.ToLower(node.NodeName),
					"node_type": node.NodeType,
				})
			}

			result := map[string]interface{}{
				"count":    len(nodes),
				"elements": elements,
				"selector": params.Selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// scrollTool scrolls the page or an element.
func (p *browserPack) scrollTool() tool.Tool {
	return tool.NewBuilder("browser_scroll").
		WithDescription("Scroll the page or scroll an element into view").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Selector string `json:"selector,omitempty"`
				X        int    `json:"x,omitempty"`
				Y        int    `json:"y,omitempty"`
				Behavior string `json:"behavior,omitempty"` // smooth or instant
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var script string
			if params.Selector != "" {
				script = fmt.Sprintf(`document.querySelector(%q).scrollIntoView({behavior: %q})`,
					params.Selector, params.Behavior)
			} else {
				behavior := params.Behavior
				if behavior == "" {
					behavior = "instant"
				}
				script = fmt.Sprintf(`window.scrollTo({top: %d, left: %d, behavior: %q})`,
					params.Y, params.X, behavior)
			}

			if err := chromedp.Run(browserCtx, chromedp.Evaluate(script, nil)); err != nil {
				return tool.Result{}, fmt.Errorf("scroll failed: %w", err)
			}

			result := map[string]interface{}{
				"scrolled": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// hoverTool hovers over an element.
func (p *browserPack) hoverTool() tool.Tool {
	return tool.NewBuilder("browser_hover").
		WithDescription("Hover over an element").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Selector string `json:"selector"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Selector == "" {
				return tool.Result{}, fmt.Errorf("selector is required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			if err := chromedp.Run(browserCtx, chromedp.MouseClickXY(0, 0)); err != nil {
				// Ignore, just ensuring we have focus
			}

			script := fmt.Sprintf(`
				const el = document.querySelector(%q);
				if (el) {
					const event = new MouseEvent('mouseenter', {
						bubbles: true,
						cancelable: true,
						view: window
					});
					el.dispatchEvent(event);
				}
			`, params.Selector)

			if err := chromedp.Run(browserCtx, chromedp.Evaluate(script, nil)); err != nil {
				return tool.Result{}, fmt.Errorf("hover failed: %w", err)
			}

			result := map[string]interface{}{
				"hovered":  true,
				"selector": params.Selector,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getCookiesTool gets cookies from the browser.
func (p *browserPack) getCookiesTool() tool.Tool {
	return tool.NewBuilder("browser_get_cookies").
		WithDescription("Get cookies from the browser").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL string `json:"url,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
			}

			var cookies []*network.Cookie
			actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				cookies, err = network.GetCookies().Do(ctx)
				return err
			}))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("get cookies failed: %w", err)
			}

			cookieList := make([]map[string]interface{}, 0, len(cookies))
			for _, c := range cookies {
				cookieList = append(cookieList, map[string]interface{}{
					"name":     c.Name,
					"value":    c.Value,
					"domain":   c.Domain,
					"path":     c.Path,
					"expires":  c.Expires,
					"secure":   c.Secure,
					"httpOnly": c.HTTPOnly,
				})
			}

			result := map[string]interface{}{
				"cookies": cookieList,
				"count":   len(cookies),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setCookieTool sets a cookie in the browser.
func (p *browserPack) setCookieTool() tool.Tool {
	return tool.NewBuilder("browser_set_cookie").
		WithDescription("Set a cookie in the browser").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Name     string  `json:"name"`
				Value    string  `json:"value"`
				Domain   string  `json:"domain"`
				Path     string  `json:"path,omitempty"`
				Expires  float64 `json:"expires,omitempty"`
				Secure   bool    `json:"secure,omitempty"`
				HTTPOnly bool    `json:"http_only,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Name == "" || params.Domain == "" {
				return tool.Result{}, fmt.Errorf("name and domain are required")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			path := params.Path
			if path == "" {
				path = "/"
			}

			err = chromedp.Run(browserCtx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					return network.SetCookie(params.Name, params.Value).
						WithDomain(params.Domain).
						WithPath(path).
						WithSecure(params.Secure).
						WithHTTPOnly(params.HTTPOnly).
						Do(ctx)
				}),
			)
			if err != nil {
				return tool.Result{}, fmt.Errorf("set cookie failed: %w", err)
			}

			result := map[string]interface{}{
				"set":    true,
				"name":   params.Name,
				"domain": params.Domain,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// clearCookiesTool clears all cookies.
func (p *browserPack) clearCookiesTool() tool.Tool {
	return tool.NewBuilder("browser_clear_cookies").
		WithDescription("Clear all cookies from the browser").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			err = chromedp.Run(browserCtx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					return network.ClearBrowserCookies().Do(ctx)
				}),
			)
			if err != nil {
				return tool.Result{}, fmt.Errorf("clear cookies failed: %w", err)
			}

			result := map[string]interface{}{
				"cleared": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// getURLTool gets the current URL.
func (p *browserPack) getURLTool() tool.Tool {
	return tool.NewBuilder("browser_get_url").
		WithDescription("Get the current page URL").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var url string
			if err := chromedp.Run(browserCtx, chromedp.Location(&url)); err != nil {
				return tool.Result{}, fmt.Errorf("get URL failed: %w", err)
			}

			result := map[string]interface{}{
				"url": url,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// goBackTool navigates back in history.
func (p *browserPack) goBackTool() tool.Tool {
	return tool.NewBuilder("browser_go_back").
		WithDescription("Navigate back in browser history").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			if err := chromedp.Run(browserCtx, chromedp.NavigateBack()); err != nil {
				return tool.Result{}, fmt.Errorf("go back failed: %w", err)
			}

			var url string
			if err := chromedp.Run(browserCtx, chromedp.Location(&url)); err != nil {
				return tool.Result{}, fmt.Errorf("get URL failed: %w", err)
			}

			result := map[string]interface{}{
				"url":       url,
				"navigated": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// goForwardTool navigates forward in history.
func (p *browserPack) goForwardTool() tool.Tool {
	return tool.NewBuilder("browser_go_forward").
		WithDescription("Navigate forward in browser history").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			if err := chromedp.Run(browserCtx, chromedp.NavigateForward()); err != nil {
				return tool.Result{}, fmt.Errorf("go forward failed: %w", err)
			}

			var url string
			if err := chromedp.Run(browserCtx, chromedp.Location(&url)); err != nil {
				return tool.Result{}, fmt.Errorf("get URL failed: %w", err)
			}

			result := map[string]interface{}{
				"url":       url,
				"navigated": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// reloadTool reloads the current page.
func (p *browserPack) reloadTool() tool.Tool {
	return tool.NewBuilder("browser_reload").
		WithDescription("Reload the current page").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IgnoreCache bool `json:"ignore_cache,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			if err := chromedp.Run(browserCtx, chromedp.Reload()); err != nil {
				return tool.Result{}, fmt.Errorf("reload failed: %w", err)
			}

			result := map[string]interface{}{
				"reloaded": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// setViewportTool sets the viewport size.
func (p *browserPack) setViewportTool() tool.Tool {
	return tool.NewBuilder("browser_set_viewport").
		WithDescription("Set the browser viewport size").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Width       int     `json:"width"`
				Height      int     `json:"height"`
				DeviceScale float64 `json:"device_scale,omitempty"`
				Mobile      bool    `json:"mobile,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Width <= 0 || params.Height <= 0 {
				return tool.Result{}, fmt.Errorf("width and height must be positive")
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			deviceScale := params.DeviceScale
			if deviceScale <= 0 {
				deviceScale = 1.0
			}

			err = chromedp.Run(browserCtx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					return emulation.SetDeviceMetricsOverride(
						int64(params.Width),
						int64(params.Height),
						deviceScale,
						params.Mobile,
					).Do(ctx)
				}),
			)
			if err != nil {
				return tool.Result{}, fmt.Errorf("set viewport failed: %w", err)
			}

			result := map[string]interface{}{
				"width":        params.Width,
				"height":       params.Height,
				"device_scale": deviceScale,
				"mobile":       params.Mobile,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// pdfTool generates a PDF of the page.
func (p *browserPack) pdfTool() tool.Tool {
	return tool.NewBuilder("browser_pdf").
		WithDescription("Generate a PDF of the current page").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				URL             string  `json:"url,omitempty"`
				Landscape       bool    `json:"landscape,omitempty"`
				PrintBackground bool    `json:"print_background,omitempty"`
				Scale           float64 `json:"scale,omitempty"`
				PaperWidth      float64 `json:"paper_width,omitempty"`  // inches
				PaperHeight     float64 `json:"paper_height,omitempty"` // inches
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			browserCtx, cancel, err := p.getContext(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			defer cancel()

			browserCtx, cancelTimeout := context.WithTimeout(browserCtx, p.cfg.Timeout)
			defer cancelTimeout()

			var actions []chromedp.Action
			if params.URL != "" {
				actions = append(actions, chromedp.Navigate(params.URL))
				actions = append(actions, chromedp.WaitReady("body"))
			}

			var buf []byte
			actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
				paperWidth := params.PaperWidth
				if paperWidth <= 0 {
					paperWidth = 8.5 // Letter
				}
				paperHeight := params.PaperHeight
				if paperHeight <= 0 {
					paperHeight = 11 // Letter
				}
				scale := params.Scale
				if scale <= 0 {
					scale = 1.0
				}

				var err error
				buf, _, err = page.PrintToPDF().
					WithLandscape(params.Landscape).
					WithPrintBackground(params.PrintBackground).
					WithScale(scale).
					WithPaperWidth(paperWidth).
					WithPaperHeight(paperHeight).
					Do(ctx)
				return err
			}))

			if err := chromedp.Run(browserCtx, actions...); err != nil {
				return tool.Result{}, fmt.Errorf("PDF generation failed: %w", err)
			}

			result := map[string]interface{}{
				"data": base64.StdEncoding.EncodeToString(buf),
				"size": len(buf),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Ensure unused imports are valid
var (
	_ = dom.GetOuterHTMLReturns{}
)
