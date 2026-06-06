# Security

agent-go provides security infrastructure for input validation, secret management, and audit logging.

## Quick Start

```go
import (
    "go.klarlabs.de/agent/infrastructure/security/validation"
    "go.klarlabs.de/agent/infrastructure/security/audit"
    "go.klarlabs.de/agent/infrastructure/security/secrets"
)

// Input validation schema
pathSchema := validation.NewSchema().
    AddRule("path", validation.Required()).
    AddRule("path", validation.MaxLength(1000)).
    AddRule("path", validation.NoPathTraversal())

// Validation middleware
schemas := map[string]*validation.Schema{
    "read_file": pathSchema,
}

// Audit logger
auditLogger := audit.NewJSONLogger(os.Stdout)

// Use with engine
engine, _ := api.New(
    api.WithMiddleware(
        validation.ValidationMiddleware(schemas),
        audit.AuditMiddleware(auditLogger),
    ),
)
```

## Input Validation

The validation package prevents malicious or malformed input from reaching tools.

### Creating Schemas

```go
import "go.klarlabs.de/agent/infrastructure/security/validation"

schema := validation.NewSchema().
    AddRule("field_name", validation.Required()).
    AddRule("field_name", validation.MaxLength(100))
```

### Available Rules

| Rule | Description | Example |
|------|-------------|---------|
| `Required()` | Field must be present and non-empty | `AddRule("name", Required())` |
| `MaxLength(n)` | String max length | `AddRule("name", MaxLength(100))` |
| `MinLength(n)` | String min length | `AddRule("password", MinLength(8))` |
| `Pattern(regex)` | Match regex pattern | `AddRule("code", Pattern("^[A-Z]{3}$"))` |
| `AllowedValues(...)` | Value must be in list | `AddRule("status", AllowedValues("active", "inactive"))` |
| `Range(min, max)` | Number within range | `AddRule("age", Range(0, 150))` |
| `Email()` | Valid email format | `AddRule("email", Email())` |
| `URL(schemes...)` | Valid URL format | `AddRule("link", URL("https"))` |

### Security Rules

Detect common attack patterns:

```go
// SQL injection detection
AddRule("query", validation.NoSQLInjection())

// Path traversal detection
AddRule("path", validation.NoPathTraversal())

// Command injection detection
AddRule("cmd", validation.NoCommandInjection())
```

### Custom Rules

```go
// Custom validation logic
isPositive := validation.Custom("positive", func(value interface{}) error {
    if num, ok := value.(float64); ok && num <= 0 {
        return errors.New("must be positive")
    }
    return nil
})

schema.AddRule("amount", isPositive)
```

### Validation Middleware

Apply validation to all tool executions:

```go
schemas := map[string]*validation.Schema{
    "read_file":   filePathSchema,
    "write_file":  fileWriteSchema,
    "db_query":    sqlQuerySchema,
}

engine, _ := api.New(
    api.WithMiddleware(
        validation.ValidationMiddleware(schemas),
    ),
)
```

### Direct Validation

Validate input directly without middleware:

```go
schema := validation.NewSchema().
    AddRule("path", validation.Required()).
    AddRule("path", validation.NoPathTraversal())

input := json.RawMessage(`{"path": "../../../etc/passwd"}`)
err := schema.Validate(input)
// err: "validation failed: path: potential path traversal detected"
```

## Audit Logging

The audit package provides comprehensive logging of security-relevant events.

### Creating Loggers

```go
import "go.klarlabs.de/agent/infrastructure/security/audit"

// JSON logger to stdout
jsonLogger := audit.NewJSONLogger(os.Stdout)

// JSON logger to file
file, _ := os.Create("audit.log")
fileLogger := audit.NewJSONLogger(file)

// In-memory logger for testing
memLogger := audit.NewMemoryLogger()
```

### Event Types

| Event Type | When Logged |
|------------|-------------|
| `EventToolExecution` | Tool starts execution |
| `EventToolComplete` | Tool completes (success or failure) |
| `EventPolicyViolation` | Policy check failed |
| `EventApprovalRequired` | Approval requested |
| `EventApprovalGranted` | Approval given |
| `EventApprovalDenied` | Approval denied |
| `EventBudgetExhausted` | Budget limit reached |
| `EventRunStart` | Agent run begins |
| `EventRunComplete` | Agent run ends |

