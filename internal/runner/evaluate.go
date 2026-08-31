package runner

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

func EvaluateCase(value Case, source SourceSpec, sourceDigest, treeDigest, contractDigest string, contract Contract, ir SemanticIR, generatedDigest string, inventory Inventory, measurements RuntimeMeasurements) (Plan, Receipt, error) {
	if err := validateCase(value); err != nil {
		return Plan{}, Receipt{}, err
	}
	if err := validateContract(contract, source); err != nil {
		return Plan{}, Receipt{}, err
	}

	scenarioDigest, err := ScenarioDigest(value)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	graph := value.SemanticGraph
	graph.SourceDigest = resolveToken(graph.SourceDigest, Bindings{SourceDigest: sourceDigest})
	graph.SemanticDigest = resolveToken(graph.SemanticDigest, Bindings{SemanticDigest: ir.Digest})
	declaredGraphDigest := graph.GraphDigest
	graph.GraphDigest = ""
	graphDigest, err := GraphDigest(graph)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	if declaredGraphDigest != "" && declaredGraphDigest != "$GRAPH_DIGEST" && declaredGraphDigest != graphDigest {
		graph.GraphDigest = declaredGraphDigest
	} else {
		graph.GraphDigest = graphDigest
	}

	oracle := value.FullOracle
	declaredOracleDigest := oracle.Digest
	oracle.Digest = ""
	oracleDigest, err := OracleDigest(oracle)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	if declaredOracleDigest != "" && declaredOracleDigest != "$FULL_ORACLE_DIGEST" && declaredOracleDigest != oracleDigest {
		oracle.Digest = declaredOracleDigest
	} else {
		oracle.Digest = oracleDigest
	}
	oracleDigestMismatch := declaredOracleDigest != "" && declaredOracleDigest != "$FULL_ORACLE_DIGEST" && declaredOracleDigest != oracleDigest

	toolchainDigest := DigestBytes([]byte(runtime.Version() + "|" + runtime.GOOS + "|" + runtime.GOARCH + "|github.actions"))
	testInventoryDigest, err := DigestJSON(value.Tests)
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	commandDigest, err := DigestJSON([]string{"gooo-causal-verification-runner", "conformance", "--full-oracle-compare"})
	if err != nil {
		return Plan{}, Receipt{}, err
	}
	bindings := Bindings{
		SourceDigest: sourceDigest, SourceTreeDigest: treeDigest, SemanticDigest: ir.Digest,
		GraphDigest: graphDigest, ToolchainDigest: toolchainDigest, ScenarioDigest: scenarioDigest,
		TestInventoryDigest: testInventoryDigest, CommandDigest: commandDigest, OracleDigest: oracleDigest,
	}

	unknowns := make([]UnknownDetail, 0)
	refutations := make([]string, 0)
	if value.CacheHit && len(value.ReusableProofs) == 0 {
		refutations = append(refutations, "CACHE_HIT_WITHOUT_EXACT_PROOF")
	}
	if value.ChangeClaim.SourceDigest != "" && resolveToken(value.ChangeClaim.SourceDigest, bindings) != sourceDigest {
		refutations = append(refutations, "CHANGE_CLAIM_SOURCE_DIGEST_MISMATCH")
	}
	if value.ChangeClaim.SemanticDigest != "" && resolveToken(value.ChangeClaim.SemanticDigest, bindings) != ir.Digest {
		refutations = append(refutations, "CHANGE_CLAIM_SEMANTIC_DIGEST_MISMATCH")
	}
	if graph.Schema != "gooo-graph/v1" || graph.SourceDigest != sourceDigest || graph.SemanticDigest != ir.Digest || graph.GraphDigest != graphDigest {
		refutations = append(refutations, "SEMANTIC_GRAPH_DIGEST_MISMATCH")
	}
	if oracleDigestMismatch {
		refutations = append(refutations, "FULL_ORACLE_DIGEST_MISMATCH")
	}

	paths, affectedNodes, unknownEdges := traceGraph(graph, value.ChangeClaim.ChangedPredicates)
	if len(unknownEdges) > 0 {
		unknownIDs := make([]string, 0, len(unknownEdges))
		for _, edge := range unknownEdges {
			unknownIDs = append(unknownIDs, "edge:"+edge.EdgeID)
		}
		unknowns = append(unknowns, unknownDetail("AFFECTED_SEMANTIC_PREDICATES", "resolve-impact-edge", "UNKNOWN_IMPACT_EDGE", "CAUSAL_EDGE_UNKNOWN", "OBTAIN_SEMANTIC_GRAPH_EDGE_PROOF", unknownIDs))
	}
	fullFallback := len(unknownEdges) > 0
	selectionMode := "CAUSAL_SELECT"
	if fullFallback {
		selectionMode = "FULL_VERIFICATION"
	}

	proofByTest := make(map[string]ReusableProof, len(value.ReusableProofs))
	for _, proof := range value.ReusableProofs {
		if _, exists := proofByTest[proof.TestID]; exists {
			refutations = append(refutations, "DUPLICATE_REUSABLE_PROOF_"+proof.TestID)
			continue
		}
		proofByTest[proof.TestID] = resolveProof(proof, bindings)
	}
	proofDecisions := make([]ProofDecision, 0, len(value.ReusableProofs))
	testDecisions := make([]TestDecision, 0, len(value.Tests))
	requiredTests := make([]string, 0)
	frontierByID := make(map[string]GraphEdge)
	for _, test := range value.Tests {
		path := paths[test.TargetNode]
		affected := len(path) > 0 || affectedNodes[test.TargetNode]
		if affected {
			requiredTests = append(requiredTests, test.TestID)
			for _, edge := range path {
				frontierByID[edge.EdgeID] = edge
			}
		}
		decision := TestDecision{TestID: test.TestID, TargetNode: test.TargetNode, Affected: affected, Path: edgeIDs(path)}
		switch {
		case fullFallback:
			decision.Action = "EXECUTE"
			decision.Reason = "UNKNOWN_IMPACT_EDGE_REQUIRES_FULL_VERIFICATION"
		case affected:
			decision.Action = "EXECUTE"
			decision.Reason = "AFFECTED_SEMANTIC_PREDICATE_REQUIRES_EXECUTION"
		case proofByTest[test.TestID].ProofID != "":
			proof := proofByTest[test.TestID]
			proofDecision := evaluateProof(proof, test.TestID, bindings)
			proofDecisions = append(proofDecisions, proofDecision)
			if proofDecision.State == Closed {
				decision.Action = "REUSE"
				decision.ProofID = proof.ProofID
				decision.ResultDigest = proof.ResultDigest
				decision.Reason = "EXACT_IMMUTABLE_PROOF_REUSED"
			} else {
				decision.Action = "EXECUTE"
				decision.Reason = proofDecision.Reason
				if proofDecision.State == Refuted {
					refutations = append(refutations, proofDecision.Reason)
				} else if proofDecision.Unknown != nil {
					unknowns = append(unknowns, *proofDecision.Unknown)
				}
			}
		default:
			decision.Action = "EXECUTE"
			decision.Reason = "NO_REUSABLE_PROOF_OBSERVED"
		}
		testDecisions = append(testDecisions, decision)
	}
	if fullFallback {
		requiredTests = requiredTests[:0]
		for _, test := range value.Tests {
			requiredTests = append(requiredTests, test.TestID)
		}
		for _, edge := range unknownEdges {
			frontierByID[edge.EdgeID] = edge
		}
	}
	frontier := sortedEdges(frontierByID)
	sort.Strings(requiredTests)

	for _, test := range value.Tests {
		if _, exists := proofByTest[test.TestID]; exists {
			if paths[test.TargetNode] != nil || affectedNodes[test.TargetNode] {
				proofDecisions = append(proofDecisions, ProofDecision{ProofID: proofByTest[test.TestID].ProofID, TestID: test.TestID, State: Closed, Reason: "AFFECTED_TEST_CANNOT_USE_REUSABLE_PROOF"})
			}
		}
	}
	sort.Slice(proofDecisions, func(i, j int) bool { return proofDecisions[i].TestID < proofDecisions[j].TestID })

	oracleComparison, oracleUnknowns, oracleRefutations := compareWithFullOracle(testDecisions, value.Tests, oracle, bindings)
	unknowns = append(unknowns, oracleUnknowns...)
	refutations = append(refutations, oracleRefutations...)
	for _, counterexample := range value.Counterexamples {
		if counterexample.ExpectedInvalidation != counterexample.ObservedInvalidation {
			refutations = append(refutations, "HIDDEN_COUNTEREXAMPLE_REFUTED:"+counterexample.CounterexampleID)
		}
	}

	performance := assessPerformance(value.PerformancePair, bindings)
	if len(refutations) > 0 {
		refutations = uniqueStrings(refutations)
	}
	decision, reason := aggregateDecision(refutations, unknowns, oracleComparison, fullFallback)
	claim := value.ChangeClaim
	claim.SourceDigest = resolveToken(claim.SourceDigest, bindings)
	claim.SemanticDigest = resolveToken(claim.SemanticDigest, bindings)
	metrics := buildMetrics(testDecisions, oracle, measurements, value.PerformancePair, bindings, unknowns)
	activities := buildActivityDecisions(contract.Cells, decision, reason, unknowns, refutations, fullFallback)
	plan := Plan{
		Schema: PlanSchema, CaseID: value.CaseID, Decision: decision, SelectionMode: selectionMode, DecisionReason: reason,
		Bindings: bindings, Activities: activities, ChangeClaim: claim, AffectedSemanticPredicates: affectedPredicates(affectedNodes), RequiredTests: requiredTests,
		Proofs: proofDecisions, Tests: testDecisions, FullOracleComparison: oracleComparison, Unknowns: unknowns, Refutations: refutations,
		Metrics: metrics, Performance: performance, InvalidationFrontier: frontier,
		Authority: Authority{RepositoryWrites: 0, LocalTestExecutions: 0, CrossProjectRequiredGates: 0, OutputLocation: "CALLER_OWNED_TEMP_ONLY", VerificationAuthority: "GITHUB_ACTIONS"},
	}
	receipt := Receipt{
		Schema: ReceiptSchema, CaseID: value.CaseID, Decision: decision, SelectionMode: selectionMode, DecisionReason: reason,
		Bindings:           bindings,
		Source:             ArtifactBinding{Path: ir.SourcePath, Digest: sourceDigest},
		SemanticIR:         ArtifactBinding{Path: "semantic-ir.json", Digest: ir.Digest},
		GeneratedEvaluator: ArtifactBinding{Path: "generated/evaluator.go", Digest: generatedDigest},
		Contract:           ArtifactBinding{Path: "contracts/causal-verification-denominator-v1.json", Digest: contractDigest},
		SemanticGraph:      ArtifactBinding{Path: "input.semantic-graph", Digest: graphDigest}, FullOracle: ArtifactBinding{Path: "input.full-oracle", Digest: oracleDigest},
		ScenarioDigest: scenarioDigest, Activities: activities, AffectedSemanticPredicates: affectedPredicates(affectedNodes), RequiredTests: requiredTests,
		Tests: testDecisions, InvalidationFrontier: frontier,
		Metrics: metrics, Unknowns: unknowns, Refutations: refutations, FullOracleComparison: oracleComparison, Performance: performance,
		Authority: plan.Authority, RootReadmeExcluded: inventory.RootReadmeExcluded,
	}
	return plan, receipt, nil
}

