package runner

import "testing"

func TestTraceGraphPreservesTransitivePath(t *testing.T) {
	graph := SemanticGraph{Edges: []GraphEdge{
		{EdgeID: "b", From: "predicate:root", To: "predicate:leaf", Status: "KNOWN"},
		{EdgeID: "a", From: "predicate:root", To: "predicate:middle", Status: "KNOWN"},
		{EdgeID: "c", From: "predicate:middle", To: "test:leaf", Status: "KNOWN"},
	}}
	paths, affected, unknown := traceGraph(graph, []string{"predicate:root"})
	if len(unknown) != 0 || !affected["test:leaf"] {
		t.Fatalf("graph traversal did not reach leaf: affected=%v unknown=%v", affected, unknown)
	}
	if got := edgeIDs(paths["test:leaf"]); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("unexpected shortest path: %v", got)
	}
}

func TestUnknownImpactEdgeHasExactlySixFields(t *testing.T) {
	unknown := unknownDetail("AFFECTED_SEMANTIC_PREDICATES", "resolve-impact-edge", "UNKNOWN_IMPACT_EDGE", "CAUSAL_EDGE_UNKNOWN", "OBTAIN_SEMANTIC_GRAPH_EDGE_PROOF", []string{"edge:e1"})
	if unknown.Stage == "" || unknown.Step == "" || unknown.Reason == "" || unknown.UnknownClass == "" || unknown.NextOperation == "" || len(unknown.BlockedBy) != 1 {
		t.Fatalf("unknown causal frontier is incomplete: %+v", unknown)
	}
}

func TestProofIdentityMismatchIsRefuted(t *testing.T) {
	proof := ReusableProof{ProofID: "proof", TestID: "test", SourceDigest: "old", SourceTreeDigest: "tree", SemanticDigest: "semantic", GraphDigest: "graph", ToolchainDigest: "toolchain", ScenarioDigest: "scenario", TestInventoryDigest: "inventory", CommandDigest: "command", TerminalResult: "PASS", ResultDigest: "result", Release: ReleaseEvidence{Provider: "github", Tag: "v0.1.0", AssetDigest: "asset", PlatformImmutable: true}}
	decision := evaluateProof(proof, "test", Bindings{SourceDigest: "current", SourceTreeDigest: "tree", SemanticDigest: "semantic", GraphDigest: "graph", ToolchainDigest: "toolchain", ScenarioDigest: "scenario", TestInventoryDigest: "inventory", CommandDigest: "command"})
	if decision.State != Refuted || decision.Reason != "STALE_PROOF_REJECTED_SOURCE_DIGEST" {
		t.Fatalf("stale proof was not refuted: %+v", decision)
	}
}

func TestMissingPerformancePairDoesNotInventSavings(t *testing.T) {
	performance := assessPerformance(nil, Bindings{})
	if performance.State != Unknown || performance.SavedTimeMS != "UNKNOWN" || performance.Improvement != "UNKNOWN" {
		t.Fatalf("missing performance pair was not kept unknown: %+v", performance)
	}
}

func TestRefutedPrecedesUnknown(t *testing.T) {
	decision, reason := aggregateDecision([]string{"STALE_PROOF_REJECTED_SCENARIO_DIGEST"}, []UnknownDetail{{Reason: "UNKNOWN_IMPACT_EDGE"}}, OracleComparison{}, true)
	if decision != Refuted || reason != "STALE_PROOF_REJECTED_SCENARIO_DIGEST" {
		t.Fatalf("resolution precedence changed: decision=%s reason=%s", decision, reason)
	}
}
