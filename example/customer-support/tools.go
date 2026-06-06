package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"go.klarlabs.de/agent/domain/tool"
	api "go.klarlabs.de/agent/interfaces/api"
)

// MockDataStore simulates a customer support backend.
type MockDataStore struct {
	mu        sync.RWMutex
	customers map[string]Customer
	orders    map[string]Order
	articles  []KBArticle
	tickets   []Ticket
	nextTktID int
}

// Customer represents a customer record.
type Customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

// Order represents an order record.
type Order struct {
	ID       string `json:"id"`
	Customer string `json:"customer_id"`
	Status   string `json:"status"`
	Carrier  string `json:"carrier,omitempty"`
	ETA      string `json:"eta,omitempty"`
}

// KBArticle represents a knowledge base article.
type KBArticle struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Tags    []string
}

// Ticket represents a support ticket.
type Ticket struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// NewMockDataStore creates a data store with sample data.
func NewMockDataStore() *MockDataStore {
	return &MockDataStore{
		customers: map[string]Customer{
			"cust_847": {ID: "cust_847", Name: "Jane Smith", Email: "jane@email.com", Tier: "premium"},
			"cust_123": {ID: "cust_123", Name: "John Doe", Email: "john@email.com", Tier: "standard"},
		},
		orders: map[string]Order{
			"38291": {ID: "38291", Customer: "cust_847", Status: "delayed", Carrier: "FedEx", ETA: "2 days"},
			"38292": {ID: "38292", Customer: "cust_123", Status: "delivered", Carrier: "UPS", ETA: ""},
		},
		articles: []KBArticle{
			{ID: "POL-201", Title: "Shipping Delay Compensation", Content: "For premium customers with shipping delays, offer 10% refund.", Tags: []string{"shipping", "delay", "compensation", "refund"}},
			{ID: "POL-101", Title: "Return Policy", Content: "Returns accepted within 30 days of delivery.", Tags: []string{"return", "refund", "policy"}},
			{ID: "FAQ-001", Title: "Track My Order", Content: "Use the order tracking page with your order number.", Tags: []string{"tracking", "order", "status"}},
		},
		tickets:   []Ticket{},
		nextTktID: 9921,
	}
}

// --- Tool Input/Output Types ---

type LookupCustomerInput struct {
	Email string `json:"email,omitempty"`
	ID    string `json:"id,omitempty"`
}

type LookupCustomerOutput struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

type GetOrderStatusInput struct {
	OrderID string `json:"order_id"`
}

type GetOrderStatusOutput struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	Carrier string `json:"carrier,omitempty"`
	ETA     string `json:"eta,omitempty"`
}

type SearchKBInput struct {
	Query string `json:"query"`
}

type SearchKBOutput struct {
	ArticleID string `json:"article"`
	Title     string `json:"title"`
	Action    string `json:"action"`
}

type CreateTicketInput struct {
	Type     string `json:"type"`
	Priority string `json:"priority"`
}

type CreateTicketOutput struct {
	TicketID string `json:"ticket_id"`
	Status   string `json:"status"`
}

type EscalateInput struct {
	TicketID string `json:"ticket_id"`
	Reason   string `json:"reason"`
}

type EscalateOutput struct {
	Escalated  bool   `json:"escalated"`
	AssignedTo string `json:"assigned_to"`
}

// --- Tool Constructors ---

// NewLookupCustomerTool creates a tool to find customers.
func NewLookupCustomerTool(store *MockDataStore) tool.Tool {
	return api.NewToolBuilder("lookup_customer").
		WithDescription("Find customer by email or ID").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"email": {"type": "string", "description": "Customer email address"},
				"id": {"type": "string", "description": "Customer ID"}
			}
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string"},
				"name": {"type": "string"},
				"email": {"type": "string"},
				"tier": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in LookupCustomerInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			store.mu.RLock()
			defer store.mu.RUnlock()

			// Search by ID first
			if in.ID != "" {
				if cust, ok := store.customers[in.ID]; ok {
					output := LookupCustomerOutput(cust)
					outputBytes, _ := json.Marshal(output)
					return tool.Result{Output: outputBytes}, nil
				}
			}

			// Search by email
			if in.Email != "" {
				for _, cust := range store.customers {
					if strings.EqualFold(cust.Email, in.Email) {
						output := LookupCustomerOutput(cust)
						outputBytes, _ := json.Marshal(output)
						return tool.Result{Output: outputBytes}, nil
					}
				}
			}

			return tool.Result{}, fmt.Errorf("customer not found")
		}).
		MustBuild()
}

