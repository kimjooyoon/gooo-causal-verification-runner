package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-causal-verification-runner/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: gooo-causal-verification-runner run|conformance [flags]")
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = run(os.Args[2:])
	case "conformance":
		err = conformance(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sourcePath := flags.String("source", "", "path to the .gooo meta-program")
	contractPath := flags.String("contract", "", "path to the fixed denominator contract")
	casePath := flags.String("case", "", "path to one caller-provided fixture case")
	outPath := flags.String("output-dir", "", "empty caller-owned output directory")
	treeRoot := flags.String("tree-root", ".", "input tree root for digest and inventory")
	buildMS := flags.Int64("build-ms", 0, "CI-observed build milliseconds")
	testMS := flags.Int64("test-ms", 0, "CI-observed test milliseconds")
	conformanceMS := flags.Int64("conformance-ms", 0, "CI-observed conformance milliseconds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *sourcePath == "" || *contractPath == "" || *casePath == "" || *outPath == "" {
		return fmt.Errorf("source, contract, case, and output-dir are required")
	}
	return evaluateOne(*sourcePath, *contractPath, *casePath, *outPath, *treeRoot, runner.RuntimeMeasurements{BuildMS: *buildMS, TestMS: *testMS, ConformanceMS: *conformanceMS})
}

func conformance(args []string) error {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sourcePath := flags.String("source", "examples/causal-verification/main.gooo", "path to the .gooo meta-program")
	contractPath := flags.String("contract", "contracts/causal-verification-denominator-v1.json", "path to the fixed denominator contract")
	corpusPath := flags.String("corpus", "fixtures/corpus.json", "path to the fixture corpus index")
	outPath := flags.String("output-dir", "", "empty caller-owned output directory")
	treeRoot := flags.String("tree-root", ".", "input tree root for digest and inventory")
	buildMS := flags.Int64("build-ms", 0, "CI-observed build milliseconds")
	testMS := flags.Int64("test-ms", 0, "CI-observed test milliseconds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *outPath == "" {
		return fmt.Errorf("output-dir is required")
	}
	absoluteTree, err := filepath.Abs(*treeRoot)
	if err != nil {
		return err
	}
	absoluteOut, err := filepath.Abs(*outPath)
	if err != nil {
		return err
	}
	if err := requireOutside(absoluteTree, absoluteOut); err != nil {
		return err
	}
	if err := prepareEmptyDirectory(absoluteOut); err != nil {
		return err
	}

	_, _, err = loadContract(*contractPath)
	if err != nil {
		return err
	}
	var corpus runner.Corpus
	if err := runner.LoadJSON(*corpusPath, &corpus); err != nil {
		return err
	}
	if corpus.Schema != runner.CorpusSchema || len(corpus.Cases) == 0 {
		return fmt.Errorf("malformed fixture corpus")
	}
	entries := append([]runner.CorpusEntry(nil), corpus.Cases...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Ordinal < entries[j].Ordinal })
	start := time.Now()
	caseSummaries := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		caseOut := filepath.Join(absoluteOut, entry.CaseID)
		if err := os.Mkdir(caseOut, 0o755); err != nil {
			return err
		}
		casePath := filepath.Join(filepath.Dir(*corpusPath), filepath.FromSlash(entry.Path))
		measurements := runner.RuntimeMeasurements{BuildMS: *buildMS, TestMS: *testMS, ConformanceMS: time.Since(start).Milliseconds()}
		plan, receipt, err := evaluateCaseFiles(*sourcePath, *contractPath, casePath, caseOut, absoluteTree, measurements)
		if err != nil {
			return err
		}
		if plan.CaseID != entry.CaseID || plan.Decision != entry.Expected {
			return fmt.Errorf("case %s decision %s does not match corpus expectation %s", entry.CaseID, plan.Decision, entry.Expected)
		}
		caseSummaries = append(caseSummaries, map[string]any{
			"case_id": plan.CaseID, "decision": plan.Decision, "selection_mode": plan.SelectionMode,
			"metrics": plan.Metrics, "unknowns": len(plan.Unknowns), "refutations": len(plan.Refutations),
			"receipt_digest": receiptDigest(receipt),
		})
	}
	summary := map[string]any{
		"schema": "gooo/causal-verification-runner/ci-summary/v1", "corpus_id": corpus.CorpusID,
		"cases": caseSummaries, "authority": map[string]any{"repository_writes": 0, "local_test_executions": 0, "cross_project_required_gates": 0, "verification_authority": "GITHUB_ACTIONS"},
		"root_readme_excluded": true,
	}
	return runner.WriteJSON(filepath.Join(absoluteOut, "ci-summary.json"), summary)
}

