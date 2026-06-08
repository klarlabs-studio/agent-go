// Package email provides email sending tools for agent-go.
//
// This pack includes tools for email operations:
//   - email_send: Send an email message
//   - email_send_template: Send an email using a template
//   - email_send_bulk: Send bulk emails
//   - email_validate: Validate an email address
//   - email_list_templates: List available email templates
//
// Supports SMTP with STARTTLS. Templates support variable substitution
// and HTML/plain text formats.
package email

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// emailRegexp is a basic email validation pattern.
var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Config holds SMTP configuration for the email pack.
type Config struct {
	// Host is the SMTP server hostname.
	Host string

	// Port is the SMTP server port (default: 587).
	Port int

	// Username is the SMTP authentication username.
	Username string

	// Password is the SMTP authentication password.
	Password string

	// From is the default sender address.
	From string

	// TLSInsecure skips TLS certificate verification (testing only).
	TLSInsecure bool

	// Timeout is the SMTP connection timeout (default: 30s).
	Timeout time.Duration
}

// Template is a reusable email template.
type Template struct {
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	HTMLBody    string `json:"html_body,omitempty"`
	TextBody    string `json:"text_body,omitempty"`
	Description string `json:"description,omitempty"`
}

type emailPack struct {
	cfg       Config
	mu        sync.RWMutex
	templates map[string]Template
}

// Pack returns the email tools pack.
func Pack(cfg Config) *pack.Pack {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	p := &emailPack{
		cfg:       cfg,
		templates: make(map[string]Template),
	}

	return pack.NewBuilder("email").
		WithDescription("Email sending and template tools").
		WithVersion("0.1.0").
		AddTools(
			p.emailSend(),
			p.emailSendTemplate(),
			p.emailSendBulk(),
			p.emailValidate(),
			p.emailListTemplates(),
		).
		AllowInState(agent.StateExplore, "email_validate", "email_list_templates").
		AllowInState(agent.StateAct, "email_send", "email_send_template", "email_send_bulk", "email_validate", "email_list_templates").
		Build()
}

// RegisterTemplate adds a template to the pack.
func RegisterTemplate(p *pack.Pack, tmpl Template) {
	// This is a no-op for packs that aren't emailPack.
	// Use WithTemplates option instead.
}

// WithTemplates returns a modified config isn't needed; use AddTemplate on the pack.
// Instead, we expose AddTemplate on the emailPack via a helper.

// AddTemplate registers a reusable email template.
func (ep *emailPack) AddTemplate(tmpl Template) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.templates[tmpl.Name] = tmpl
}

// PackWithTemplates creates a pack with pre-registered templates.
func PackWithTemplates(cfg Config, templates []Template) *pack.Pack {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	p := &emailPack{
		cfg:       cfg,
		templates: make(map[string]Template),
	}
	for _, t := range templates {
		p.templates[t.Name] = t
	}

	return pack.NewBuilder("email").
		WithDescription("Email sending and template tools").
		WithVersion("0.1.0").
		AddTools(
			p.emailSend(),
			p.emailSendTemplate(),
			p.emailSendBulk(),
			p.emailValidate(),
			p.emailListTemplates(),
		).
		AllowInState(agent.StateExplore, "email_validate", "email_list_templates").
		AllowInState(agent.StateAct, "email_send", "email_send_template", "email_send_bulk", "email_validate", "email_list_templates").
		Build()
}

