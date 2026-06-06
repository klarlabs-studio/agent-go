# Domain Packs

Domain packs are pre-built collections of tools for common use cases. Each pack provides related tools with proper annotations, schemas, and state eligibility configured.

## Quick Start

```go
import (
    "database/sql"
    _ "github.com/lib/pq"

    api "go.klarlabs.de/agent/interfaces/api"
    "go.klarlabs.de/agent/pack/database"
)

// Create pack
db, _ := sql.Open("postgres", connectionString)
dbPack, _ := database.New(db, database.WithWriteAccess())

// Use with engine
engine, _ := api.New(
    api.WithPack(dbPack),
    api.WithPlanner(myPlanner),
)
```

## Available Packs

| Pack | Import Path | Tools | Purpose |
|------|-------------|-------|---------|
| Database | `pack/database` | db_query, db_execute, db_tables, db_schema | SQL database operations |
| Git | `pack/git` | git_status, git_log, git_diff, git_commit, git_add | Git repository operations |
| Kubernetes | `pack/kubernetes` | k8s_get, k8s_list, k8s_logs, k8s_apply, k8s_delete | Kubernetes cluster management |
| Cloud | `pack/cloud` | cloud_list_buckets, cloud_get_object, cloud_put_object | Cloud storage operations |
| FileOps | `pack/fileops` | read_file, write_file, list_dir, delete_file | Local filesystem operations |
| HTTP | `pack/http` | http_get, http_post, http_request | HTTP requests |

## Database Pack

Tools for SQL database operations with support for PostgreSQL, MySQL, and SQLite.

### Configuration

```go
import "go.klarlabs.de/agent/pack/database"

dbPack, err := database.New(db,
    database.WithQueryTimeout(30*time.Second), // Query timeout
    database.WithMaxRows(1000),                // Limit returned rows
    database.WithWriteAccess(),                // Enable INSERT/UPDATE/DELETE
    database.WithDDLAccess(),                  // Enable CREATE/ALTER/DROP
)
```

### Tools

| Tool | Type | Description |
|------|------|-------------|
| `db_query` | ReadOnly | Execute SELECT queries |
| `db_execute` | Destructive | Execute INSERT/UPDATE/DELETE |
| `db_tables` | ReadOnly, Cacheable | List all tables |
| `db_schema` | ReadOnly, Cacheable | Get table schema |

### Example Usage

```go
// Query tool input
{"query": "SELECT * FROM users WHERE active = $1", "args": [true], "limit": 100}

// Execute tool input
{"query": "UPDATE users SET last_login = NOW() WHERE id = $1", "args": [123]}

// Schema tool input
{"table": "users"}
```

## Git Pack

Tools for Git repository operations.

### Configuration

```go
import "go.klarlabs.de/agent/pack/git"

gitPack, err := git.New("/path/to/repo",
    git.WithAllowPush(true),   // Allow git push
    git.WithAllowForce(false), // Prevent force push (default)
)
```

### Tools

| Tool | Type | Description |
|------|------|-------------|
| `git_status` | ReadOnly | Repository status |
| `git_log` | ReadOnly, Cacheable | Commit history |
| `git_diff` | ReadOnly | Show changes |
| `git_branch` | ReadOnly | List branches |
| `git_add` | Destructive | Stage files |
| `git_commit` | Destructive | Create commit |
| `git_checkout` | Destructive | Switch branches |

### Example Usage

```go
// Log tool input
{"limit": 10, "since": "2024-01-01"}

// Commit tool input
{"message": "Add new feature", "files": ["src/feature.go"]}

// Diff tool input
{"ref": "HEAD~1", "paths": ["src/"]}
```

## Kubernetes Pack

Tools for Kubernetes cluster operations.

### Configuration

```go
import (
    "go.klarlabs.de/agent/pack/kubernetes"
    "k8s.io/client-go/kubernetes"
)

k8sClient, _ := kubernetes.NewForConfig(config)
k8sPack, err := kubernetes.New(k8sClient,
    kubernetes.WithNamespace("production"), // Restrict to namespace
    kubernetes.WithReadOnly(true),          // Disable mutations
)
```

### Tools

| Tool | Type | Description |
|------|------|-------------|
| `k8s_get` | ReadOnly | Get resource |
| `k8s_list` | ReadOnly | List resources |
| `k8s_describe` | ReadOnly | Describe resource |
| `k8s_logs` | ReadOnly | Pod logs |
| `k8s_apply` | Destructive | Apply manifest |
| `k8s_delete` | Destructive | Delete resource |
| `k8s_exec` | Destructive | Exec into pod |

