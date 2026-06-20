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
//     the engine ledger. OwnsApproval reports true. The default when
//     EngineConfig.Governance is unset.
//
//   - KernelFactory — full delegation (Track F). Each agent run executes as
//     ONE axi session, so budget, approval, and the evidence chain are all
//     axi-native. OwnsApproval reports true. Select it with
//     EngineConfig.Governance = NewKernelFactory(approver) (api.WithGovernance).
//
// How full delegation works without changing axi:
//
// axi's ExecutionBudget (MaxCapabilityInvocations) is enforced per session,
// across every CapabilityInvoker.Invoke call within one in-flight action
// execution (domain/execution.go: boundInvoker.checkInvocation, called from
// every Invoke). A held session is therefore realised in agent-go (the
// consumer) rather than added to axi: KernelFactory.Governor starts ONE
// kernel.Execute(runSessionAction) on a goroutine whose
// OrchestratorActionExecutor blocks on a channel. Each act-state tool call
// drives one caps.Invoke through that in-flight session, so axi's per-session
// budget enforcer counts run-level tool calls and fails the (N+1)th natively
// with the budget error. The run loop stays statekit-driven; only the
// governed tool gate routes through the kernel session. No held-session API
// is added to axi — its independence is preserved.
//
//   - Budget — tool_calls maps onto the run session's
//     ExecutionBudget.MaxCapabilityInvocations. axi is the authoritative
//     enforcer (see TestKernel_AxiSessionIsAuthoritativeBudgetEnforcer); the
//     local policy.Budget mirror only orders budget precedence before
//     approval and keeps BudgetSnapshot in lockstep.
//   - Approval — destructive tools drive the write-external gate action
//     through the kernel, gated per call.
//   - Evidence — each committed tool call appends one EvidenceRecord to the
//     run's single axidomain.ExecutionSession, a tamper-evident chain per run
//     verified at run completion via VerifyEvidenceChain.
//
// This supersedes the earlier scope boundary that kept budget and evidence in
// agent-go pending an axi held-session API: the OrchestratorActionExecutor /
// CapabilityInvoker pattern already enforces a run-spanning budget within one
// session, so no axi change is required.
package governance