// NewGetOrderStatusTool creates a tool to check order status.
func NewGetOrderStatusTool(store *MockDataStore) tool.Tool {
	return api.NewToolBuilder("get_order_status").
		WithDescription("Check order shipping status").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"order_id": {"type": "string", "description": "Order ID to check"}
			},
			"required": ["order_id"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"order_id": {"type": "string"},
				"status": {"type": "string"},
				"carrier": {"type": "string"},
				"eta": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in GetOrderStatusInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			store.mu.RLock()
			defer store.mu.RUnlock()

			order, ok := store.orders[in.OrderID]
			if !ok {
				return tool.Result{}, fmt.Errorf("order not found: %s", in.OrderID)
			}

			output := GetOrderStatusOutput{
				OrderID: order.ID,
				Status:  order.Status,
				Carrier: order.Carrier,
				ETA:     order.ETA,
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewSearchKBTool creates a tool to search the knowledge base.
func NewSearchKBTool(store *MockDataStore) tool.Tool {
	return api.NewToolBuilder("search_kb").
		WithDescription("Search knowledge base articles").
		WithAnnotations(api.Annotations{
			ReadOnly:   true,
			Idempotent: true,
			Cacheable:  true,
			RiskLevel:  api.RiskLow,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {"type": "string", "description": "Search query"}
			},
			"required": ["query"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"article": {"type": "string"},
				"title": {"type": "string"},
				"action": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in SearchKBInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			store.mu.RLock()
			defer store.mu.RUnlock()

			query := strings.ToLower(in.Query)
			for _, article := range store.articles {
				// Check tags and content
				for _, tag := range article.Tags {
					if strings.Contains(query, tag) {
						output := SearchKBOutput{
							ArticleID: article.ID,
							Title:     article.Title,
							Action:    extractAction(article.Content),
						}
						outputBytes, _ := json.Marshal(output)
						return tool.Result{Output: outputBytes}, nil
					}
				}
			}

			return tool.Result{}, fmt.Errorf("no matching articles found")
		}).
		MustBuild()
}

// extractAction extracts an actionable recommendation from article content.
func extractAction(content string) string {
	// Simple extraction - in reality would be more sophisticated
	if strings.Contains(content, "10% refund") {
		return "10% refund"
	}
	if strings.Contains(content, "30 days") {
		return "30 day return window"
	}
	return "See article for details"
}

// NewCreateTicketTool creates a tool to create support tickets.
func NewCreateTicketTool(store *MockDataStore) tool.Tool {
	return api.NewToolBuilder("create_ticket").
		WithDescription("Create a support ticket").
		WithAnnotations(api.Annotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true,
			RiskLevel:   api.RiskMedium,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"type": {"type": "string", "description": "Ticket type (e.g., shipping_delay, return_request)"},
				"priority": {"type": "string", "description": "Priority level (low, medium, high)"}
			},
			"required": ["type", "priority"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"ticket_id": {"type": "string"},
				"status": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in CreateTicketInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			store.mu.Lock()
			defer store.mu.Unlock()

			ticketID := fmt.Sprintf("TKT-%d", store.nextTktID)
			store.nextTktID++

			ticket := Ticket{
				ID:       ticketID,
				Type:     in.Type,
				Priority: in.Priority,
				Status:   "open",
			}
			store.tickets = append(store.tickets, ticket)

			output := CreateTicketOutput{
				TicketID: ticketID,
				Status:   "open",
			}
			outputBytes, _ := json.Marshal(output)
			return tool.Result{Output: outputBytes}, nil
		}).
		MustBuild()
}

// NewEscalateTool creates a tool to escalate to human agents.
func NewEscalateTool(store *MockDataStore) tool.Tool {
	return api.NewToolBuilder("escalate").
		WithDescription("Escalate to human agent").
		WithAnnotations(api.Annotations{
			ReadOnly:    false,
			Destructive: true, // Requires human approval
			Idempotent:  false,
			RiskLevel:   api.RiskHigh,
		}).
		WithInputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"ticket_id": {"type": "string", "description": "Ticket to escalate"},
				"reason": {"type": "string", "description": "Reason for escalation"}
			},
			"required": ["ticket_id", "reason"]
		}`))).
		WithOutputSchema(tool.NewSchema(json.RawMessage(`{
			"type": "object",
			"properties": {
				"escalated": {"type": "boolean"},
				"assigned_to": {"type": "string"}
			}
		}`))).
		WithHandler(func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var in EscalateInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			store.mu.Lock()
			defer store.mu.Unlock()

			// Find and update ticket
			for i, ticket := range store.tickets {
				if ticket.ID == in.TicketID {
					store.tickets[i].Status = "escalated"
					output := EscalateOutput{
						Escalated:  true,
						AssignedTo: "support-team-lead",
					}
					outputBytes, _ := json.Marshal(output)
					return tool.Result{Output: outputBytes}, nil
				}
			}

			return tool.Result{}, fmt.Errorf("ticket not found: %s", in.TicketID)
		}).
		MustBuild()
}