### Example Usage

```go
// Get tool input
{"kind": "Pod", "name": "web-app-123", "namespace": "production"}

// Logs tool input
{"pod": "web-app-123", "container": "app", "tail": 100}

// Apply tool input
{"manifest": "apiVersion: v1\nkind: ConfigMap\n..."}
```

## Cloud Pack

Tools for cloud storage operations (S3-compatible).

### Configuration

```go
import "go.klarlabs.de/agent/pack/cloud"

// Use with AWS S3
cloudPack, err := cloud.New(s3Provider,
    cloud.WithBucket("my-bucket"),    // Default bucket
    cloud.WithMaxObjectSize(10<<20),  // 10MB limit
)
```

### Tools

| Tool | Type | Description |
|------|------|-------------|
| `cloud_list_buckets` | ReadOnly | List available buckets |
| `cloud_list_objects` | ReadOnly | List objects in bucket |
| `cloud_get_object` | ReadOnly | Download object |
| `cloud_put_object` | Destructive | Upload object |
| `cloud_delete_object` | Destructive | Delete object |

### Example Usage

```go
// List objects input
{"bucket": "my-bucket", "prefix": "data/", "max_keys": 100}

// Get object input
{"bucket": "my-bucket", "key": "data/config.json"}

// Put object input
{"bucket": "my-bucket", "key": "data/output.json", "content": "{...}"}
```

## FileOps Pack

Tools for local filesystem operations.

### Configuration

```go
import "go.klarlabs.de/agent/pack/fileops"

filesPack, err := fileops.New(
    fileops.WithBasePath("/app/data"), // Restrict to directory
    fileops.WithAllowWrite(true),      // Enable writes
    fileops.WithMaxFileSize(5<<20),    // 5MB limit
)
```

### Tools

| Tool | Type | Description |
|------|------|-------------|
| `read_file` | ReadOnly, Cacheable | Read file contents |
| `write_file` | Destructive | Write to file |
| `list_dir` | ReadOnly | List directory contents |
| `delete_file` | Destructive | Delete file |

## HTTP Pack

Tools for making HTTP requests.

### Configuration

```go
import "go.klarlabs.de/agent/pack/http"

httpPack, err := http.New(
    http.WithTimeout(30*time.Second),
    http.WithAllowedHosts("api.example.com", "*.internal.com"),
    http.WithDefaultHeaders(map[string]string{
        "User-Agent": "agent-go/1.0",
    }),
)
```

### Tools

| Tool | Type | Description |
|------|------|-------------|
| `http_get` | ReadOnly | GET request |
| `http_post` | Destructive | POST request |
| `http_request` | varies | Custom method |

## State Eligibility

Packs configure tool eligibility automatically:

```go
// Database pack default eligibility:
// - explore: db_query, db_tables, db_schema (read-only)
// - act: db_query, db_execute, db_tables, db_schema (with writes)
// - validate: db_query, db_tables, db_schema (read-only)
```

Override with custom eligibility:

```go
eligibility := api.NewToolEligibility()
eligibility.Allow(api.StateExplore, "db_query", "db_tables")
eligibility.Allow(api.StateAct, "db_execute")

engine, _ := api.New(
    api.WithPack(dbPack),
    api.WithToolEligibility(eligibility), // Override pack defaults
)
```

## Creating Custom Packs

Build your own packs using the pack builder:

```go
import (
    "go.klarlabs.de/agent/domain/pack"
    "go.klarlabs.de/agent/domain/agent"
)

myPack := pack.NewBuilder("my-pack").
    WithDescription("My custom tool pack").
    WithVersion("1.0.0").
    AddTools(
        tool1,
        tool2,
        tool3,
    ).
    AllowInState(agent.StateExplore, "tool1", "tool2").
    AllowInState(agent.StateAct, "tool1", "tool2", "tool3").
    Build()
```

## Best Practices

1. **Principle of least privilege** - Start with read-only access, add writes explicitly
2. **Set appropriate limits** - Configure max rows, file sizes, timeouts
3. **Use namespaces/paths** - Restrict operations to specific scopes
4. **Test with mocks** - Packs work with mock backends for testing
5. **Combine packs carefully** - Watch for tool name conflicts

## See Also

- [Example: Tools](../../example/02-tools/) - Tool creation examples
- [Example: Production](../../example/07-production/) - Production pack usage
- [Tools Concept](../concepts/tools.md) - Understanding tools and annotations