func validateCase(value Case) error {
	if value.Schema != "gooo/causal-verification-runner/case/v1" || value.CaseID == "" || value.ChangeClaim.ClaimID == "" || len(value.ChangeClaim.ChangedPredicates) == 0 {
		return fmt.Errorf("malformed causal verification case")
	}
	seen := map[string]bool{}
	for _, test := range value.Tests {
		if test.TestID == "" || test.TargetNode == "" || len(test.Command) == 0 || seen[test.TestID] {
			return fmt.Errorf("malformed or duplicate test %q", test.TestID)
		}
		seen[test.TestID] = true
	}
	if len(value.Tests) == 0 || !value.FullOracle.Independent || value.FullOracle.OracleID == "" {
		return fmt.Errorf("full oracle is not independent or test inventory is empty")
	}
	graphNodes := map[string]bool{}
	for _, predicate := range value.SemanticGraph.Predicates {
		if predicate == "" || graphNodes[predicate] {
			return fmt.Errorf("malformed or duplicate semantic predicate %q", predicate)
		}
		graphNodes[predicate] = true
	}
	edgeIDs := map[string]bool{}
	for _, edge := range value.SemanticGraph.Edges {
		if edge.EdgeID == "" || edge.From == "" || edge.To == "" || edgeIDs[edge.EdgeID] || (edge.Status != "KNOWN" && edge.Status != "UNKNOWN") {
			return fmt.Errorf("malformed or duplicate semantic graph edge %q", edge.EdgeID)
		}
		edgeIDs[edge.EdgeID] = true
	}
	oracleTests := map[string]bool{}
	for _, result := range value.FullOracle.Results {
		if result.TestID == "" || (result.Status != "PASS" && result.Status != "FAIL") || result.ResultDigest == "" || oracleTests[result.TestID] {
			return fmt.Errorf("malformed or duplicate full oracle result %q", result.TestID)
		}
		oracleTests[result.TestID] = true
	}
	if len(oracleTests) != len(value.Tests) {
		return fmt.Errorf("full oracle does not cover every test")
	}
	return nil
}