func (ep *emailPack) emailSend() tool.Tool {
	return tool.NewBuilder("email_send").
		WithDescription("Send an email message").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				To      []string `json:"to"`
				CC      []string `json:"cc,omitempty"`
				BCC     []string `json:"bcc,omitempty"`
				Subject string   `json:"subject"`
				Body    string   `json:"body"`
				HTML    bool     `json:"html,omitempty"`
				From    string   `json:"from,omitempty"`
				ReplyTo string   `json:"reply_to,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.To) == 0 {
				return tool.Result{}, fmt.Errorf("at least one recipient is required")
			}
			if params.Subject == "" {
				return tool.Result{}, fmt.Errorf("subject is required")
			}
			if params.Body == "" {
				return tool.Result{}, fmt.Errorf("body is required")
			}

			from := params.From
			if from == "" {
				from = ep.cfg.From
			}
			if from == "" {
				return tool.Result{}, fmt.Errorf("from address is required (set in config or params)")
			}

			msg := ep.buildMessage(from, params.To, params.CC, params.Subject, params.Body, params.HTML, params.ReplyTo)

			allRecipients := make([]string, 0, len(params.To)+len(params.CC)+len(params.BCC))
			allRecipients = append(allRecipients, params.To...)
			allRecipients = append(allRecipients, params.CC...)
			allRecipients = append(allRecipients, params.BCC...)

			if err := ep.send(ctx, from, allRecipients, msg); err != nil {
				return tool.Result{}, fmt.Errorf("failed to send email: %w", err)
			}

			result := map[string]any{
				"success":    true,
				"recipients": len(allRecipients),
				"subject":    params.Subject,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (ep *emailPack) emailSendTemplate() tool.Tool {
	return tool.NewBuilder("email_send_template").
		WithDescription("Send an email using a predefined template").
		WithRiskLevel(tool.RiskMedium).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				To           []string          `json:"to"`
				CC           []string          `json:"cc,omitempty"`
				BCC          []string          `json:"bcc,omitempty"`
				TemplateName string            `json:"template_name"`
				Variables    map[string]string `json:"variables,omitempty"`
				From         string            `json:"from,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.To) == 0 {
				return tool.Result{}, fmt.Errorf("at least one recipient is required")
			}
			if params.TemplateName == "" {
				return tool.Result{}, fmt.Errorf("template_name is required")
			}

			ep.mu.RLock()
			tmpl, ok := ep.templates[params.TemplateName]
			ep.mu.RUnlock()
			if !ok {
				return tool.Result{}, fmt.Errorf("template not found: %s", params.TemplateName)
			}

			subject := substituteVars(tmpl.Subject, params.Variables)
			body := tmpl.TextBody
			isHTML := false
			if tmpl.HTMLBody != "" {
				body = tmpl.HTMLBody
				isHTML = true
			}
			body = substituteVars(body, params.Variables)

			from := params.From
			if from == "" {
				from = ep.cfg.From
			}
			if from == "" {
				return tool.Result{}, fmt.Errorf("from address is required")
			}

			msg := ep.buildMessage(from, params.To, params.CC, subject, body, isHTML, "")

			allRecipients := make([]string, 0, len(params.To)+len(params.CC)+len(params.BCC))
			allRecipients = append(allRecipients, params.To...)
			allRecipients = append(allRecipients, params.CC...)
			allRecipients = append(allRecipients, params.BCC...)

			if err := ep.send(ctx, from, allRecipients, msg); err != nil {
				return tool.Result{}, fmt.Errorf("failed to send template email: %w", err)
			}

			result := map[string]any{
				"success":    true,
				"template":   params.TemplateName,
				"recipients": len(allRecipients),
				"subject":    subject,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (ep *emailPack) emailSendBulk() tool.Tool {
	return tool.NewBuilder("email_send_bulk").
		WithDescription("Send bulk emails to multiple recipients").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Recipients []struct {
					To        string            `json:"to"`
					Variables map[string]string `json:"variables,omitempty"`
				} `json:"recipients"`
				Subject      string `json:"subject,omitempty"`
				Body         string `json:"body,omitempty"`
				HTML         bool   `json:"html,omitempty"`
				TemplateName string `json:"template_name,omitempty"`
				From         string `json:"from,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if len(params.Recipients) == 0 {
				return tool.Result{}, fmt.Errorf("at least one recipient is required")
			}

			from := params.From
			if from == "" {
				from = ep.cfg.From
			}
			if from == "" {
				return tool.Result{}, fmt.Errorf("from address is required")
			}

			var tmpl *Template
			if params.TemplateName != "" {
				ep.mu.RLock()
				t, ok := ep.templates[params.TemplateName]
				ep.mu.RUnlock()
				if !ok {
					return tool.Result{}, fmt.Errorf("template not found: %s", params.TemplateName)
				}
				tmpl = &t
			} else if params.Subject == "" || params.Body == "" {
				return tool.Result{}, fmt.Errorf("either template_name or subject+body is required")
			}

			var sent, failed int
			var errors []string

			for _, r := range params.Recipients {
				if err := ctx.Err(); err != nil {
					errors = append(errors, fmt.Sprintf("cancelled: %v", err))
					failed += len(params.Recipients) - sent - failed
					break
				}

				var subject, body string
				var isHTML bool

				if tmpl != nil {
					subject = substituteVars(tmpl.Subject, r.Variables)
					if tmpl.HTMLBody != "" {
						body = substituteVars(tmpl.HTMLBody, r.Variables)
						isHTML = true
					} else {
						body = substituteVars(tmpl.TextBody, r.Variables)
					}
				} else {
					subject = substituteVars(params.Subject, r.Variables)
					body = substituteVars(params.Body, r.Variables)
					isHTML = params.HTML
				}

				msg := ep.buildMessage(from, []string{r.To}, nil, subject, body, isHTML, "")

				if err := ep.send(ctx, from, []string{r.To}, msg); err != nil {
					failed++
					errors = append(errors, fmt.Sprintf("%s: %v", r.To, err))
				} else {
					sent++
				}
			}

			result := map[string]any{
				"success": failed == 0,
				"total":   len(params.Recipients),
				"sent":    sent,
				"failed":  failed,
			}
			if len(errors) > 0 {
				result["errors"] = errors
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (ep *emailPack) emailValidate() tool.Tool {
	return tool.NewBuilder("email_validate").
		WithDescription("Validate an email address format and deliverability").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				Email   string `json:"email"`
				CheckMX bool   `json:"check_mx,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, fmt.Errorf("invalid input: %w", err)
			}

			if params.Email == "" {
				return tool.Result{}, fmt.Errorf("email is required")
			}

			result := map[string]any{
				"email":        params.Email,
				"format_valid": false,
			}

			if !emailRegexp.MatchString(params.Email) {
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}
			result["format_valid"] = true

			parts := strings.SplitN(params.Email, "@", 2)
			result["local_part"] = parts[0]
			result["domain"] = parts[1]

			if params.CheckMX {
				mxRecords, err := net.LookupMX(parts[1])
				if err != nil || len(mxRecords) == 0 {
					result["mx_valid"] = false
					result["deliverable"] = false
				} else {
					hosts := make([]string, len(mxRecords))
					for i, mx := range mxRecords {
						hosts[i] = mx.Host
					}
					result["mx_valid"] = true
					result["mx_hosts"] = hosts
					result["deliverable"] = true
				}
			}

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func (ep *emailPack) emailListTemplates() tool.Tool {
	return tool.NewBuilder("email_list_templates").
		WithDescription("List available email templates").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			ep.mu.RLock()
			defer ep.mu.RUnlock()

			templates := make([]map[string]any, 0, len(ep.templates))
			for _, t := range ep.templates {
				entry := map[string]any{
					"name":    t.Name,
					"subject": t.Subject,
				}
				if t.Description != "" {
					entry["description"] = t.Description
				}
				if t.HTMLBody != "" {
					entry["format"] = "html"
				} else {
					entry["format"] = "text"
				}
				templates = append(templates, entry)
			}

			result := map[string]any{
				"templates": templates,
				"count":     len(templates),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// buildMessage constructs an RFC 2822 email message.
func (ep *emailPack) buildMessage(from string, to, cc []string, subject, body string, isHTML bool, replyTo string) []byte {
	var buf strings.Builder

	buf.WriteString("From: " + from + "\r\n")
	buf.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	if len(cc) > 0 {
		buf.WriteString("Cc: " + strings.Join(cc, ", ") + "\r\n")
	}
	if replyTo != "" {
		buf.WriteString("Reply-To: " + replyTo + "\r\n")
	}
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	if isHTML {
		buf.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	} else {
		buf.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	}

	buf.WriteString("\r\n")
	buf.WriteString(body)

	return []byte(buf.String())
}

// send delivers the message via SMTP with STARTTLS.
func (ep *emailPack) send(ctx context.Context, from string, to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", ep.cfg.Host, ep.cfg.Port)

	dialer := net.Dialer{Timeout: ep.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	client, err := smtp.NewClient(conn, ep.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client failed: %w", err)
	}
	defer client.Close()

	// STARTTLS
	tlsCfg := &tls.Config{
		ServerName:         ep.cfg.Host,
		InsecureSkipVerify: ep.cfg.TLSInsecure,
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls failed: %w", err)
		}
	}

	// Auth
	if ep.cfg.Username != "" {
		auth := smtp.PlainAuth("", ep.cfg.Username, ep.cfg.Password, ep.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from failed: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("rcpt to %s failed: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data command failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data failed: %w", err)
	}

	return client.Quit()
}

// substituteVars replaces {{key}} placeholders with values.
func substituteVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}
