// Delegation tiers for agent-go governance (spec § Changes Required #1).
// The canonical package comment lives in governance.go; this file documents
// the three Factory tiers and how full axi delegation works.
//
// Three Factories, increasing delegation:
//
//   - PassthroughFactory — budget + approval fully in-process (approval via
//     the engine's approval middleware). For LLM-free scripted runs or tests
//     that do not want a kernel. OwnsApproval reports false.
//
//   - AxiFactory — delegates only the destructive-tool approval gate to a
//     shared axi.Kernel; run-level budget stays in agent-go and evidence on
//     the engine ledger. OwnsApproval reports true. Selectable via
//     api.WithGovernance(NewAxiFactory(approver)).
//
//   - KernelFactory — full delegation (Track F). Each agent run executes as
//     ONE axi session, so budget, approval, and the evidence chain are all
//     axi-native. OwnsApproval reports true. This is the DEFAULT when
//     EngineConfig.Governance is unset — the only tier that satisfies the
//     spec non-negotiable "budget AND approval always through axi".
//
// How full delegation works without changing axi:
//
// axi's ExecutionBudget (MaxCapabilityInvocations) is enforced per session,
// across every CapabilityInvoker.Invoke call within one in-flight action
// execution (domain/execution.go: boundInvoker.checkInvocation, called from
// every Invoke). A held session is therefore realised in agent-go (the
// consumer) rather than added to axi: KernelFactory.Governor starts ONE
// kernel.Execute(runSessionAction) on a goroutine whose
// OrchestratorActionExecutor blocks on a channel, scoped to the run ctx (so
// cancellation/MaxDuration propagate). Each SUCCESSFUL act-state tool call
// drives one caps.Invoke through that in-flight session at Commit time, so
// axi's per-session budget enforcer counts run-level tool calls and fails the
// (N+1)th natively. The run loop stays statekit-driven; only the governed tool
// gate routes through the kernel session. No held-session API is added to axi
// — its independence is preserved.
//
//   - Budget — tool_calls maps onto the run session's
//     ExecutionBudget.MaxCapabilityInvocations. A slot is consumed only on
//     tool SUCCESS (at Commit), matching the passthrough and approval-only
//     Governors: a failed or denied tool burns nothing. axi is the
//     authoritative count of successful invocations within a segment (see
//     TestKernel_AxiSessionIsAuthoritativeBudgetEnforcer). The local
//     policy.Budget mirror is the authoritative PRECEDENCE gate, checked
//     (non-consuming) at Authorize before approval; it is the sole gate when
//     the remaining budget is zero, because axi treats
//     MaxCapabilityInvocations:0 as "no limit" — so the governor never
//     delegates a session for a non-positive remaining budget, and a zero
//     budget can never leak through as unlimited.
//   - Approval — destructive tools drive the write-external gate action
//     through the kernel, gated per call.
//   - Evidence — each committed tool call appends one EvidenceRecord to the
//     run's single axidomain.ExecutionSession, a tamper-evident chain per run
//     verified at run completion via VerifyEvidenceChain. Across a human-input
//     pause the chain is ONE continuous physical chain: the engine captures a
//     SessionSnapshot (axi's public Snapshot API) onto the run aggregate at
//     pause and rehydrates it via SessionFromSnapshot on resume, and the
//     run-spanning budget is seeded from the persisted consumed tool-call
//     count so it is not reset to full.
//
// This supersedes the earlier scope boundary that kept budget and evidence in
// agent-go pending an axi held-session API: the OrchestratorActionExecutor /
// CapabilityInvoker pattern already enforces a run-spanning budget within one
// session, so no axi change is required.
//
// axi v1.4.0 note: axi's budget-exceeded error is a formatted string, not a
// typed sentinel, so isBudgetExceeded does a substring match. A contract test
// (TestContract_AxiBudgetExceededMatchesDetector) pins that coupling so an axi
// upgrade that changes the wording breaks loudly.
package governance