func validateContract(contract Contract, source SourceSpec) error {
	if contract.Schema != "gooo/causal-verification-runner/denominator/v1" || contract.FixedDenominator != 12 || len(contract.Cells) != 12 || len(source.Activities) != 12 || len(contract.Precedence) != 3 || contract.Precedence[0] != Refuted || contract.Precedence[1] != Unknown || contract.Precedence[2] != Closed {
		return fmt.Errorf("fixed twelve-cell denominator is required")
	}
	proofTotals := map[string]int{}
	indicatorTotals := map[string]int{}
	seenIDs := map[string]bool{}
	seenActivities := map[string]bool{}
	for index, cell := range contract.Cells {
		if cell.Ordinal != index+1 || cell.Activity != source.Activities[index].Name || cell.ID == "" || cell.Stage == "" || cell.Step == "" || cell.ProofChoice == "" || cell.IndicatorClass == "" || seenIDs[cell.ID] || seenActivities[cell.Activity] {
			return fmt.Errorf("source and denominator activity %d do not match", index+1)
		}
		activity := source.Activities[index]
		if metadataValue(activity.Computes, "stage") != cell.Stage || metadataValue(activity.Computes, "step") != cell.Step || metadataValue(activity.Computes, "proof") != cell.ProofChoice || metadataValue(activity.Computes, "indicator") != cell.IndicatorClass {
			return fmt.Errorf(".gooo metadata and denominator activity %d do not match", index+1)
		}
		seenIDs[cell.ID] = true
		seenActivities[cell.Activity] = true
		proofTotals[cell.ProofChoice]++
		indicatorTotals[cell.IndicatorClass]++
	}
	for _, choice := range []string{"FOUNDATION", "COHERENCE", "REGRESSION"} {
		if contract.ProofTotals[choice] != 4 || proofTotals[choice] != 4 {
			return fmt.Errorf("proof denominator is not balanced")
		}
	}
	for _, indicator := range []string{"DRIVER", "OUTCOME", "GUARDRAIL"} {
		if contract.IndicatorTotals[indicator] != 4 || indicatorTotals[indicator] != 4 {
			return fmt.Errorf("indicator denominator is not balanced")
		}
	}
	return nil
}

