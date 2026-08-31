package runner

const (
	ProtocolSchema = "gooo/causal-verification-runner/protocol/v1"
	IRSchema       = "gooo/causal-verification-runner/semantic-ir/v1"
	PlanSchema     = "gooo/causal-verification-runner/plan/v1"
	ReceiptSchema  = "gooo/causal-verification-runner/receipt/v1"
	CorpusSchema   = "gooo/causal-verification-runner/corpus/v1"

	Closed  Decision = "CLOSED"
	Unknown Decision = "UNKNOWN"
	Refuted Decision = "REFUTED"
)

type Decision string

type Contract struct {
	Schema           string         `json:"schema"`
	DenominatorID    string         `json:"denominator_id"`
	FixedDenominator int            `json:"fixed_denominator"`
	Precedence       []Decision     `json:"precedence"`
	ProofTotals      map[string]int `json:"proof_totals"`
	IndicatorTotals  map[string]int `json:"indicator_totals"`
	Cells            []ContractCell `json:"cells"`
}

type ContractCell struct {
	Ordinal        int    `json:"ordinal"`
	ID             string `json:"id"`
	Activity       string `json:"activity"`
	Stage          string `json:"stage"`
	Step           string `json:"step"`
	ProofChoice    string `json:"proof_choice"`
	IndicatorClass string `json:"indicator_class"`
}

type SourceActivity struct {
	Ordinal    int      `json:"ordinal"`
	Name       string   `json:"name"`
	Inputs     []string `json:"inputs"`
	Output     string   `json:"output"`
	Computes   string   `json:"computes,omitempty"`
	SourceLine int      `json:"source_line"`
}

type SourceSpec struct {
	Package    string           `json:"package"`
	Namespace  string           `json:"namespace"`
	Activities []SourceActivity `json:"activities"`
}

type SemanticIR struct {
	Schema       string           `json:"schema"`
	Protocol     string           `json:"protocol"`
	SourcePath   string           `json:"source_path"`
	SourceDigest string           `json:"source_digest"`
	Activities   []SourceActivity `json:"activities"`
	Digest       string           `json:"digest"`
}

type ChangeClaim struct {
	ClaimID           string   `json:"claim_id"`
	ChangedPredicates []string `json:"changed_predicates"`
	SourceDigest      string   `json:"source_digest"`
	SemanticDigest    string   `json:"semantic_digest"`
}

type SemanticGraph struct {
	Schema         string      `json:"schema"`
	GraphID        string      `json:"graph_id"`
	SourceDigest   string      `json:"source_digest"`
	SemanticDigest string      `json:"semantic_digest"`
	GraphDigest    string      `json:"graph_digest"`
	Predicates     []string    `json:"predicates"`
	Edges          []GraphEdge `json:"edges"`
}

