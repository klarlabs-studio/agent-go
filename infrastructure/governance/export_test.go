package governance

import axidomain "go.klarlabs.de/axi/domain"

// corruptEvidence is a test-only hook that tampers with a record on the run
// chain so verification failure can be exercised. It lives in _test.go so it
// is never compiled into production binaries. It round-trips the session
// through axi's public snapshot API, mutates the record's Value while leaving
// its Hash untouched, and reloads — so VerifyEvidenceChain recomputes a
// mismatching hash. No axi internals are touched; the chain is otherwise
// append-only. The g.evidence write is guarded by g.mu.
func (g *kernelGovernor) corruptEvidence(index int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	snap := g.evidence.ToSnapshot()
	if index < 0 || index >= len(snap.Evidence) {
		return
	}
	snap.Evidence[index].Value = "tampered"
	reloaded, err := axidomain.SessionFromSnapshot(snap)
	if err != nil {
		return
	}
	g.evidence = reloaded
}