func traceGraph(graph SemanticGraph, starts []string) (map[string][]GraphEdge, map[string]bool, []GraphEdge) {
	edges := append([]GraphEdge(nil), graph.Edges...)
	sort.Slice(edges, func(i, j int) bool { return edges[i].EdgeID < edges[j].EdgeID })
	adj := map[string][]GraphEdge{}
	for _, edge := range edges {
		adj[edge.From] = append(adj[edge.From], edge)
	}
	queue := append([]string(nil), starts...)
	sort.Strings(queue)
	paths := map[string][]GraphEdge{}
	affected := map[string]bool{}
	for _, start := range queue {
		if !affected[start] {
			affected[start] = true
			paths[start] = []GraphEdge{}
		}
	}
	unknownByID := map[string]GraphEdge{}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, edge := range adj[from] {
			if edge.Status != "KNOWN" {
				unknownByID[edge.EdgeID] = edge
				continue
			}
			if affected[edge.To] {
				continue
			}
			affected[edge.To] = true
			path := append([]GraphEdge(nil), paths[from]...)
			path = append(path, edge)
			paths[edge.To] = path
			queue = append(queue, edge.To)
		}
	}
	unknown := make([]GraphEdge, 0, len(unknownByID))
	for _, edge := range unknownByID {
		unknown = append(unknown, edge)
	}
	sort.Slice(unknown, func(i, j int) bool { return unknown[i].EdgeID < unknown[j].EdgeID })
	return paths, affected, unknown
}