type GraphEdge struct {
	EdgeID string `json:"edge_id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

type TestSpec struct {
	TestID      string       `json:"test_id"`
	TargetNode  string       `json:"target_node"`
	Command     []string     `json:"command"`
	Observation *Observation `json:"observation,omitempty"`
}

type Observation struct {
	Status       string `json:"status"`
	WallMS       int64  `json:"wall_ms"`
	PeakRSSKiB   int64  `json:"peak_rss_kib"`
	ResultDigest string `json:"result_digest"`
}

type ReleaseEvidence struct {
	Provider          string `json:"provider"`
	Repository        string `json:"repository"`
	ReleaseID         int64  `json:"release_id"`
	Tag               string `json:"tag"`
	SelfAsserted      bool   `json:"self_asserted_immutable"`
	PlatformImmutable bool   `json:"platform_immutable"`
	AssetDigest       string `json:"asset_digest"`
	AssetName         string `json:"asset_name"`
}

type ReusableProof struct {
	ProofID             string          `json:"proof_id"`
	TestID              string          `json:"test_id"`
	SourceDigest        string          `json:"source_digest"`
	SourceTreeDigest    string          `json:"source_tree_digest"`
	SemanticDigest      string          `json:"semantic_digest"`
	GraphDigest         string          `json:"graph_digest"`
	ToolchainDigest     string          `json:"toolchain_digest"`
	ScenarioDigest      string          `json:"scenario_digest"`
	TestInventoryDigest string          `json:"test_inventory_digest"`
	CommandDigest       string          `json:"command_digest"`
	TerminalResult      string          `json:"terminal_result"`
	ResultDigest        string          `json:"result_digest"`
	Release             ReleaseEvidence `json:"release"`
}

type FullOracle struct {
	OracleID    string         `json:"oracle_id"`
	Independent bool           `json:"independent"`
	Digest      string         `json:"digest"`
	Results     []OracleResult `json:"results"`
}

type OracleResult struct {
	TestID       string `json:"test_id"`
	Status       string `json:"status"`
	ResultDigest string `json:"result_digest"`
}

type TimingSnapshot struct {
	SourceDigest    string `json:"source_digest"`
	ToolchainDigest string `json:"toolchain_digest"`
	ScenarioDigest  string `json:"scenario_digest"`
	WallMS          int64  `json:"wall_ms"`
	PeakRSSKiB      int64  `json:"peak_rss_kib"`
}

type PerformancePair struct {
	Before TimingSnapshot `json:"before"`
	After  TimingSnapshot `json:"after"`
}

type Case struct {
	Schema          string           `json:"schema"`
	CaseID          string           `json:"case_id"`
	Description     string           `json:"description"`
	ChangeClaim     ChangeClaim      `json:"change_claim"`
	SemanticGraph   SemanticGraph    `json:"semantic_graph"`
	Tests           []TestSpec       `json:"tests"`
	ReusableProofs  []ReusableProof  `json:"reusable_proofs"`
	FullOracle      FullOracle       `json:"full_oracle"`
	PerformancePair *PerformancePair `json:"performance_pair,omitempty"`
	Counterexamples []Counterexample `json:"counterexamples"`
	CacheHit        bool             `json:"cache_hit"`
	ScenarioDigest  string           `json:"scenario_digest,omitempty"`
	Expected        Expected         `json:"expected"`
}

type Counterexample struct {
	CounterexampleID     string `json:"counterexample_id"`
	TestID               string `json:"test_id"`
	ExpectedInvalidation bool   `json:"expected_invalidation"`
	ObservedInvalidation bool   `json:"observed_invalidation"`
}

type Expected struct {
	Decision             Decision `json:"decision"`
	SelectionMode        string   `json:"selection_mode"`
	TotalTests           int      `json:"total_tests"`
	Selected             int      `json:"selected"`
	Executed             int      `json:"executed"`
	Reused               int      `json:"reused"`
	FullOracleExecuted   int      `json:"full_oracle_executed"`
	Failures             int      `json:"failures"`
	Unknowns             int      `json:"unknowns"`
	AvoidedExecutions    int      `json:"avoided_executions"`
	InvalidatedEdgeCount int      `json:"invalidated_edge_count"`
}

type Corpus struct {
	Schema   string        `json:"schema"`
	CorpusID string        `json:"corpus_id"`
	Cases    []CorpusEntry `json:"cases"`
}

type CorpusEntry struct {
	Ordinal  int      `json:"ordinal"`
	CaseID   string   `json:"case_id"`
	Path     string   `json:"path"`
	Expected Decision `json:"expected"`
}

type UnknownDetail struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

type ActivityDecision struct {
	Ordinal        int            `json:"ordinal"`
	ID             string         `json:"id"`
	Activity       string         `json:"activity"`
	Stage          string         `json:"stage"`
	Step           string         `json:"step"`
	ProofChoice    string         `json:"proof_choice"`
	IndicatorClass string         `json:"indicator_class"`
	State          Decision       `json:"state"`
	Reason         string         `json:"reason"`
	Unknown        *UnknownDetail `json:"unknown,omitempty"`
}

type ProofDecision struct {
	ProofID string         `json:"proof_id"`
	TestID  string         `json:"test_id"`
	State   Decision       `json:"state"`
	Reason  string         `json:"reason"`
	Unknown *UnknownDetail `json:"unknown,omitempty"`
}

type TestDecision struct {
	TestID       string   `json:"test_id"`
	TargetNode   string   `json:"target_node"`
	Action       string   `json:"action"`
	Affected     bool     `json:"affected"`
	Path         []string `json:"path"`
	Reason       string   `json:"reason"`
	ProofID      string   `json:"proof_id,omitempty"`
	ResultDigest string   `json:"result_digest,omitempty"`
	WallMS       *int64   `json:"wall_ms,omitempty"`
	PeakRSSKiB   *int64   `json:"peak_rss_kib,omitempty"`
}

type OracleMismatch struct {
	TestID string `json:"test_id"`
	Reason string `json:"reason"`
}

type OracleComparison struct {
	State              Decision         `json:"state"`
	Reason             string           `json:"reason"`
	Independent        bool             `json:"independent"`
	OracleID           string           `json:"oracle_id"`
	OracleDigest       string           `json:"oracle_digest"`
	FullOracleExecuted int              `json:"full_oracle_executed"`
	Mismatches         []OracleMismatch `json:"mismatches"`
}

type Metrics struct {
	TotalTests         int   `json:"total_tests"`
	Selected           int   `json:"selected"`
	Executed           int   `json:"executed"`
	Reused             int   `json:"reused"`
	FullOracleExecuted int   `json:"full_oracle_executed"`
	Failures           int   `json:"failures"`
	Unknowns           int   `json:"unknowns"`
	BeforeWallMS       int64 `json:"before_wall_ms"`
	AfterWallMS        int64 `json:"after_wall_ms"`
	BeforePeakRSSKiB   int64 `json:"before_peak_rss_kib"`
	AfterPeakRSSKiB    int64 `json:"after_peak_rss_kib"`
	BuildMS            int64 `json:"build_ms"`
	TestMS             int64 `json:"test_ms"`
	ConformanceMS      int64 `json:"conformance_ms"`
	AvoidedExecutions  int   `json:"avoided_executions"`
}

type PerformanceAssessment struct {
	State       Decision        `json:"state"`
	Reason      string          `json:"reason"`
	Before      *TimingSnapshot `json:"before,omitempty"`
	After       *TimingSnapshot `json:"after,omitempty"`
	SavedTimeMS any             `json:"saved_time_ms"`
	Improvement any             `json:"improvement"`
}

type Authority struct {
	RepositoryWrites          int    `json:"repository_writes"`
	LocalTestExecutions       int    `json:"local_test_executions"`
	CrossProjectRequiredGates int    `json:"cross_project_required_gates"`
	OutputLocation            string `json:"output_location"`
	VerificationAuthority     string `json:"verification_authority"`
}

type Bindings struct {
	SourceDigest        string `json:"source_digest"`
	SourceTreeDigest    string `json:"source_tree_digest"`
	SemanticDigest      string `json:"semantic_digest"`
	GraphDigest         string `json:"graph_digest"`
	ToolchainDigest     string `json:"toolchain_digest"`
	ScenarioDigest      string `json:"scenario_digest"`
	TestInventoryDigest string `json:"test_inventory_digest"`
	CommandDigest       string `json:"command_digest"`
	OracleDigest        string `json:"oracle_digest"`
}

type Plan struct {
	Schema                     string                `json:"schema"`
	CaseID                     string                `json:"case_id"`
	Decision                   Decision              `json:"decision"`
	SelectionMode              string                `json:"selection_mode"`
	DecisionReason             string                `json:"decision_reason"`
	Bindings                   Bindings              `json:"bindings"`
	Activities                 []ActivityDecision    `json:"activities"`
	ChangeClaim                ChangeClaim           `json:"change_claim"`
	AffectedSemanticPredicates []string              `json:"affected_semantic_predicates"`
	RequiredTests              []string              `json:"required_tests"`
	Proofs                     []ProofDecision       `json:"reusable_proofs"`
	Tests                      []TestDecision        `json:"tests"`
	FullOracleComparison       OracleComparison      `json:"full_oracle_comparison"`
	Unknowns                   []UnknownDetail       `json:"unknowns"`
	Refutations                []string              `json:"refutations"`
	Metrics                    Metrics               `json:"metrics"`
	Performance                PerformanceAssessment `json:"performance"`
	InvalidationFrontier       []GraphEdge           `json:"invalidation_frontier"`
	Authority                  Authority             `json:"authority"`
}

type Receipt struct {
	Schema                     string                `json:"schema"`
	CaseID                     string                `json:"case_id"`
	Decision                   Decision              `json:"decision"`
	SelectionMode              string                `json:"selection_mode"`
	DecisionReason             string                `json:"decision_reason"`
	Bindings                   Bindings              `json:"bindings"`
	Source                     ArtifactBinding       `json:"source"`
	SemanticIR                 ArtifactBinding       `json:"semantic_ir"`
	GeneratedEvaluator         ArtifactBinding       `json:"generated_evaluator"`
	Contract                   ArtifactBinding       `json:"contract"`
	SemanticGraph              ArtifactBinding       `json:"semantic_graph"`
	FullOracle                 ArtifactBinding       `json:"full_oracle"`
	ScenarioDigest             string                `json:"scenario_digest"`
	Activities                 []ActivityDecision    `json:"activities"`
	AffectedSemanticPredicates []string              `json:"affected_semantic_predicates"`
	RequiredTests              []string              `json:"required_tests"`
	Tests                      []TestDecision        `json:"tests"`
	InvalidationFrontier       []GraphEdge           `json:"invalidation_frontier"`
	Metrics                    Metrics               `json:"metrics"`
	Unknowns                   []UnknownDetail       `json:"unknowns"`
	Refutations                []string              `json:"refutations"`
	FullOracleComparison       OracleComparison      `json:"full_oracle_comparison"`
	Performance                PerformanceAssessment `json:"performance"`
	Authority                  Authority             `json:"authority"`
	RootReadmeExcluded         bool                  `json:"root_readme_excluded"`
}

type ArtifactBinding struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type Inventory struct {
	RegularFiles       int64 `json:"regular_files"`
	TreeBytes          int64 `json:"tree_bytes"`
	GoFiles            int64 `json:"go_files"`
	GoLines            int64 `json:"go_lines"`
	GoooFiles          int64 `json:"gooo_files"`
	GoooLines          int64 `json:"gooo_lines"`
	RootReadmeExcluded bool  `json:"root_readme_excluded"`
}

type Manifest struct {
	Schema              string         `json:"schema"`
	CaseID              string         `json:"case_id"`
	Files               []ArtifactFile `json:"files"`
	ArtifactFiles       int            `json:"artifact_files"`
	Inventory           Inventory      `json:"inventory"`
	RepositoryWrites    int            `json:"repository_writes"`
	LocalTestExecutions int            `json:"local_test_executions"`
}

type ArtifactFile struct {
	Path      string `json:"path"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"size_bytes"`
}

type RuntimeMeasurements struct {
	BuildMS       int64 `json:"build_ms"`
	TestMS        int64 `json:"test_ms"`
	ConformanceMS int64 `json:"conformance_ms"`
}
