// Package payments provides payment processing tools for agent-go.
//
// The pack uses an interface-based approach, allowing any payment gateway
// (Stripe, PayPal, Adyen, etc.) to be plugged in.
package payments

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// PaymentGateway provides payment processing capabilities.
type PaymentGateway interface {
	CreateInvoice(ctx context.Context, invoice Invoice) (*InvoiceResult, error)
	ProcessPayment(ctx context.Context, payment Payment) (*PaymentResult, error)
	Refund(ctx context.Context, paymentID string, opts RefundOptions) (*RefundResult, error)
	ListTransactions(ctx context.Context, opts ListOptions) (*TransactionList, error)
	GetTransaction(ctx context.Context, transactionID string) (*Transaction, error)
}

// ReconciliationEngine provides payment reconciliation.
type ReconciliationEngine interface {
	Reconcile(ctx context.Context, opts ReconcileOptions) (*ReconcileResult, error)
}

// Invoice describes an invoice to create.
type Invoice struct {
	CustomerID  string            `json:"customer_id"`
	Items       []InvoiceItem     `json:"items"`
	Currency    string            `json:"currency,omitempty"`
	DueDate     string            `json:"due_date,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// InvoiceItem represents a line item on an invoice.
type InvoiceItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitAmount  int64  `json:"unit_amount"` // in smallest currency unit (cents)
	Currency    string `json:"currency,omitempty"`
}

// InvoiceResult contains invoice creation output.
type InvoiceResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Total     int64  `json:"total"`
	Currency  string `json:"currency"`
	URL       string `json:"url,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Payment describes a payment to process.
type Payment struct {
	Amount      int64             `json:"amount"` // in smallest currency unit
	Currency    string            `json:"currency"`
	Method      string            `json:"method"` // "card", "bank_transfer", "wallet"
	CustomerID  string            `json:"customer_id,omitempty"`
	InvoiceID   string            `json:"invoice_id,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PaymentResult contains payment processing output.
type PaymentResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // "succeeded", "pending", "failed"
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	CreatedAt string `json:"created_at"`
}

// RefundOptions configures a refund.
type RefundOptions struct {
	Amount int64  `json:"amount,omitempty"` // partial refund; 0 = full
	Reason string `json:"reason,omitempty"`
}

// RefundResult contains refund output.
type RefundResult struct {
	ID        string `json:"id"`
	PaymentID string `json:"payment_id"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ListOptions configures transaction listing.
type ListOptions struct {
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	StartDate  string `json:"start_date,omitempty"`
	EndDate    string `json:"end_date,omitempty"`
	Status     string `json:"status,omitempty"`
	CustomerID string `json:"customer_id,omitempty"`
}

// TransactionList contains a list of transactions.
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
	Total        int           `json:"total"`
	HasMore      bool          `json:"has_more"`
}

// Transaction represents a financial transaction.
type Transaction struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "payment", "refund", "payout"
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	CustomerID  string `json:"customer_id,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// ReconcileOptions configures reconciliation.
type ReconcileOptions struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	AccountID string `json:"account_id,omitempty"`
}

// ReconcileResult contains reconciliation output.
type ReconcileResult struct {
	Matched       int           `json:"matched"`
	Unmatched     int           `json:"unmatched"`
	Discrepancies []Discrepancy `json:"discrepancies,omitempty"`
}

// Discrepancy represents a reconciliation discrepancy.
type Discrepancy struct {
	TransactionID string `json:"transaction_id"`
	Expected      int64  `json:"expected"`
	Actual        int64  `json:"actual"`
	Reason        string `json:"reason"`
}

// Config holds payments pack configuration.
type Config struct {
	Gateway        PaymentGateway
	Reconciliation ReconciliationEngine // optional
}

// Pack returns the payment processing tool pack.
func Pack(cfg Config) *pack.Pack {
	p := &paymentsPack{cfg: cfg}

	tools := []tool.Tool{
		p.createInvoiceTool(), p.processPaymentTool(), p.refundTool(),
		p.listTransactionsTool(), p.getTransactionTool(),
	}

	if cfg.Reconciliation != nil {
		tools = append(tools, p.reconcileTool())
	}

	return pack.NewBuilder("payments").
		WithDescription("Payment processing tools: create_invoice, process_payment, refund, reconcile, list_transactions").
		WithVersion("1.0.0").
		AddTools(tools...).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

type paymentsPack struct{ cfg Config }

func (p *paymentsPack) createInvoiceTool() tool.Tool {
	return tool.NewBuilder("payments_create_invoice").
		WithDescription("Create a payment invoice").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Invoice
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.CustomerID == "" {
				return tool.Result{}, fmt.Errorf("customer_id is required")
			}
			if len(in.Items) == 0 {
				return tool.Result{}, fmt.Errorf("at least one item is required")
			}
			if in.Currency == "" {
				in.Currency = "usd"
			}
			result, err := p.cfg.Gateway.CreateInvoice(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("create invoice failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *paymentsPack) processPaymentTool() tool.Tool {
	return tool.NewBuilder("payments_process_payment").
		WithDescription("Process a payment transaction").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in Payment
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Amount <= 0 {
				return tool.Result{}, fmt.Errorf("amount must be positive")
			}
			if in.Currency == "" {
				return tool.Result{}, fmt.Errorf("currency is required")
			}
			if in.Method == "" {
				return tool.Result{}, fmt.Errorf("payment method is required")
			}
			result, err := p.cfg.Gateway.ProcessPayment(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("process payment failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *paymentsPack) refundTool() tool.Tool {
	return tool.NewBuilder("payments_refund").
		WithDescription("Refund a payment transaction").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				PaymentID string `json:"payment_id"`
				Amount    int64  `json:"amount,omitempty"`
				Reason    string `json:"reason,omitempty"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.PaymentID == "" {
				return tool.Result{}, fmt.Errorf("payment_id is required")
			}
			result, err := p.cfg.Gateway.Refund(ctx, in.PaymentID, RefundOptions{
				Amount: in.Amount, Reason: in.Reason,
			})
			if err != nil {
				return tool.Result{}, fmt.Errorf("refund failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *paymentsPack) listTransactionsTool() tool.Tool {
	return tool.NewBuilder("payments_list_transactions").
		WithDescription("List payment transactions with filters").
		ReadOnly().Idempotent().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in ListOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.Limit == 0 {
				in.Limit = 25
			}
			result, err := p.cfg.Gateway.ListTransactions(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("list transactions failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *paymentsPack) getTransactionTool() tool.Tool {
	return tool.NewBuilder("payments_get_transaction").
		WithDescription("Get details of a specific transaction").
		ReadOnly().Idempotent().Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in struct {
				TransactionID string `json:"transaction_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.TransactionID == "" {
				return tool.Result{}, fmt.Errorf("transaction_id is required")
			}
			result, err := p.cfg.Gateway.GetTransaction(ctx, in.TransactionID)
			if err != nil {
				return tool.Result{}, fmt.Errorf("get transaction failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}

func (p *paymentsPack) reconcileTool() tool.Tool {
	return tool.NewBuilder("payments_reconcile").
		WithDescription("Reconcile transactions against records").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in ReconcileOptions
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}
			if in.StartDate == "" || in.EndDate == "" {
				return tool.Result{}, fmt.Errorf("start_date and end_date are required")
			}
			result, err := p.cfg.Reconciliation.Reconcile(ctx, in)
			if err != nil {
				return tool.Result{}, fmt.Errorf("reconcile failed: %w", err)
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).MustBuild()
}
