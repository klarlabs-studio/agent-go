package governance

// Migration status — governance delegation to axi-go (agent-go spec § Changes
// Required #1).
//
// Done (default build, go 1.25, no axi dependency):
//   - Governor seam (governance.go) — the single governance port the engine
//     uses for act-state tool authorization, budget accounting, and the
//     approval-ownership switch.
//   - Passthrough (passthrough.go) — behaviour-identical to the engine's
//     original inline budget enforcement; approval stays with the engine's
//     approval middleware (OwnsApproval == false).
//   - axiGovernor (axi.go, build tag "axi") — the route-through target:
//     budget, approval, and evidence owned by an axi.Kernel
//     (OwnsApproval == true). Excluded from the default build.
//
// Deferred (the axi-go toolchain/dependency bump) — activate with:
//   1. bump the agent-go module go directive: go 1.25.0 -> go 1.26.2
//      (axi-go v1.4.0's floor); propagate to contrib modules + CI.
//   2. go.mod: require go.klarlabs.de/axi v1.4.0
//   3. build and test with -tags axi
//   4. swap the engine's per-run Governor construction from
//      NewPassthrough(budget, approver) to NewAxi(limits, approver); when
//      OwnsApproval() is true the engine omits its approval middleware.
//   5. finalise the TODO(activation) points in axi.go (kernel action/
//      capability registration, evidence recording, budget projection)
//      against the live axi v1.4.0 API.
//
// Non-negotiable (agent-go spec): budget and approval always go through axi
// once activated. The domain/policy interfaces (Budget, Approval,
// Eligibility) are retained; this package binds them — it does not fork
// governance semantics.
