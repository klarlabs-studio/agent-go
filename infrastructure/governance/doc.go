package governance

// Governance delegation to axi-go (agent-go spec § Changes Required #1).
//
// Active (default): the engine builds an AxiFactory unless EngineConfig.
// Governance is set. The destructive-tool approval gate is delegated to a
// shared axi.Kernel — the tool's risk annotations drive a write-external
// effect profile, the kernel pauses the session at awaiting_approval, and
// the configured approver settles it via Approve/Reject. When the factory
// OwnsApproval, the engine drops its approval middleware so the gate is
// enforced exactly once.
//
// Scope boundary (axi v1.4.0):
//   - Approval — delegated to axi. Native, per-call, the spec's headline
//     safety primitive.
//   - Budget — stays run-level in agent-go. axi's ExecutionBudget is
//     per-session (one action's capability fan-out) and cannot express a
//     run-spanning tool count without a held-session / incremental-
//     invocation handle that axi does not yet expose.
//   - Evidence — stays on the engine's run ledger (one chain per run).
//     Routing it through axi today would yield fragmented per-call chains,
//     a regression for a run-level audit trail.
//
// Path to full delegation: an axi held-session API (open a session with a
// run budget, invoke capabilities against it across the agent loop, close
// it) would let budget and the evidence chain move to axi as one session
// per run. That is an axi-side change — track it in the axi spec. Until
// then this split is the faithful, shippable boundary.
//
// Opt out with EngineConfig.Governance = NewPassthroughFactory(approver):
// budget + approval stay fully in-process (approval via the engine
// middleware), e.g. for LLM-free scripted runs or tests that do not want a
// kernel.
