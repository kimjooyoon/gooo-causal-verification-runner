package runner

import (
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func decodeJSON(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

func RenderEvaluator(ir SemanticIR) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("package generated\n\n")
	fmt.Fprintf(&builder, "const Protocol = %s\n", strconv.Quote(ir.Protocol))
	fmt.Fprintf(&builder, "const SourceDigest = %s\n", strconv.Quote(ir.SourceDigest))
	fmt.Fprintf(&builder, "const SemanticDigest = %s\n\n", strconv.Quote(ir.Digest))
	builder.WriteString("type Activity struct {\n\tOrdinal int\n\tName string\n\tStage string\n\tStep string\n}\n\n")
	builder.WriteString("var Activities = []Activity{\n")
	for _, activity := range ir.Activities {
		fmt.Fprintf(&builder, "\t{Ordinal: %d, Name: %s, Stage: %s, Step: %s},\n", activity.Ordinal, strconv.Quote(activity.Name), strconv.Quote(metadataValue(activity.Computes, "stage")), strconv.Quote(metadataValue(activity.Computes, "step")))
	}
	builder.WriteString("}\n\nfunc ActivityCount() int { return len(Activities) }\n")
	return format.Source([]byte(builder.String()))
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func WriteText(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o644)
}

func RenderHumanReport(plan Plan) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Causal verification runner report\n\n")
	fmt.Fprintf(&builder, "decision: `%s`\n", plan.Decision)
	fmt.Fprintf(&builder, "selection_mode: `%s`\n", plan.SelectionMode)
	fmt.Fprintf(&builder, "reason: `%s`\n\n", plan.DecisionReason)
	fmt.Fprintf(&builder, "tests: total=%d selected=%d executed=%d reused=%d full_oracle_executed=%d failures=%d unknowns=%d avoided_executions=%d\n", plan.Metrics.TotalTests, plan.Metrics.Selected, plan.Metrics.Executed, plan.Metrics.Reused, plan.Metrics.FullOracleExecuted, plan.Metrics.Failures, plan.Metrics.Unknowns, plan.Metrics.AvoidedExecutions)
	fmt.Fprintf(&builder, "timing: before_wall_ms=%d after_wall_ms=%d before_peak_rss_kib=%d after_peak_rss_kib=%d build_ms=%d test_ms=%d conformance_ms=%d\n", plan.Metrics.BeforeWallMS, plan.Metrics.AfterWallMS, plan.Metrics.BeforePeakRSSKiB, plan.Metrics.AfterPeakRSSKiB, plan.Metrics.BuildMS, plan.Metrics.TestMS, plan.Metrics.ConformanceMS)
	fmt.Fprintf(&builder, "performance: saved_time_ms=%v improvement=%v state=%s\n\n", plan.Performance.SavedTimeMS, plan.Performance.Improvement, plan.Performance.State)
	builder.WriteString("## Causal chain\n\n")
	fmt.Fprintf(&builder, "change claim: %s\n", plan.ChangeClaim.ClaimID)
	fmt.Fprintf(&builder, "affected semantic predicates: %s\n", strings.Join(plan.AffectedSemanticPredicates, ", "))
	fmt.Fprintf(&builder, "required tests: %s\n", strings.Join(plan.RequiredTests, ", "))
	builder.WriteString("\n## Activities\n\n")
	for _, activity := range plan.Activities {
		fmt.Fprintf(&builder, "- `%s` %s / %s / %s\n", activity.ID, activity.State, activity.Stage, activity.Reason)
	}
	builder.WriteString("\n## Tests\n\n")
	for _, test := range plan.Tests {
		fmt.Fprintf(&builder, "- `%s`: %s affected=%t path=%s reason=%s\n", test.TestID, test.Action, test.Affected, strings.Join(test.Path, " -> "), test.Reason)
	}
	fmt.Fprintf(&builder, "\nfull oracle: state=%s independent=%t mismatches=%d\n", plan.FullOracleComparison.State, plan.FullOracleComparison.Independent, len(plan.FullOracleComparison.Mismatches))
	if len(plan.Unknowns) > 0 {
		builder.WriteString("\n## UNKNOWN causal frontiers\n\n")
		for _, unknown := range plan.Unknowns {
			fmt.Fprintf(&builder, "- stage=%s step=%s reason=%s unknown_class=%s next_operation=%s blocked_by=%s\n", unknown.Stage, unknown.Step, unknown.Reason, unknown.UnknownClass, unknown.NextOperation, strings.Join(unknown.BlockedBy, ","))
		}
	}
	if len(plan.Refutations) > 0 {
		builder.WriteString("\n## REFUTATIONS\n\n")
		for _, refutation := range plan.Refutations {
			fmt.Fprintf(&builder, "- %s\n", refutation)
		}
	}
	builder.WriteString("\nNo changed-file glob, cache hit, or historical success is proof. Reused tests carry no current execution metrics.\n")
	return builder.String()
}

func BuildManifest(root string, receipt Receipt, inventory Inventory) (Manifest, error) {
	paths := []string{
		"semantic-ir.json", "generated/evaluator.go", "plan.json", "verification-receipt.json", "human-report.md",
	}
	files := make([]ArtifactFile, 0, len(paths))
	for _, path := range paths {
		digest, data, err := DigestFile(filepath.Join(root, path))
		if err != nil {
			return Manifest{}, err
		}
		files = append(files, ArtifactFile{Path: path, Digest: digest, SizeBytes: int64(len(data))})
	}
	return Manifest{
		Schema: "gooo/causal-verification-runner/manifest/v1", CaseID: receipt.CaseID,
		Files: files, ArtifactFiles: len(files), Inventory: inventory,
		RepositoryWrites: receipt.Authority.RepositoryWrites, LocalTestExecutions: receipt.Authority.LocalTestExecutions,
	}, nil
}