func evaluateProof(proof ReusableProof, testID string, current Bindings) ProofDecision {
	result := ProofDecision{ProofID: proof.ProofID, TestID: testID, State: Closed, Reason: "EXACT_IMMUTABLE_PROOF_BOUND"}
	if proof.ProofID == "" || proof.TestID != testID {
		return refutedProof(result, "REUSABLE_PROOF_TEST_ID_MISMATCH")
	}
	if !proof.Release.PlatformImmutable {
		if proof.Release.SelfAsserted {
			return refutedProof(result, "SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM")
		}
		return refutedProof(result, "RELEASE_IMMUTABILITY_NOT_CONFIRMED_BY_PLATFORM")
	}
	if proof.Release.AssetDigest == "" || proof.Release.Provider != "github" || proof.Release.Tag == "" {
		return refutedProof(result, "IMMUTABLE_RELEASE_EVIDENCE_INCOMPLETE")
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"SOURCE_DIGEST", proof.SourceDigest, current.SourceDigest},
		{"SOURCE_TREE_DIGEST", proof.SourceTreeDigest, current.SourceTreeDigest},
		{"SEMANTIC_DIGEST", proof.SemanticDigest, current.SemanticDigest},
		{"GRAPH_DIGEST", proof.GraphDigest, current.GraphDigest},
		{"TOOLCHAIN_DIGEST", proof.ToolchainDigest, current.ToolchainDigest},
		{"SCENARIO_DIGEST", proof.ScenarioDigest, current.ScenarioDigest},
		{"TEST_INVENTORY_DIGEST", proof.TestInventoryDigest, current.TestInventoryDigest},
		{"COMMAND_DIGEST", proof.CommandDigest, current.CommandDigest},
	}
	for _, check := range checks {
		if check.got == "" {
			result.State = Unknown
			result.Reason = "REUSABLE_PROOF_FIELD_MISSING_" + check.name
			detail := unknownDetail("REUSABLE_PROOFS", "verify-proof-identity", result.Reason, "DIRECT_MISSING", "OBTAIN_EXACT_IMMUTABLE_PROOF", []string{"proof:" + proof.ProofID, strings.ToLower(check.name)})
			result.Unknown = &detail
			return result
		}
		if check.got != check.want {
			return refutedProof(result, "STALE_PROOF_REJECTED_"+check.name)
		}
	}
	if proof.TerminalResult != "PASS" {
		return refutedProof(result, "REUSABLE_PROOF_TERMINAL_RESULT_NOT_PASS")
	}
	if proof.ResultDigest == "" {
		return refutedProof(result, "REUSABLE_PROOF_RESULT_DIGEST_MISSING")
	}
	return result
}