### Logging Events

```go
// Manual event logging
auditLogger.Log(ctx, audit.Event{
    EventType:  audit.EventToolExecution,
    ToolName:   "read_file",
    RunID:      run.ID,
    Success:    true,
    Annotations: map[string]interface{}{
        "path":    "/app/config.json",
        "size":    1234,
        "user_id": "user-123",
    },
})
```

### Audit Middleware

Automatically log all tool executions:

```go
engine, _ := api.New(
    api.WithMiddleware(
        audit.AuditMiddleware(auditLogger),
    ),
)
```

### Querying Audit Log

With memory logger:

```go
memLogger := audit.NewMemoryLogger()

// After execution...
events := memLogger.Events()
for _, e := range events {
    if e.EventType == audit.EventPolicyViolation {
        fmt.Printf("Violation: %s at %s\n", e.Error, e.Timestamp)
    }
}

// Filter by type
violations := memLogger.EventsByType(audit.EventPolicyViolation)
```

## Secret Management

The secrets package provides secure access to credentials.

### Secret Manager Interface

```go
type SecretManager interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
}
```

### Environment Variable Secrets

```go
import "go.klarlabs.de/agent/infrastructure/security/secrets"

// Load from environment variables
envManager := secrets.NewEnvManager(secrets.EnvConfig{
    Prefix: "APP_",  // Only load APP_* variables
})

apiKey, _ := envManager.Get(ctx, "API_KEY")
// Reads from APP_API_KEY environment variable
```

### Memory Secrets (Testing)

```go
memManager := secrets.NewMemoryManager()
memManager.Set(ctx, "db_password", "secret123")

password, _ := memManager.Get(ctx, "db_password")
```

### Using Secrets in Tools

```go
// Inject secrets into tool context
tool := api.NewToolBuilder("api_call").
    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        // Get secret from context
        apiKey := secrets.FromContext(ctx, "api_key")
        if apiKey == "" {
            return tool.Result{}, errors.New("API key not configured")
        }

        // Use the secret...
        return makeAPICall(apiKey, input)
    }).
    MustBuild()
```

## Combining Security Layers

Complete security setup:

```go
// 1. Create validation schemas
schemas := map[string]*validation.Schema{
    "read_file": validation.NewSchema().
        AddRule("path", validation.Required()).
        AddRule("path", validation.MaxLength(1000)).
        AddRule("path", validation.NoPathTraversal()),
    "db_query": validation.NewSchema().
        AddRule("query", validation.Required()).
        AddRule("query", validation.NoSQLInjection()),
}

// 2. Create audit logger
auditLogger := audit.NewJSONLogger(os.Stdout)

// 3. Create approval handler for destructive operations
approver := api.NewCallbackApprover(func(ctx context.Context, req api.ApprovalRequest) (bool, error) {
    if req.RiskLevel == api.RiskHigh {
        // Require manual approval for high-risk operations
        return promptUser(req)
    }
    return true, nil // Auto-approve low-risk
})

// 4. Build engine with all security layers
engine, _ := api.New(
    api.WithPlanner(planner),
    api.WithRegistry(registry),
    api.WithApprover(approver),
    api.WithMiddleware(
        validation.ValidationMiddleware(schemas),
        audit.AuditMiddleware(auditLogger),
        api.LoggingMiddleware(nil),
    ),
)
```

## Best Practices

### Input Validation
1. **Validate all external input** - Never trust tool input
2. **Use security rules** - NoSQLInjection, NoPathTraversal, NoCommandInjection
3. **Whitelist over blacklist** - Use AllowedValues when possible
4. **Validate early** - Use middleware to catch issues before tools execute

### Audit Logging
1. **Log security events** - Policy violations, approvals, budget exhaustion
2. **Include context** - User IDs, request IDs, timestamps
3. **Protect audit logs** - Store securely, restrict access
4. **Retain appropriately** - Keep logs for compliance requirements

### Secret Management
1. **Never log secrets** - Exclude from audit annotations
2. **Use environment variables** - Not config files or code
3. **Rotate regularly** - Change credentials periodically
4. **Principle of least privilege** - Only grant necessary access

## See Also

- [Example: Production](../../example/07-production/) - Complete security setup
- [Example: Policies](../../example/03-policies/) - Approval workflows
- [Policies Concept](../concepts/policies.md) - Understanding policies