func evaluateOne(sourcePath, contractPath, casePath, outPath, treeRoot string, measurements runner.RuntimeMeasurements) error {
	absoluteTree, err := filepath.Abs(treeRoot)
	if err != nil {
		return err
	}
	absoluteOut, err := filepath.Abs(outPath)
	if err != nil {
		return err
	}
	if err := requireOutside(absoluteTree, absoluteOut); err != nil {
		return err
	}
	if err := prepareEmptyDirectory(absoluteOut); err != nil {
		return err
	}
	_, _, err = evaluateCaseFiles(sourcePath, contractPath, casePath, absoluteOut, absoluteTree, measurements)
	return err
}

func evaluateCaseFiles(sourcePath, contractPath, casePath, outPath, treeRoot string, measurements runner.RuntimeMeasurements) (runner.Plan, runner.Receipt, error) {
	source, sourceDigest, err := runner.ParseSource(sourcePath)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	contract, contractDigest, err := loadContract(contractPath)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	var value runner.Case
	if err := runner.LoadJSON(casePath, &value); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	treeDigest, inventory, err := runner.TreeDigest(treeRoot)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	ir, err := runner.BuildSemanticIR(sourcePath, source, sourceDigest)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	evaluator, err := runner.RenderEvaluator(ir)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	generatedPath := filepath.Join(outPath, "generated", "evaluator.go")
	if err := os.MkdirAll(filepath.Dir(generatedPath), 0o755); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	if err := runner.WriteJSON(filepath.Join(outPath, "semantic-ir.json"), ir); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	if err := runner.WriteText(generatedPath, string(evaluator)); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	generatedDigest, _, err := runner.DigestFile(generatedPath)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	plan, receipt, err := runner.EvaluateCase(value, source, sourceDigest, treeDigest, contractDigest, contract, ir, generatedDigest, inventory, measurements)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	if err := runner.WriteJSON(filepath.Join(outPath, "plan.json"), plan); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	if err := runner.WriteJSON(filepath.Join(outPath, "verification-receipt.json"), receipt); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	if err := runner.WriteText(filepath.Join(outPath, "human-report.md"), runner.RenderHumanReport(plan)); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	manifest, err := runner.BuildManifest(outPath, receipt, inventory)
	if err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	if err := runner.WriteJSON(filepath.Join(outPath, "manifest.json"), manifest); err != nil {
		return runner.Plan{}, runner.Receipt{}, err
	}
	return plan, receipt, nil
}

func loadContract(path string) (runner.Contract, string, error) {
	var contract runner.Contract
	digest, _, err := runner.DigestFile(path)
	if err != nil {
		return runner.Contract{}, "", err
	}
	if err := runner.LoadJSON(path, &contract); err != nil {
		return runner.Contract{}, "", err
	}
	return contract, digest, nil
}

func receiptDigest(receipt runner.Receipt) string {
	digest, _ := runner.DigestJSON(receipt)
	return digest
}

func requireOutside(root, output string) error {
	relative, err := filepath.Rel(root, output)
	if err != nil {
		return err
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return fmt.Errorf("output directory must be outside the input tree")
	}
	return nil
}

func prepareEmptyDirectory(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory must be empty")
	}
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