func refutedProof(result ProofDecision, reason string) ProofDecision {
	result.State = Refuted
	result.Reason = reason
	return result
}

func compareWithFullOracle(decisions []TestDecision, tests []TestSpec, oracle FullOracle, current Bindings) (OracleComparison, []UnknownDetail, []string) {
	comparison := OracleComparison{State: Closed, Reason: "SELECTIVE_RESULT_EQUALS_INDEPENDENT_FULL_ORACLE", Independent: oracle.Independent, OracleID: oracle.OracleID, OracleDigest: current.OracleDigest, Mismatches: []OracleMismatch{}}
	unknowns := make([]UnknownDetail, 0)
	refutations := make([]string, 0)
	if !oracle.Independent {
		comparison.State = Unknown
		comparison.Reason = "FULL_ORACLE_INDEPENDENCE_NOT_OBSERVED"
		unknowns = append(unknowns, unknownDetail("INDEPENDENT_FULL_ORACLE_COMPARISON", "bind-full-oracle", comparison.Reason, "DIRECT_MISSING", "OBTAIN_INDEPENDENT_FULL_ORACLE", []string{"full_oracle.independent"}))
		return comparison, unknowns, refutations
	}
	byTest := map[string]TestSpec{}
	for _, test := range tests {
		byTest[test.TestID] = test
	}
	resultByTest := map[string]string{}
	statusByTest := map[string]string{}
	for _, decision := range decisions {
		switch decision.Action {
		case "REUSE":
			resultByTest[decision.TestID] = decision.ResultDigest
			statusByTest[decision.TestID] = "PASS"
		case "EXECUTE":
			test := byTest[decision.TestID]
			if test.Observation == nil {
				unknowns = append(unknowns, unknownDetail("EXECUTED_TESTS", "observe-selected-test", "SELECTED_TEST_OBSERVATION_MISSING", "DIRECT_MISSING", "EXECUTE_SELECTED_TEST_AND_RECORD_RECEIPT", []string{"test:" + decision.TestID}))
				continue
			}
			if test.Observation.WallMS < 0 || test.Observation.PeakRSSKiB < 0 {
				refutations = append(refutations, "NEGATIVE_SELECTED_TEST_METRIC_"+decision.TestID)
				continue
			}
			if test.Observation.Status != "PASS" && test.Observation.Status != "FAIL" {
				refutations = append(refutations, "INVALID_SELECTED_TEST_STATUS_"+decision.TestID)
				continue
			}
			resultByTest[decision.TestID] = test.Observation.ResultDigest
			statusByTest[decision.TestID] = test.Observation.Status
		}
	}
	fullExecuted := 0
	for _, expected := range oracle.Results {
		if expected.Status == "PASS" || expected.Status == "FAIL" {
			fullExecuted++
		}
		actualDigest, digestObserved := resultByTest[expected.TestID]
		actualStatus, statusObserved := statusByTest[expected.TestID]
		if !digestObserved || !statusObserved {
			continue
		}
		if actualDigest != expected.ResultDigest || actualStatus != expected.Status {
			comparison.Mismatches = append(comparison.Mismatches, OracleMismatch{TestID: expected.TestID, Reason: "SELECTIVE_RESULT_DIFFERS_FROM_FULL_ORACLE"})
		}
	}
	comparison.FullOracleExecuted = fullExecuted
	if len(comparison.Mismatches) > 0 {
		comparison.State = Refuted
		comparison.Reason = "SELECTIVE_FULL_ORACLE_MISMATCH"
		refutations = append(refutations, "SELECTIVE_FULL_ORACLE_MISMATCH")
	}
	for _, expected := range oracle.Results {
		if expected.Status == "FAIL" {
			refutations = append(refutations, "FULL_ORACLE_FAILURE_"+expected.TestID)
		}
	}
	sort.Slice(comparison.Mismatches, func(i, j int) bool { return comparison.Mismatches[i].TestID < comparison.Mismatches[j].TestID })
	return comparison, unknowns, refutations
}

