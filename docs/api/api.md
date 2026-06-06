# Package `api`

**Import path:** `go.klarlabs.de/agent/interfaces/api`

## Overview

package api // import "go.klarlabs.de/agent/interfaces/api"

Package api provides the public API for the agent-go runtime.

agent-go is a state-driven agent framework for Go that enables developers to
build trustworthy, adaptable AI-powered systems by designing the structure and
constraints of agent behavior rather than scripting intelligence with prompts.


Create a minimal agent with a tool and scripted planner:







The agent operates within a canonical state graph:

## Full API Reference

```
package api // import "go.klarlabs.de/agent/interfaces/api"

Package api provides the public API for the agent-go runtime.

agent-go is a state-driven agent framework for Go that enables developers to
build trustworthy, adaptable AI-powered systems by designing the structure and
constraints of agent behavior rather than scripting intelligence with prompts.

# Quick Start

Create a minimal agent with a tool and scripted planner:

    // 1. Create a tool
    echoTool := api.NewToolBuilder("echo").
        WithDescription("Echoes input").
        WithAnnotations(api.Annotations{ReadOnly: true}).
        WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
            return tool.Result{Output: input}, nil
        }).
        MustBuild()

    // 2. Create a planner
    planner := api.NewScriptedPlanner(
        api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "start")},
        api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewCallToolDecision("echo", input, "echo")},
        api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "done")},
        api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("completed", result)},
    )

    // 3. Configure tool eligibility
    // Option A: Declarative style (recommended for static configuration)
    eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
        api.StateExplore: {"echo", "read_file"},
        api.StateAct:     {"write_file"},
    })

    // Option B: Imperative style (useful for dynamic configuration)
    eligibility := api.NewToolEligibility()
    eligibility.Allow(api.StateExplore, "echo")
    eligibility.Allow(api.StateExplore, "read_file")
    eligibility.Allow(api.StateAct, "write_file")

    // 4. Create and run the engine
    engine, _ := api.New(
        api.WithTool(echoTool),
        api.WithPlanner(planner),
        api.WithToolEligibility(eligibility),
    )
    run, _ := engine.Run(ctx, "Echo a message")

# States

The agent operates within a canonical state graph:

  - StateIntake: Normalize and understand the goal
  - StateExplore: Gather information (read-only tools only)
  - StateDecide: Choose next action
  - StateAct: Execute side-effects (destructive tools allowed)
  - StateValidate: Verify outcomes
  - StateDone: Terminal success state
  - StateFailed: Terminal failure state

# Tools

Tools are capabilities the agent can invoke. Each tool has annotations that
describe its behavior:

  - ReadOnly: Tool does not modify external state
  - Destructive: Tool performs irreversible operations
  - Idempotent: Repeated calls produce same result
  - Cacheable: Results can be cached
  - RiskLevel: Potential impact (None, Low, Medium, High, Critical)

# Planners

Planners make decisions about what the agent should do next:

  - ScriptedPlanner: Predefined sequence for testing (supports looping,
    conditional steps, error injection)
  - MockPlanner: Returns specific decisions for testing
  - RuleBasedPlanner: Evaluates Go-native rules in priority order, first match
    wins
  - HybridPlanner: Combines rule-based with any fallback planner
  - LLMPlanner: Uses an LLM provider for intelligent planning

# Policies

Policies enforce constraints on agent behavior:

  - ToolEligibility: Which tools can run in which states
  - StateTransitions: Which state transitions are allowed
  - Approvers: Human approval for destructive operations
  - Budgets: Limits on tool calls, tokens, etc.

Package api provides the public API for the agent-go library. This file provides
configuration-related exports.

Package api provides the public API for the agent runtime.

Package api provides the public API for the agent-go library. This file provides
metrics-related exports.

Package api provides the public API for the agent runtime.

Package api provides the public API for the agent-go library. This file provides
notification-related exports.

Package api provides the public API for the agent runtime.

Package api provides the public API for the agent runtime.

Package api provides the public API for the agent runtime.

CONSTANTS

const (
	StateIntake   = agent.StateIntake
	StateExplore  = agent.StateExplore
	StateDecide   = agent.StateDecide
	StateAct      = agent.StateAct
	StateValidate = agent.StateValidate
	StateDone     = agent.StateDone
	StateFailed   = agent.StateFailed
)
    Re-export state constants.

const (
	RiskNone     = tool.RiskNone
	RiskLow      = tool.RiskLow
	RiskMedium   = tool.RiskMedium
	RiskHigh     = tool.RiskHigh
	RiskCritical = tool.RiskCritical
)
    Re-export risk levels.

const (
	StatusPending   = agent.RunStatusPending
	StatusRunning   = agent.RunStatusRunning
	StatusPaused    = agent.RunStatusPaused
	StatusCompleted = agent.RunStatusCompleted
	StatusFailed    = agent.RunStatusFailed
)
const (
	// ConfigFormatYAML is the YAML format.
	ConfigFormatYAML = infraconfig.FormatYAML
	// ConfigFormatJSON is the JSON format.
	ConfigFormatJSON = infraconfig.FormatJSON
)
    Configuration format constants.

const (
	FormatJSON     = inspector.FormatJSON
	FormatDOT      = inspector.FormatDOT
	FormatMermaid  = inspector.FormatMermaid
	FormatTimeline = inspector.FormatTimeline
)
    Re-export export format constants.

const (
	EventRunStarted      = domainnotif.EventRunStarted
	EventRunCompleted    = domainnotif.EventRunCompleted
	EventRunFailed       = domainnotif.EventRunFailed
	EventRunPaused       = domainnotif.EventRunPaused
	EventStateChanged    = domainnotif.EventStateChanged
	EventToolStarted     = domainnotif.EventToolStarted
	EventToolCompleted   = domainnotif.EventToolCompleted
	EventToolFailed      = domainnotif.EventToolFailed
	EventApprovalNeeded  = domainnotif.EventApprovalNeeded
	EventBudgetWarning   = domainnotif.EventBudgetWarning
	EventBudgetExhausted = domainnotif.EventBudgetExhausted
)
    Event type constants.

const (
	PatternTypeToolSequence     = pattern.PatternTypeToolSequence
	PatternTypeStateLoop        = pattern.PatternTypeStateLoop
	PatternTypeToolAffinity     = pattern.PatternTypeToolAffinity
	PatternTypeRecurringFailure = pattern.PatternTypeRecurringFailure
	PatternTypeToolFailure      = pattern.PatternTypeToolFailure
	PatternTypeBudgetExhaustion = pattern.PatternTypeBudgetExhaustion
	PatternTypeSlowTool         = pattern.PatternTypeSlowTool
	PatternTypeLongRuns         = pattern.PatternTypeLongRuns
)
    Re-export pattern type constants.

const (
	ProposalStatusDraft         = proposal.ProposalStatusDraft
	ProposalStatusPendingReview = proposal.ProposalStatusPendingReview
	ProposalStatusApproved      = proposal.ProposalStatusApproved
	ProposalStatusRejected      = proposal.ProposalStatusRejected
	ProposalStatusApplied       = proposal.ProposalStatusApplied
	ProposalStatusRolledBack    = proposal.ProposalStatusRolledBack
)
    Re-export proposal status constants.

const (
	ChangeTypeEligibility = proposal.ChangeTypeEligibility
	ChangeTypeTransition  = proposal.ChangeTypeTransition
	ChangeTypeBudget      = proposal.ChangeTypeBudget
	ChangeTypeApproval    = proposal.ChangeTypeApproval
)
    Re-export change type constants.

const (
	SuggestionTypeAddEligibility    = suggestion.SuggestionTypeAddEligibility
	SuggestionTypeRemoveEligibility = suggestion.SuggestionTypeRemoveEligibility
	SuggestionTypeAddTransition     = suggestion.SuggestionTypeAddTransition
	SuggestionTypeRemoveTransition  = suggestion.SuggestionTypeRemoveTransition
	SuggestionTypeIncreaseBudget    = suggestion.SuggestionTypeIncreaseBudget
	SuggestionTypeDecreaseBudget    = suggestion.SuggestionTypeDecreaseBudget
	SuggestionTypeRequireApproval   = suggestion.SuggestionTypeRequireApproval
)
    Re-export suggestion type constants.

const (
	SuggestionStatusPending    = suggestion.SuggestionStatusPending
	SuggestionStatusAccepted   = suggestion.SuggestionStatusAccepted
	SuggestionStatusRejected   = suggestion.SuggestionStatusRejected
	SuggestionStatusSuperseded = suggestion.SuggestionStatusSuperseded
)
    Re-export suggestion status constants.

const (
	ImpactLevelLow    = suggestion.ImpactLevelLow
	ImpactLevelMedium = suggestion.ImpactLevelMedium
	ImpactLevelHigh   = suggestion.ImpactLevelHigh
)
    Re-export impact level constants.


VARIABLES

var (
	// ErrAwaitingHumanInput is returned when the agent pauses for human input.
	// Check run.PendingQuestion for the question and options.
	ErrAwaitingHumanInput = agent.ErrAwaitingHumanInput

	// ErrNoPendingQuestion is returned when attempting to resume a run
	// that has no pending question.
	ErrNoPendingQuestion = agent.ErrNoPendingQuestion

	// ErrInvalidHumanInput is returned when the provided input doesn't
	// match the allowed options for the pending question.
	ErrInvalidHumanInput = agent.ErrInvalidHumanInput
)
    Re-export human input related errors.

var (
	// ErrKnowledgeNotFound indicates the requested vector was not found.
	ErrKnowledgeNotFound = knowledge.ErrNotFound

	// ErrKnowledgeInvalidID indicates the vector ID is empty or invalid.
	ErrKnowledgeInvalidID = knowledge.ErrInvalidID

	// ErrKnowledgeInvalidEmbedding indicates the embedding is empty or invalid.
	ErrKnowledgeInvalidEmbedding = knowledge.ErrInvalidEmbedding

	// ErrKnowledgeDimensionMismatch indicates the embedding dimension doesn't match.
	ErrKnowledgeDimensionMismatch = knowledge.ErrDimensionMismatch
)
    Re-export knowledge errors.

var (
	// ErrConfigNotFound indicates the configuration file was not found.
	ErrConfigNotFound = domainconfig.ErrConfigNotFound
	// ErrInvalidFormat indicates the configuration format is invalid.
	ErrInvalidFormat = domainconfig.ErrInvalidFormat
	// ErrUnsupportedFormat indicates the file format is not supported.
	ErrUnsupportedFormat = domainconfig.ErrUnsupportedFormat
	// ErrValidationFailed indicates configuration validation failed.
	ErrValidationFailed = domainconfig.ErrValidationFailed
	// ErrEnvExpansionFailed indicates environment variable expansion failed.
	ErrEnvExpansionFailed = domainconfig.ErrEnvExpansionFailed
	// ErrMissingEnvVar indicates a required environment variable is not set.
	ErrMissingEnvVar = domainconfig.ErrMissingEnvVar
	// ErrBuildFailed indicates engine building from config failed.
	ErrBuildFailed = domainconfig.ErrBuildFailed
	// ErrSchemaGenerationFailed indicates JSON schema generation failed.
	ErrSchemaGenerationFailed = domainconfig.ErrSchemaGenerationFailed
)
    Configuration errors.

var (
	// ErrEndpointUnavailable indicates the webhook endpoint is not reachable.
	ErrEndpointUnavailable = domainnotif.ErrEndpointUnavailable
	// ErrEndpointRejected indicates the endpoint rejected the notification.
	ErrEndpointRejected = domainnotif.ErrEndpointRejected
	// ErrNotifierClosed indicates the notifier has been closed.
	ErrNotifierClosed = domainnotif.ErrNotifierClosed
	// ErrInvalidEndpoint indicates the endpoint configuration is invalid.
	ErrInvalidEndpoint = domainnotif.ErrInvalidEndpoint
	// ErrBatchTooLarge indicates the batch exceeds the maximum size.
	ErrBatchTooLarge = domainnotif.ErrBatchTooLarge
	// ErrEventFilteredOut indicates the event was filtered out.
	ErrEventFilteredOut = domainnotif.ErrEventFilteredOut
	// ErrSigningFailed indicates payload signing failed.
	ErrSigningFailed = domainnotif.ErrSigningFailed
)
    Notification errors.


FUNCTIONS

func AutoApprover() policy.Approver
    AutoApprover returns an approver that automatically approves all requests.
    This is a convenience function for development and testing.

func ConfigSchemaJSON() (string, error)
    ConfigSchemaJSON returns the configuration JSON Schema as a JSON string.

func CreateApprovalChange(toolName string, required bool, description string) (*proposal.PolicyChange, error)
    CreateApprovalChange creates an approval requirement policy change.

func CreateBudgetChange(budgetName string, oldValue, newValue int, description string) (*proposal.PolicyChange, error)
    CreateBudgetChange creates a budget policy change.

func CreateEligibilityChange(state State, toolName string, allowed bool, description string) (*proposal.PolicyChange, error)
    CreateEligibilityChange creates an eligibility policy change.

func CreateTransitionChange(from, to State, allowed bool, description string) (*proposal.PolicyChange, error)
    CreateTransitionChange creates a transition policy change.

func DefaultTransitions() *policy.StateTransitions
    DefaultTransitions returns the canonical state transition configuration.

func DenyApprover(reason string) policy.Approver
    DenyApprover returns an approver that automatically denies all requests.
    This is a convenience function for testing rejection scenarios.

func ExpandEnv(input string) string
    ExpandEnv expands environment variables in a string. Supported patterns:
    ${VAR}, ${VAR:-default}, ${VAR:?error}

func ExpandEnvStrict(input string) (string, error)
    ExpandEnvStrict expands environment variables and returns an error for
    missing vars.

func NewAutoApprover(name string) *policy.AutoApprover
    NewAutoApprover creates an approver that automatically approves all
    requests.

func NewBudgetGenerator() suggestion.Generator
    NewBudgetGenerator creates a generator for budget suggestions.

func NewCompositeDetector(detectors ...pattern.Detector) pattern.Detector
    NewCompositeDetector creates a detector that combines multiple detectors.

func NewCompositeSuggestionGenerator(generators ...suggestion.Generator) suggestion.Generator
    NewCompositeSuggestionGenerator creates a generator that combines multiple
    generators.

func NewConfigValidator() *domainconfig.Validator
    NewConfigValidator creates a new configuration validator.

func NewDefaultToolEligibility() *policy.ToolEligibility
    NewDefaultToolEligibility creates a tool eligibility with sensible defaults.
    All registered tools are allowed (via wildcard "*") in explore, decide, act,
    and validate states. The intake state has no tools allowed. Terminal states
    (done, failed) have no tools allowed.

    This is the easiest way to get started. For fine-grained per-state control,
    use NewToolEligibility() or NewToolEligibilityWith() instead.

func NewDenyApprover(reason string) *policy.DenyApprover
    NewDenyApprover creates an approver that automatically denies all requests.

func NewEligibilityGenerator() suggestion.Generator
    NewEligibilityGenerator creates a generator for eligibility suggestions.

func NewFailureDetector(eventStore EventStore, runStore RunStore) pattern.Detector
    NewFailureDetector creates a detector for failure patterns.

func NewHybridPlanner(rules *planner.RuleBasedPlanner, fallback planner.Planner) *planner.HybridPlanner
    NewHybridPlanner creates a hybrid planner that tries rules first, then falls
    back to the given planner when no rule matches.

func NewInspector(
	runExporter inspector.RunExporter,
	stateMachineExporter inspector.StateMachineExporter,
	metricsExporter inspector.MetricsExporter,
) inspector.Inspector
    NewInspector creates a new inspector with the provided exporters.

func NewKnowledgeStore(dimension int) *memory.KnowledgeStore
    NewKnowledgeStore creates a new in-memory knowledge store for vector
    embeddings. If dimension is 0, it will be auto-detected from the first
    vector stored.

func NewMemoryCache(maxEntries int) *memory.Cache
    NewMemoryCache creates a new in-memory cache with the specified maximum
    entries.

func NewMetricsExporter(runStore RunStore, eventStore EventStore) inspector.MetricsExporter
    NewMetricsExporter creates a new metrics exporter.

func NewMockPlanner(decisions ...Decision) *planner.MockPlanner
    NewMockPlanner creates a mock planner with predefined decisions.

func NewPatternStore() pattern.Store
    NewPatternStore creates a new in-memory pattern store.

func NewPerformanceDetector(eventStore EventStore, runStore RunStore, opts ...PerformanceDetectorOption) pattern.Detector
    NewPerformanceDetector creates a detector for performance patterns.

func NewPolicyApplier() *infraProposal.PolicyApplier
    NewPolicyApplier creates a new policy applier.

func NewPolicyVersionStore() policy.VersionStore
    NewPolicyVersionStore creates a new in-memory policy version store.

func NewProposalStore() proposal.Store
    NewProposalStore creates a new in-memory proposal store.

func NewRule(name string) *planner.RuleBuilder
    NewRule creates a new rule builder with the given name.

func NewRuleBasedPlanner(fallback Decision, rules ...planner.Rule) *planner.RuleBasedPlanner
    NewRuleBasedPlanner creates a rule-based planner that evaluates rules in
    priority order. The fallback decision is returned when no rule matches.

func NewRunExporter(runStore RunStore, eventStore EventStore) inspector.RunExporter
    NewRunExporter creates a new run exporter.

func NewScriptedPlanner(steps ...planner.ScriptStep) *planner.ScriptedPlanner
    NewScriptedPlanner creates a scripted planner for deterministic testing.

func NewSequenceDetector(eventStore EventStore, runStore RunStore) pattern.Detector
    NewSequenceDetector creates a detector for tool sequence patterns.

func NewStateMachineExporter(
	eligibility *policy.ToolEligibility,
	transitions *policy.StateTransitions,
) inspector.StateMachineExporter
    NewStateMachineExporter creates a new state machine exporter.

func NewStateTransitions() *policy.StateTransitions
    NewStateTransitions creates a new empty state transitions configuration.
    Use the Allow method to add rules incrementally.

    Use this imperative style when:
      - Building transitions dynamically based on runtime conditions
      - Adding transitions conditionally or in a loop
      - Preferring method chaining for readability

    For static configuration, prefer NewStateTransitionsWith or
    DefaultTransitions instead.

    Example:

        transitions := api.NewStateTransitions()
        transitions.Allow(api.StateIntake, api.StateExplore)
        transitions.Allow(api.StateExplore, api.StateDecide)

func NewStateTransitionsWith(rules TransitionRules) *policy.StateTransitions
    NewStateTransitionsWith creates a state transition configuration from a
    rules map. This is the preferred constructor for declarative, readable
    configuration.

    Example:

        transitions := api.NewStateTransitionsWith(api.TransitionRules{
            api.StateIntake:   {api.StateExplore, api.StateFailed},
            api.StateExplore:  {api.StateDecide, api.StateFailed},
            api.StateDecide:   {api.StateAct, api.StateDone, api.StateFailed},
            api.StateAct:      {api.StateValidate, api.StateFailed},
            api.StateValidate: {api.StateDone, api.StateExplore, api.StateFailed},
        })

func NewSuggestionStore() suggestion.Store
    NewSuggestionStore creates a new in-memory suggestion store.

func NewToolBuilder(name string) *domaintool.Builder
    NewToolBuilder creates a new tool builder.

func NewToolEligibility() *policy.ToolEligibility
    NewToolEligibility creates a new empty tool eligibility configuration.
    Use the Allow or AllowMultiple methods to add rules incrementally.

    Use this imperative style when:
      - Building eligibility dynamically based on runtime conditions
      - Adding tools conditionally or in a loop
      - Preferring method chaining for readability

    For static configuration, prefer NewToolEligibilityWith instead.

    Example:

        eligibility := api.NewToolEligibility()
        eligibility.Allow(api.StateExplore, "read_file")
        eligibility.Allow(api.StateExplore, "list_dir")
        eligibility.Allow(api.StateAct, "write_file")

func NewToolEligibilityWith(rules EligibilityRules) *policy.ToolEligibility
    NewToolEligibilityWith creates a tool eligibility configuration from a
    rules map. This is the preferred constructor for declarative, readable
    configuration.

    Example:

        eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
            api.StateExplore: {"lookup_customer", "get_order_status", "search_kb"},
            api.StateAct:     {"create_ticket", "escalate"},
            api.StateValidate: {"search_kb"},
        })

func NewToolRegistry() *memory.ToolRegistry
    NewToolRegistry creates a new in-memory tool registry.

func NewWorkflowService(
	proposalStore proposal.Store,
	versionStore policy.VersionStore,
	applier *infraProposal.PolicyApplier,
) *infraProposal.WorkflowService
    NewWorkflowService creates a new workflow service.


TYPES

type AgentConfig = domainconfig.AgentConfig
    AgentConfig represents the complete agent configuration.

func DefaultAgentConfig() *AgentConfig
    DefaultAgentConfig returns a minimal default configuration.

type AgentSettings = domainconfig.AgentSettings
    AgentSettings contains core agent behavior settings.

type Annotations = tool.Annotations
    Annotations describe tool behavior.

type ApprovalConfig = domainconfig.ApprovalConfig
    ApprovalConfig configures approval behavior.

type ApprovalNeededPayload = domainnotif.ApprovalNeededPayload
    Re-export domain notification types.

type ApprovalRequest = policy.ApprovalRequest
    ApprovalRequest is re-exported for callback approvers.

type ApprovalResponse = policy.ApprovalResponse
    ApprovalResponse is re-exported for callback approvers.

type BatcherConfig = infranotif.BatcherConfig
    BatcherConfig configures the event batcher.

func DefaultBatcherConfig() BatcherConfig
    DefaultBatcherConfig returns sensible default configuration for the event
    batcher.

type BatchingConfigSpec = domainconfig.BatchingConfig
    BatchingConfigSpec configures event batching.

type BudgetExhaustedPayload = domainnotif.BudgetExhaustedPayload
    Re-export domain notification types.

type BudgetView = middleware.BudgetView
    BudgetView provides read-only access to budget state.

type BudgetWarningPayload = domainnotif.BudgetWarningPayload
    Re-export domain notification types.

type BulkheadConfigSpec = domainconfig.BulkheadConfig
    BulkheadConfigSpec configures bulkhead behavior.

type Cache = cache.Cache
    Cache is the interface for tool result caching.

type CacheMetricsRecorder = inframw.CacheMetricsRecorder
    CacheMetricsRecorder records cache-related metrics.

func NewCacheMetricsRecorder(provider Metrics) CacheMetricsRecorder
    NewCacheMetricsRecorder creates a recorder for cache metrics. Use this with
    caching middleware to track cache hits and misses.

type CacheOption = inframw.CacheOption
    CacheOption configures the caching middleware.

func WithCacheTTL(ttl time.Duration) CacheOption
    WithCacheTTL sets the cache TTL for cached entries.

type CallbackApprover struct {
	// Has unexported fields.
}
    CallbackApprover implements the Approver interface using a callback
    function.

func NewCallbackApprover(fn func(ctx context.Context, req ApprovalRequest) (bool, error)) *CallbackApprover
    NewCallbackApprover creates an approver that uses a callback function for
    decisions.

func (c *CallbackApprover) Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
    Approve processes the approval request using the callback function.

type ChangeType = proposal.ChangeType
    ChangeType categorizes the type of policy change.

type CircuitBreakerConfigSpec = domainconfig.CircuitBreakerConfig
    CircuitBreakerConfigSpec configures circuit breaker behavior.

type CircuitBreakerMetricsRecorder = inframw.CircuitBreakerMetricsRecorder
    CircuitBreakerMetricsRecorder records circuit breaker metrics.

func NewCircuitBreakerMetricsRecorder(provider Metrics) CircuitBreakerMetricsRecorder
    NewCircuitBreakerMetricsRecorder creates a recorder for circuit breaker
    metrics.

type ConfigBuildResult = infraconfig.BuildResult
    ConfigBuildResult contains the built components from configuration.

type ConfigBuilder = infraconfig.Builder
    ConfigBuilder builds engine options from configuration.

func NewConfigBuilder(config *AgentConfig) *ConfigBuilder
    NewConfigBuilder creates a new configuration builder.

type ConfigDuration = domainconfig.Duration
    ConfigDuration is a time.Duration that supports JSON/YAML string
    representation.

type ConfigLoader = infraconfig.Loader
    ConfigLoader loads agent configuration from files.

func NewConfigLoader() *ConfigLoader
    NewConfigLoader creates a new configuration loader with default settings.

func NewConfigLoaderWithOptions(opts ...ConfigLoaderOption) *ConfigLoader
    NewConfigLoaderWithOptions creates a loader with the specified options.

type ConfigLoaderOption = infraconfig.LoaderOption
    ConfigLoaderOption configures the loader.

func ConfigWithEnvExpansion(enabled bool) ConfigLoaderOption
    ConfigWithEnvExpansion enables or disables environment variable expansion.

func ConfigWithStrictEnv(enabled bool) ConfigLoaderOption
    ConfigWithStrictEnv enables strict environment variable checking.

func ConfigWithValidation(enabled bool) ConfigLoaderOption
    ConfigWithValidation enables or disables configuration validation.

type Decision = agent.Decision
    Decision represents the planner's output.

func NewCallToolDecision(toolName string, input []byte, reason string) Decision
    NewCallToolDecision creates a decision to execute a tool.

func NewFailDecision(reason string, err error) Decision
    NewFailDecision creates a decision to terminate with failure.

func NewFinishDecision(summary string, result []byte) Decision
    NewFinishDecision creates a decision to complete successfully.

func NewTransitionDecision(toState State, reason string) Decision
    NewTransitionDecision creates a decision to transition states.

type DetectionOptions = pattern.DetectionOptions
    DetectionOptions configures pattern detection.

type EligibilityRules = policy.EligibilityRules
    EligibilityRules maps states to the tools allowed in each state. This is the
    preferred way to configure tool eligibility declaratively.

    Example:

        eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
            api.StateExplore: {"read_file", "list_dir"},
            api.StateAct:     {"write_file", "delete_file"},
            api.StateValidate: {"read_file"},
        })

type Endpoint = domainnotif.Endpoint
    Endpoint represents a webhook endpoint configuration.

type EndpointConfigSpec = domainconfig.EndpointConfig
    EndpointConfigSpec configures a webhook endpoint.

type Engine struct {
	// Has unexported fields.
}
    Engine is the main runtime for agent execution.

func New(opts ...Option) (*Engine, error)
    New creates a new Engine with the provided options.

func (e *Engine) Knowledge() knowledge.Store
    Knowledge returns the knowledge store, if configured. Returns nil if no
    knowledge store was provided via WithKnowledgeStore.

func (e *Engine) ResumeWithInput(ctx context.Context, run *Run, input string) (*Run, error)
    ResumeWithInput continues a paused run with human-provided input. This is
    used after a run returns ErrAwaitingHumanInput to provide the response to
    the pending question.

    Example:

        run, err := engine.Run(ctx, "Process data and ask for confirmation")
        if errors.Is(err, api.ErrAwaitingHumanInput) {
            fmt.Printf("Agent asks: %s\n", run.PendingQuestion.Question)
            input := getUserInput()
            run, err = engine.ResumeWithInput(ctx, run, input)
        }

func (e *Engine) Run(ctx context.Context, goal string) (*Run, error)
    Run executes the agent with the given goal.

func (e *Engine) RunWithVars(ctx context.Context, goal string, vars map[string]any) (*Run, error)
    RunWithVars executes the agent with the given goal and initial variables.

func (e *Engine) Stream(ctx context.Context, goal string) (string, <-chan event.Event, error)
    Stream executes the agent in the background and returns a channel of events.
    Requires an EventStore to be configured via WithEventStore.

    Example:

        runID, events, _ := engine.Stream(ctx, "Process files")
        for evt := range events {
            fmt.Printf("[%s] %s\n", evt.Type, evt.Payload)
        }

type Event = domainnotif.Event
    Event represents a notification event to be sent to webhooks.

func NewEvent(id string, eventType EventType, runID string, payload any) (*Event, error)
    NewEvent creates a new notification event.

type EventFilter = domainnotif.EventFilter
    EventFilter is a function that determines whether an event should be sent.

func CombineFilters(filters ...EventFilter) EventFilter
    CombineFilters creates a filter that requires all provided filters to pass.

func FilterByRunID(runID string) EventFilter
    FilterByRunID creates a filter that only allows events for the specified run
    ID.

func FilterByType(types ...EventType) EventFilter
    FilterByType creates a filter that only allows events of the specified
    types.

type EventStore = event.Store
    EventStore stores events for pattern detection.

type EventType = domainnotif.EventType
    EventType represents the type of notification event.

type Evidence = agent.Evidence
    Evidence represents an observation during a run.

type ExecutionContext = middleware.ExecutionContext
    ExecutionContext contains all information needed for middleware decisions.

type ExportFormat = inspector.ExportFormat
    ExportFormat specifies the output format for exports.

type Handler = middleware.Handler
    Handler executes a tool and returns its result.

type ImpactLevel = suggestion.ImpactLevel
    ImpactLevel indicates the potential impact of a suggestion.

type InlineToolConfig = domainconfig.InlineToolConfig
    InlineToolConfig defines an inline tool.

type Inspector = inspector.Inspector
    Inspector provides inspection and export capabilities.

type JSONSchema = infraconfig.JSONSchema
    JSONSchema represents a JSON Schema document.

func GenerateConfigSchema() *JSONSchema
    GenerateConfigSchema generates a JSON Schema for the AgentConfig.

type KnowledgeStats = knowledge.Stats
    KnowledgeStats provides statistics about the knowledge store.

type KnowledgeStore = knowledge.Store
    KnowledgeStore is the interface for vector knowledge storage.

type LegacyMiddlewareCache = inframw.LegacyCache //nolint:staticcheck // intentional backward compatibility
    LegacyMiddlewareCache provides in-memory caching for tool results.

    Deprecated: Use Cache interface with NewMemoryCache instead.

func NewLegacyMiddlewareCache(maxEntries int) *LegacyMiddlewareCache
    NewLegacyMiddlewareCache creates a new cache with the specified maximum
    entries.

    Deprecated: Use NewMemoryCache instead.

type ListFilter = knowledge.ListFilter
    ListFilter provides filtering options for knowledge list operations.

type LoggingMiddlewareConfig struct {
	// LogInput logs the tool input (may contain sensitive data).
	LogInput bool
	// LogOutput logs the tool output (may be large).
	LogOutput bool
}
    LoggingMiddlewareConfig configures the logging middleware.

type Metrics = metrics.Metrics
    Metrics is the interface for recording metrics.

type MetricsExport = inspector.MetricsExport
    MetricsExport contains exported metrics data.

type MetricsFilter = inspector.MetricsFilter
    MetricsFilter filters metrics queries.

type MetricsMiddlewareConfig = inframw.MetricsConfig
    MetricsMiddlewareConfig configures the metrics middleware.

type Middleware = middleware.Middleware
    Middleware wraps a Handler with additional behavior.

func ApprovalMiddleware(approver policy.Approver) Middleware
    ApprovalMiddleware returns middleware that enforces human approval for risky
    tools. Tools marked with ShouldRequireApproval() must be approved before
    execution.

func BudgetFromContextMiddleware(budgetName string, amount int) Middleware
    BudgetFromContextMiddleware returns middleware that uses the budget from
    ExecutionContext. This is useful when budget needs to be determined at
    runtime.

func BudgetMiddleware(budget *policy.Budget, budgetName string, amount int) Middleware
    BudgetMiddleware returns middleware that enforces budget limits. It checks
    budget availability before execution and consumes on success.

func CachingMiddleware(c Cache, opts ...CacheOption) Middleware
    CachingMiddleware returns middleware that caches cacheable tool results.
    Only tools marked as cacheable (via annotations) will be cached. Accepts any
    cache.Cache implementation (memory, Redis, etc).

func ChainMiddleware(middlewares ...Middleware) Middleware
    ChainMiddleware composes multiple middleware into a single middleware.
    Middleware are executed in the order provided, with each wrapping the next.

func EligibilityMiddleware(eligibility *policy.ToolEligibility) Middleware
    EligibilityMiddleware returns middleware that enforces tool eligibility
    per state. It checks if the tool is allowed in the current state before
    execution.

func LedgerRecordingMiddleware(l *ledger.Ledger) Middleware
    LedgerRecordingMiddleware returns middleware that records tool calls to the
    ledger. This provides an audit trail of all tool executions.

func LegacyCachingMiddleware(legacyCache *LegacyMiddlewareCache) Middleware
    LegacyCachingMiddleware returns middleware using the deprecated LegacyCache.

    Deprecated: Use CachingMiddleware with Cache interface instead.

func LoggingMiddleware(cfg *LoggingMiddlewareConfig) Middleware
    LoggingMiddleware returns middleware that logs tool execution. Pass nil
    config for default settings (no input/output logging).

func NoopMiddleware() Middleware
    NoopMiddleware returns a middleware that does nothing, just passes through.

type MiddlewareRegistry = middleware.Registry
    MiddlewareRegistry manages an ordered list of middleware.

func NewMiddlewareRegistry() *MiddlewareRegistry
    NewMiddlewareRegistry creates a new middleware registry.

type NoopMetricsProvider = metrics.NoopProvider
    NoopMetricsProvider is a no-op implementation for testing.

type NotificationConfigSpec = domainconfig.NotificationConfig
    NotificationConfigSpec contains notification settings.

type Notifier = domainnotif.Notifier
    Notifier is the interface for sending notification events.

type Option func(*engineConfig)
    Option configures the Engine.

func WithApprover(a policy.Approver) Option
    WithApprover sets the approval handler.

func WithArtifactStore(s artifact.Store) Option
    WithArtifactStore sets the artifact store.

func WithBudget(name string, limit int) Option
    WithBudget sets a single budget limit. This is a convenience function that
    can be called multiple times.

func WithBudgets(budgets map[string]int) Option
    WithBudgets sets budget limits.

func WithEventStore(s event.Store) Option
    WithEventStore sets the event store for event sourcing and streaming.
    Required for the Stream() method to work.

func WithExecutor(e *resilience.Executor) Option
    WithExecutor sets the resilient executor.

func WithKnowledgeStore(s knowledge.Store) Option
    WithKnowledgeStore sets the knowledge store for RAG (Retrieval-Augmented
    Generation). The knowledge store enables agents to store and retrieve
    knowledge based on semantic similarity using vector embeddings.

func WithMaxSteps(n int) Option
    WithMaxSteps sets the maximum number of execution steps.

func WithMeter(m telemetry.Meter) Option
    WithMeter sets the OpenTelemetry meter for metrics collection.

func WithMetrics(provider Metrics) Option
    WithMetrics adds metrics middleware to the engine.

    This middleware records:
      - Tool execution count (with tool name, state, and success attributes)
      - Tool execution duration histogram
      - Errors (when tool execution fails)

    Example:

        engine, _ := api.New(
            api.WithPlanner(planner),
            api.WithMetrics(provider),
        )

func WithMiddleware(middlewares ...middleware.Middleware) Option
    WithMiddleware sets a custom middleware registry for tool execution.
    If not set, the engine uses a default middleware chain with: - Eligibility
    middleware (tool access control per state) - Approval middleware (human
    approval for destructive tools) - Logging middleware (execution timing and
    results)

func WithPerToolRateLimit(defaultRate, defaultBurst int, toolRates map[string]ToolRateConfig) Option
    WithPerToolRateLimit enables per-tool rate limiting. Each tool can have its
    own rate limit, falling back to defaults.

    Example:

        engine, _ := api.New(
            api.WithPlanner(planner),
            api.WithPerToolRateLimit(10, 10, map[string]api.ToolRateConfig{
                "fast_tool": {Rate: 100, Burst: 100},
                "slow_tool": {Rate: 5, Burst: 5},
            }),
        )

func WithPlanner(p planner.Planner) Option
    WithPlanner sets the planner.

func WithRateLimit(rate, burst int) Option
    WithRateLimit enables rate limiting for tool executions. This uses fortify's
    token bucket rate limiter to control request rates.

    Parameters:
      - rate: Number of tokens added per second
      - burst: Maximum tokens (bucket capacity) for handling bursts

    Example:

        engine, _ := api.New(
            api.WithPlanner(planner),
            api.WithRateLimit(100, 100), // 100 requests/sec, burst of 100
        )

func WithRegistry(r tool.Registry) Option
    WithRegistry sets the tool registry.

func WithRunStore(s run.Store) Option
    WithRunStore sets the run store for persistent run state. Runs are
    automatically saved on creation and updated on each step.

func WithTool(t tool.Tool) Option
    WithTool registers a tool with the engine's registry. Can be called multiple
    times to register multiple tools. If a tool with the same name already
    exists, it will be silently ignored. Use WithRegistry to get full control
    over tool registration errors.

func WithToolEligibility(e *policy.ToolEligibility) Option
    WithToolEligibility sets tool eligibility per state.

func WithTracer(t telemetry.Tracer) Option
    WithTracer sets the OpenTelemetry tracer for distributed tracing. When
    configured, the engine creates spans for runs, steps, planner decisions,
    and tool executions.

func WithTransitions(t *policy.StateTransitions) Option
    WithTransitions sets allowed state transitions.

type Pattern = pattern.Pattern
    Pattern represents a detected behavioral pattern.

type PatternDetector = pattern.Detector
    PatternDetector detects patterns from run data.

type PatternEvidence = pattern.PatternEvidence
    PatternEvidence records evidence supporting a pattern.

type PatternListFilter = pattern.ListFilter
    PatternListFilter filters pattern queries.

type PatternStore = pattern.Store
    PatternStore stores detected patterns.

type PatternType = pattern.PatternType
    PatternType categorizes patterns.

type PerformanceDetectorOption = infraPattern.PerformanceOption
    PerformanceDetectorOption configures the performance detector.

func WithLongRunThreshold(d time.Duration) PerformanceDetectorOption
    WithLongRunThreshold sets the threshold for long run detection.

func WithSlowToolThreshold(d time.Duration) PerformanceDetectorOption
    WithSlowToolThreshold sets the threshold for slow tool detection.

type PolicyApplier = infraProposal.PolicyApplier
    PolicyApplier applies approved proposals to policy configuration.

type PolicyChange = proposal.PolicyChange
    PolicyChange describes a change to be applied.

type PolicyConfig = domainconfig.PolicyConfig
    PolicyConfig contains policy settings.

type PolicyVersion = policy.PolicyVersion
    PolicyVersion represents a versioned policy snapshot.

type PolicyVersionStore = policy.VersionStore
    PolicyVersionStore stores policy versions.

type Proposal = proposal.Proposal
    Proposal represents a policy change proposal requiring human approval.

type ProposalEvidence = proposal.ProposalEvidence
    ProposalEvidence records evidence supporting a proposal.

type ProposalListFilter = proposal.ListFilter
    ProposalListFilter filters proposal queries.

type ProposalNote = proposal.ProposalNote
    ProposalNote is a comment on a proposal.

type ProposalStatus = proposal.ProposalStatus
    ProposalStatus tracks proposal lifecycle.

type ProposalStore = proposal.Store
    ProposalStore stores proposals.

type RateLimitConfigSpec = domainconfig.RateLimitConfig
    RateLimitConfigSpec configures rate limiting.

type RateLimitMetricsRecorder = inframw.RateLimitMetricsRecorder
    RateLimitMetricsRecorder records rate limit metrics.

func NewRateLimitMetricsRecorder(provider Metrics) RateLimitMetricsRecorder
    NewRateLimitMetricsRecorder creates a recorder for rate limit metrics.

type ResilienceConfig = domainconfig.ResilienceConfig
    ResilienceConfig contains resilience settings.

type RetryConfigSpec = domainconfig.RetryConfig
    RetryConfigSpec configures retry behavior.

type RiskLevel = tool.RiskLevel
    RiskLevel indicates the potential impact of a tool execution.

type Rule = planner.Rule
    Rule is a condition-decision pair for the rule-based planner.

type RuleBuilder = planner.RuleBuilder
    RuleBuilder constructs rules using a fluent API.

type Run = agent.Run
    Run represents a single execution of the agent.

type RunCompletedPayload = domainnotif.RunCompletedPayload
    Re-export domain notification types.

type RunExport = inspector.RunExport
    RunExport contains exported run data.

type RunFailedPayload = domainnotif.RunFailedPayload
    Re-export domain notification types.

type RunMetadata = inspector.RunMetadata
    RunMetadata contains run metadata.

type RunMetrics = inspector.RunMetrics
    RunMetrics contains run performance metrics.

type RunStartedPayload = domainnotif.RunStartedPayload
    Payload types for various event types.

type RunStatus = agent.RunStatus
    Re-export run status.

type RunStore = run.Store
    RunStore stores runs for pattern detection.

type ScriptStep = planner.ScriptStep
    ScriptStep is a step in a scripted planner.

type SearchResult = knowledge.SearchResult
    SearchResult represents a similarity search result.

type SenderConfig = infranotif.SenderConfig
    SenderConfig configures the HTTP sender.

func DefaultSenderConfig() SenderConfig
    DefaultSenderConfig returns sensible default configuration for the HTTP
    sender.

type Signer = infranotif.Signer
    Signer handles payload signing for webhook requests.

func NewSigner() *Signer
    NewSigner creates a new payload signer.

type State = agent.State
    State represents a structural constraint in the agent's execution.

type StateChangedPayload = domainnotif.StateChangedPayload
    Re-export domain notification types.

type StateExport = inspector.StateExport
    StateExport contains exported state data.

type StateMachineExport = inspector.StateMachineExport
    StateMachineExport contains exported state machine data.

type StateMachineTransition = inspector.StateMachineTransition
    StateMachineTransition contains state transition data.

type Suggestion = suggestion.Suggestion
    Suggestion represents a policy improvement suggestion.

type SuggestionChange = suggestion.PolicyChange
    SuggestionChange describes a proposed policy change.

type SuggestionChangeType = suggestion.PolicyChangeType
    SuggestionChangeType categorizes the type of change.

type SuggestionGenerator = suggestion.Generator
    SuggestionGenerator generates suggestions from patterns.

type SuggestionListFilter = suggestion.ListFilter
    SuggestionListFilter filters suggestion queries.

type SuggestionStatus = suggestion.SuggestionStatus
    SuggestionStatus tracks suggestion lifecycle.

type SuggestionStore = suggestion.Store
    SuggestionStore stores suggestions.

type SuggestionType = suggestion.SuggestionType
    SuggestionType categorizes suggestions.

type TimelineEntry = inspector.TimelineEntry
    TimelineEntry represents an entry in the run timeline.

type Tool = tool.Tool
    Tool represents a registered capability the agent can invoke.

type ToolAnnotationsConfig = domainconfig.ToolAnnotationsConfig
    ToolAnnotationsConfig configures tool annotations.

type ToolCallExport = inspector.ToolCallExport
    ToolCallExport contains exported tool call data.

type ToolCompletedPayload = domainnotif.ToolCompletedPayload
    Re-export domain notification types.

type ToolFailedPayload = domainnotif.ToolFailedPayload
    Re-export domain notification types.

type ToolHandlerConfig = domainconfig.ToolHandlerConfig
    ToolHandlerConfig specifies how to execute a tool.

type ToolPackConfig = domainconfig.ToolPackConfig
    ToolPackConfig configures a tool pack.

type ToolRateConfig struct {
	// Rate is the number of tokens added per second.
	Rate int
	// Burst is the maximum tokens (bucket capacity).
	Burst int
}
    ToolRateConfig configures rate limits for a specific tool.

type ToolRateLimitConfigSpec = domainconfig.ToolRateLimitConfig
    ToolRateLimitConfigSpec configures per-tool rate limiting.

type ToolResult = tool.Result
    ToolResult is returned by tool execution.

type ToolStartedPayload = domainnotif.ToolStartedPayload
    Re-export domain notification types.

type ToolsConfig = domainconfig.ToolsConfig
    ToolsConfig contains tool-related configuration.

type TransitionConfig = domainconfig.TransitionConfig
    TransitionConfig defines a state transition.

type TransitionExport = inspector.TransitionExport
    TransitionExport contains exported state transition data.

type TransitionRules = policy.TransitionRules
    TransitionRules maps states to the states they can transition to. This is
    the preferred way to configure state transitions declaratively.

    Example:

        transitions := api.NewStateTransitionsWith(api.TransitionRules{
            api.StateIntake:   {api.StateExplore, api.StateFailed},
            api.StateExplore:  {api.StateDecide, api.StateFailed},
            api.StateDecide:   {api.StateAct, api.StateDone, api.StateFailed},
        })

type ValidationError = domainconfig.ValidationError
    ValidationError represents a configuration validation error.

type ValidationErrors = domainconfig.ValidationErrors
    ValidationErrors is a collection of validation errors.

type Vector = knowledge.Vector
    Vector represents an embedding with associated text and metadata.

type WebhookNotifier = infranotif.WebhookNotifier
    WebhookNotifier sends notifications to configured webhook endpoints.

func NewWebhookNotifier(config WebhookNotifierConfig) *WebhookNotifier
    NewWebhookNotifier creates a new webhook notifier.

type WebhookNotifierConfig = infranotif.WebhookNotifierConfig
    WebhookNotifierConfig configures the webhook notifier.

func DefaultWebhookNotifierConfig() WebhookNotifierConfig
    DefaultWebhookNotifierConfig returns sensible default configuration for the
    webhook notifier.

type WorkflowService = infraProposal.WorkflowService
    WorkflowService manages the proposal approval workflow.
```
