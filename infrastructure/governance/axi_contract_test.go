package governance

import (
	"context"
	"testing"

	"go.klarlabs.de/axi"
	axidomain "go.klarlabs.de/axi/domain"
)

// isBudgetExceeded relies on a substring of axi's per-session budget-exceeded
// error message, because axi v1.4.0 does not yet expose a typed sentinel. This
// contract test pins that coupling: it drives a real axi session past its
// MaxCapabilityInvocations limit and asserts the resulting error still matches
// isBudgetExceeded. If an axi upgrade changes the wording, this test breaks
// loudly instead of letting full-delegation budget enforcement silently fail.
func TestContract_AxiBudgetExceededMatchesDetector(t *testing.T) {
	const (
		capName  = "contract.cap"
		capExec  = "exec.contract.cap"
		actName  = "contract.act"
		actExec  = "exec.contract.act"
		pluginID = "contract.plugin"
	)

	// invokeErr captures the error axi returns from CapabilityInvoker.Invoke
	// when the budget is tripped — the exact seam the full-delegation governor
	// reads (its runSessionExecutor returns caps.Invoke's error on respCh).
	invokeErrCh := make(chan error, 1)

	kernel := axi.New()
	kernel.WithBudget(axi.Budget{MaxCapabilityInvocations: 1})
	kernel.RegisterCapabilityExecutor(capExec, contractCapExecutor{})
	kernel.RegisterActionExecutor(actExec, &contractActExecutor{invokeErrCh: invokeErrCh})
	if err := kernel.RegisterPlugin(contractPlugin{
		capName: capName, capExec: capExec,
		actName: actName, actExec: actExec, pluginID: pluginID,
	}); err != nil {
		t.Fatalf("RegisterPlugin: %v", err)
	}

	if _, err := kernel.Execute(context.Background(), axi.Invocation{Action: actName}); err != nil {
		t.Fatalf("kernel.Execute: %v", err)
	}

	select {
	case err := <-invokeErrCh:
		if err == nil {
			t.Fatal("expected axi to fail the second capability invocation on budget")
		}
		if !isBudgetExceeded(err) {
			t.Fatalf("axi budget-exceeded error no longer matches isBudgetExceeded; "+
				"the substring contract changed — update isBudgetExceeded. err=%v", err)
		}
	default:
		t.Fatal("executor did not report a CapabilityInvoker.Invoke error")
	}
}

type contractCapExecutor struct{}

func (contractCapExecutor) Execute(_ context.Context, _ any) (any, error) {
	return struct{}{}, nil
}

// contractActExecutor invokes the capability twice within one session, so the
// second Invoke trips MaxCapabilityInvocations:1. It reports the second
// Invoke's error on invokeErrCh (mirroring the full-delegation governor, which
// reads caps.Invoke's error directly rather than via the session result).
type contractActExecutor struct {
	invokeErrCh chan error
}

func (e *contractActExecutor) Execute(
	_ context.Context,
	_ any,
	caps axidomain.CapabilityInvoker,
) (axidomain.ExecutionResult, []axidomain.EvidenceRecord, error) {
	_, _ = caps.Invoke("contract.cap", nil)
	_, err := caps.Invoke("contract.cap", nil)
	e.invokeErrCh <- err
	return axidomain.ExecutionResult{Summary: "done"}, nil, nil
}

type contractPlugin struct {
	capName, capExec string
	actName, actExec string
	pluginID         string
}

func (p contractPlugin) Contribute() (*axidomain.PluginContribution, error) {
	cap, err := axidomain.NewCapabilityDefinition(
		axidomain.CapabilityName(p.capName),
		"contract capability",
		axidomain.EmptyContract(),
		axidomain.EmptyContract(),
	)
	if err != nil {
		return nil, err
	}
	if err := cap.BindExecutor(axidomain.CapabilityExecutorRef(p.capExec)); err != nil {
		return nil, err
	}
	req, err := axidomain.NewRequirementSet(axidomain.Requirement{Capability: axidomain.CapabilityName(p.capName)})
	if err != nil {
		return nil, err
	}
	act, err := axidomain.NewActionDefinition(
		axidomain.ActionName(p.actName),
		"contract action",
		axidomain.EmptyContract(),
		axidomain.EmptyContract(),
		req,
		axidomain.EffectProfile{Level: axidomain.EffectNone},
		axidomain.IdempotencyProfile{IsIdempotent: false},
	)
	if err != nil {
		return nil, err
	}
	if err := act.BindExecutor(axidomain.ActionExecutorRef(p.actExec)); err != nil {
		return nil, err
	}
	return axidomain.NewPluginContribution(
		axidomain.PluginID(p.pluginID),
		[]*axidomain.ActionDefinition{act},
		[]*axidomain.CapabilityDefinition{cap},
	)
}