func assessPerformance(pair *PerformancePair, current Bindings) PerformanceAssessment {
	if pair == nil {
		return PerformanceAssessment{State: Unknown, Reason: "BEFORE_AFTER_PAIR_NOT_OBSERVED", SavedTimeMS: "UNKNOWN", Improvement: "UNKNOWN"}
	}
	before := pair.Before
	after := pair.After
	if resolveToken(before.SourceDigest, current) != current.SourceDigest || resolveToken(after.SourceDigest, current) != current.SourceDigest ||
		resolveToken(before.ToolchainDigest, current) != current.ToolchainDigest || resolveToken(after.ToolchainDigest, current) != current.ToolchainDigest ||
		resolveToken(before.ScenarioDigest, current) != current.ScenarioDigest || resolveToken(after.ScenarioDigest, current) != current.ScenarioDigest {
		return PerformanceAssessment{State: Unknown, Reason: "BEFORE_AFTER_PAIR_KEY_MISMATCH", Before: &before, After: &after, SavedTimeMS: "UNKNOWN", Improvement: "UNKNOWN"}
	}
	saved := before.WallMS - after.WallMS
	return PerformanceAssessment{State: Closed, Reason: "EXACT_BEFORE_AFTER_PAIR_BOUND", Before: &before, After: &after, SavedTimeMS: saved, Improvement: saved}
}

func buildMetrics(decisions []TestDecision, oracle FullOracle, measurements RuntimeMeasurements, pair *PerformancePair, current Bindings, unknowns []UnknownDetail) Metrics {
	metrics := Metrics{TotalTests: len(decisions), FullOracleExecuted: len(oracle.Results), BuildMS: measurements.BuildMS, TestMS: measurements.TestMS, ConformanceMS: measurements.ConformanceMS, Unknowns: len(unknowns)}
	for _, decision := range decisions {
		switch decision.Action {
		case "EXECUTE":
			metrics.Selected++
			metrics.Executed++
		case "REUSE":
			metrics.Reused++
		}
	}
	for _, result := range oracle.Results {
		if result.Status == "FAIL" {
			metrics.Failures++
		}
	}
	metrics.AvoidedExecutions = metrics.TotalTests - metrics.Executed
	if pair != nil {
		performance := assessPerformance(pair, current)
		if performance.Before != nil && performance.After != nil {
			metrics.BeforeWallMS = performance.Before.WallMS
			metrics.AfterWallMS = performance.After.WallMS
			metrics.BeforePeakRSSKiB = performance.Before.PeakRSSKiB
			metrics.AfterPeakRSSKiB = performance.After.PeakRSSKiB
		}
	}
	return metrics
}

func buildActivityDecisions(cells []ContractCell, decision Decision, reason string, unknowns []UnknownDetail, refutations []string, fullFallback bool) []ActivityDecision {
	result := make([]ActivityDecision, 0, len(cells))
	for _, cell := range cells {
		state := Closed
		cellReason := "ACTIVITY_CLOSED"
		var unknown *UnknownDetail
		if decision == Refuted {
			state = Refuted
			cellReason = reason
		} else if decision == Unknown {
			state = Unknown
			cellReason = reason
			if len(unknowns) > 0 {
				copyValue := unknowns[0]
				unknown = &copyValue
			}
		}
		if fullFallback && cell.Activity == "DetectUnknownImpactEdges" && len(unknowns) > 0 {
			state = Unknown
			cellReason = "UNKNOWN_IMPACT_EDGE"
			copyValue := unknowns[0]
			unknown = &copyValue
		}
		if len(refutations) > 0 && cell.Activity == "CompareSelectiveWithFullOracle" {
			state = Refuted
			cellReason = reason
		}
		result = append(result, ActivityDecision{Ordinal: cell.Ordinal, ID: cell.ID, Activity: cell.Activity, Stage: cell.Stage, Step: cell.Step, ProofChoice: cell.ProofChoice, IndicatorClass: cell.IndicatorClass, State: state, Reason: cellReason, Unknown: unknown})
	}
	return result
}

func aggregateDecision(refutations []string, unknowns []UnknownDetail, comparison OracleComparison, fullFallback bool) (Decision, string) {
	if len(refutations) > 0 || comparison.State == Refuted {
		for _, reason := range refutations {
			if strings.HasPrefix(reason, "SELECTIVE_FULL_ORACLE_MISMATCH") {
				return Refuted, "SELECTIVE_FULL_ORACLE_MISMATCH"
			}
		}
		if len(refutations) > 0 {
			return Refuted, refutations[0]
		}
		return Refuted, comparison.Reason
	}
	if len(unknowns) > 0 {
		if fullFallback {
			return Unknown, "UNKNOWN_IMPACT_EDGE_FULL_VERIFICATION"
		}
		return Unknown, unknowns[0].Reason
	}
	return Closed, "CAUSAL_VERIFICATION_AND_FULL_ORACLE_AGREE"
}

func unknownDetail(stage, step, reason, class, next string, blocked []string) UnknownDetail {
	sort.Strings(blocked)
	return UnknownDetail{Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: blocked}
}

func resolveProof(value ReusableProof, current Bindings) ReusableProof {
	value.SourceDigest = resolveToken(value.SourceDigest, current)
	value.SourceTreeDigest = resolveToken(value.SourceTreeDigest, current)
	value.SemanticDigest = resolveToken(value.SemanticDigest, current)
	value.GraphDigest = resolveToken(value.GraphDigest, current)
	value.ToolchainDigest = resolveToken(value.ToolchainDigest, current)
	value.ScenarioDigest = resolveToken(value.ScenarioDigest, current)
	value.TestInventoryDigest = resolveToken(value.TestInventoryDigest, current)
	value.CommandDigest = resolveToken(value.CommandDigest, current)
	return value
}

func resolveToken(value string, current Bindings) string {
	switch value {
	case "$SOURCE_DIGEST":
		return current.SourceDigest
	case "$SOURCE_TREE_DIGEST":
		return current.SourceTreeDigest
	case "$SEMANTIC_DIGEST":
		return current.SemanticDigest
	case "$GRAPH_DIGEST":
		return current.GraphDigest
	case "$TOOLCHAIN_DIGEST":
		return current.ToolchainDigest
	case "$SCENARIO_DIGEST":
		return current.ScenarioDigest
	case "$TEST_INVENTORY_DIGEST":
		return current.TestInventoryDigest
	case "$COMMAND_DIGEST":
		return current.CommandDigest
	case "$ORACLE_DIGEST", "$FULL_ORACLE_DIGEST":
		return current.OracleDigest
	default:
		return value
	}
}

func edgeIDs(edges []GraphEdge) []string {
	result := make([]string, 0, len(edges))
	for _, edge := range edges {
		result = append(result, edge.EdgeID)
	}
	return result
}

func sortedEdges(edges map[string]GraphEdge) []GraphEdge {
	result := make([]GraphEdge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, edge)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].EdgeID < result[j].EdgeID })
	return result
}

func affectedPredicates(nodes map[string]bool) []string {
	result := make([]string, 0)
	for node := range nodes {
		if strings.HasPrefix(node, "predicate:") {
			result = append(result, node)
		}
	}
	sort.Strings(result)
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
