package runner

import (
	"fmt"
	"sort"
	"strconv"
)

const ProcessAuthoritySchema = "gooo/causal-verification-runner/process-authority/v1"

func VerifyProcessAuthority(contract ProcessAuthorityContract, evidence GitHubProcessEvidence) (ProcessGuardResult, error) {
	if err := validateProcessContract(contract); err != nil {
		return ProcessGuardResult{}, err
	}
	if evidence.Schema != "gooo/causal-verification-runner/github-process-evidence/v1" {
		return ProcessGuardResult{}, fmt.Errorf("malformed GitHub process evidence schema")
	}
	if evidence.Repository != processFieldOne(contract, "repository") {
		return ProcessGuardResult{}, fmt.Errorf("GitHub process evidence repository does not match .gooo contract")
	}
	if evidence.Phase != "pull_request" && evidence.Phase != "main" && evidence.Phase != "release" {
		return ProcessGuardResult{}, fmt.Errorf("unsupported process evidence phase %q", evidence.Phase)
	}

	result := ProcessGuardResult{
		Schema: ProcessAuthoritySchema, Repository: evidence.Repository, Phase: evidence.Phase,
		Decision: Refuted, CurrentGuardDecision: Closed,
		Precedence: []Decision{Refuted, Unknown, Closed},
		Cells:      make([]ProcessCellResult, 0, len(contract.Cells)), Cases: make([]ProcessCaseResult, 0, len(contract.Cases)),
		Unknowns: []UnknownDetail{}, Refutations: []string{}, HistoricalCounterexamples: []string{},
		UtilityGlobalCore: map[string]string{"state": processFieldOne(contract, "utility_global_core_state"), "status": processFieldOne(contract, "utility_global_core_status")},
	}

	commits := make(map[string]GitHubCommitEvidence, len(evidence.MainCommits))
	commitIndexes := make(map[string]int, len(evidence.MainCommits))
	for index, commit := range evidence.MainCommits {
		if commit.SHA == "" {
			return ProcessGuardResult{}, fmt.Errorf("malformed main commit evidence")
		}
		if _, exists := commits[commit.SHA]; exists {
			return ProcessGuardResult{}, fmt.Errorf("duplicate main commit evidence %q", commit.SHA)
		}
		commits[commit.SHA] = commit
		commitIndexes[commit.SHA] = index + 1
	}
	prs := make(map[int]GitHubPullRequestEvidence, len(evidence.PullRequests))
	for _, pr := range evidence.PullRequests {
		if pr.Number <= 0 || pr.URL == "" || prs[pr.Number].Number != 0 {
			return ProcessGuardResult{}, fmt.Errorf("duplicate or malformed pull request evidence %d", pr.Number)
		}
		prs[pr.Number] = pr
	}
	tags := make(map[string]GitHubTagEvidence, len(evidence.Tags))
	for _, tag := range evidence.Tags {
		if tag.Name == "" || tags[tag.Name].Name != "" {
			return ProcessGuardResult{}, fmt.Errorf("duplicate or malformed tag evidence %q", tag.Name)
		}
		tags[tag.Name] = tag
	}
	releases := make(map[string]GitHubReleaseEvidence, len(evidence.Releases))
	for _, release := range evidence.Releases {
		if release.TagName == "" || releases[release.TagName].TagName != "" {
			return ProcessGuardResult{}, fmt.Errorf("duplicate or malformed release evidence %q", release.TagName)
		}
		releases[release.TagName] = release
	}

	guardRefutations := make([]string, 0)
	guardUnknowns := make([]UnknownDetail, 0)
	addGuardRefutation := func(reason string) {
		guardRefutations = append(guardRefutations, reason)
		result.Refutations = append(result.Refutations, reason)
	}
	addGuardUnknown := func(detail UnknownDetail) {
		guardUnknowns = append(guardUnknowns, detail)
		result.Unknowns = append(result.Unknowns, detail)
	}
	addNonGuardUnknown := func(detail UnknownDetail) {
		result.Unknowns = append(result.Unknowns, detail)
	}

	rootSHA := processFieldOne(contract, "bootstrap_root")
	rootIndex, rootObserved := commitIndexes[rootSHA]
	if !rootObserved {
		addGuardUnknown(processUnknown("BOOTSTRAP_WINDOW", "locate-bootstrap-root", "BOOTSTRAP_ROOT_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_MAIN_COMMIT_HISTORY", []string{"commit:" + rootSHA}))
	} else if rootCommit := commits[rootSHA]; len(rootCommit.PullRequestNumbers) != 0 {
		addGuardRefutation("BOOTSTRAP_ROOT_HAS_PULL_REQUEST")
	} else {
		result.Counts.BootstrapDirectMain = 1
	}
	if result.Counts.BootstrapDirectMain != parseProcessInt(contract, "bootstrap_direct_main") {
		addGuardRefutation("BOOTSTRAP_DIRECT_MAIN_COUNT_MISMATCH")
	}

	historicalDirect := 0
	for sha, index := range commitIndexes {
		if rootObserved && index < rootIndex && len(commits[sha].PullRequestNumbers) == 0 {
			historicalDirect++
		}
	}
	result.Counts.HistoricalPostBootstrapDirect = historicalDirect
	if historicalDirect != parseProcessInt(contract, "historical_post_bootstrap_direct_main") {
		addGuardRefutation("HISTORICAL_POST_BOOTSTRAP_DIRECT_MAIN_COUNT_MISMATCH")
	}

	for _, rule := range contract.Cases {
		caseResult := ProcessCaseResult{ID: rule.ID, Evidence: rule.Evidence, Expected: rule.Expected, State: Unknown, Reason: "PROCESS_CASE_EVIDENCE_NOT_OBSERVED"}
		switch rule.Expected {
		case Refuted:
			commit, ok := commits[rule.Evidence]
			if !ok {
				detail := processUnknown("HISTORICAL_VIOLATIONS", "bind-known-violation", "HISTORICAL_COMMIT_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_MAIN_COMMIT_HISTORY", []string{"commit:" + rule.Evidence})
				result.Unknowns = append(result.Unknowns, detail)
				caseResult.Reason = detail.Reason
				break
			}
			if commitIndexes[rule.Evidence] >= rootIndex || len(commit.PullRequestNumbers) != 0 {
				caseResult.State = Refuted
				caseResult.Reason = "HISTORICAL_DIRECT_MAIN_WITHOUT_PULL_REQUEST"
				caseResult.Counterexample = true
				result.HistoricalCounterexamples = append(result.HistoricalCounterexamples, rule.ID)
				result.Refutations = append(result.Refutations, "HISTORICAL_DIRECT_MAIN_REFUTED:"+rule.Evidence)
				break
			}
			caseResult.State = Refuted
			caseResult.Reason = "HISTORICAL_DIRECT_MAIN_WITHOUT_PULL_REQUEST"
			caseResult.Counterexample = true
			result.HistoricalCounterexamples = append(result.HistoricalCounterexamples, rule.ID)
			result.Refutations = append(result.Refutations, "HISTORICAL_DIRECT_MAIN_REFUTED:"+rule.Evidence)
		case Closed:
			prNumber, err := strconv.Atoi(rule.Evidence)
			if err != nil {
				return ProcessGuardResult{}, fmt.Errorf("process case %q has non-numeric current guard evidence", rule.ID)
			}
			_, ok := prs[prNumber]
			if !ok {
				detail := processUnknown("PR_MERGE_ONLY", "bind-source-pull-request", "CURRENT_GUARD_PR_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_PULL_REQUEST_API_EVIDENCE", []string{fmt.Sprintf("pull_request:%d", prNumber)})
				result.Unknowns = append(result.Unknowns, detail)
				caseResult.Reason = detail.Reason
				break
			}
			caseResult.State = Closed
			caseResult.Reason = "CURRENT_PROCESS_GUARD_BOUND_TO_PULL_REQUEST"
		default:
			return ProcessGuardResult{}, fmt.Errorf("process case %q has unsupported expected state %q", rule.ID, rule.Expected)
		}
		result.Cases = append(result.Cases, caseResult)
	}

	prOneOK := verifyHistoricalPR1(contract, prs, commits, addGuardRefutation, addGuardUnknown)
	prTwoOK := verifyGuardPR(contract, evidence.Phase, prs, commits, commitIndexes, addGuardRefutation, addGuardUnknown)
	if !prOneOK || !prTwoOK {
		result.CurrentGuardDecision = Refuted
	} else if len(guardUnknowns) > 0 {
		result.CurrentGuardDecision = Unknown
	}

	v010OK := verifyHistoricalRelease(contract, tags, releases, addGuardRefutation, addGuardUnknown)
	durableReleaseState := verifyDurableRelease(contract, evidence.Phase, tags, releases, prs, commits, addGuardRefutation, addNonGuardUnknown)

	postGuard := 0
	guardPR := prs[parseProcessInt(contract, "guard_pr_number")]
	if guardPR.Merged && guardPR.MergeCommitSHA != "" {
		if guardIndex, ok := commitIndexes[guardPR.MergeCommitSHA]; ok {
			for sha, index := range commitIndexes {
				if index < guardIndex && len(commits[sha].PullRequestNumbers) == 0 {
					postGuard++
				}
			}
		}
	}
	result.Counts.PostGuardDirectMain = postGuard
	if evidence.Phase != "pull_request" && postGuard != parseProcessInt(contract, "post_guard_direct_main") {
		addGuardRefutation("POST_GUARD_DIRECT_MAIN_COUNT_MISMATCH")
	}

	for _, cell := range contract.Cells {
		cellResult := ProcessCellResult{ID: cell.ID, Rule: cell.Rule, Expected: cell.Expected, State: Closed, Reason: "PROCESS_GUARD_CELL_CLOSED"}
		switch cell.ID {
		case "BOOTSTRAP_WINDOW":
			if !rootObserved || result.Counts.BootstrapDirectMain != parseProcessInt(contract, "bootstrap_direct_main") {
				cellResult.State, cellResult.Reason = Unknown, "BOOTSTRAP_WINDOW_EVIDENCE_INCOMPLETE"
			}
		case "HISTORICAL_VIOLATIONS":
			cellResult.State, cellResult.Reason = Refuted, "HISTORICAL_REFUTED_COUNTEREXAMPLES_PRESERVED"
		case "PR_MERGE_ONLY", "PARENT_SOURCE_PR":
			if !prOneOK || !prTwoOK {
				cellResult.State, cellResult.Reason = Refuted, "PR_MERGE_ONLY_EVIDENCE_REFUTED"
			} else if len(guardUnknowns) > 0 {
				cellResult.State, cellResult.Reason = Unknown, "PR_MERGE_ONLY_EVIDENCE_INCOMPLETE"
			}
		case "POST_GUARD_DIRECT_MAIN":
			if evidence.Phase != "pull_request" && result.Counts.PostGuardDirectMain != parseProcessInt(contract, "post_guard_direct_main") {
				cellResult.State, cellResult.Reason = Refuted, "POST_GUARD_DIRECT_MAIN_NOT_ZERO"
			}
		case "TAG_POLICY", "RELEASE_POLICY":
			if !v010OK {
				cellResult.State, cellResult.Reason = Refuted, "HISTORICAL_RELEASE_POLICY_REFUTED"
			}
		case "ASSET_IMMUTABILITY":
			cellResult.State, cellResult.Reason = durableReleaseState, "DURABLE_RELEASE_ASSET_POLICY_AUDITED"
		}
		if cellResult.State != cell.Expected && cell.ID != "ASSET_IMMUTABILITY" {
			result.Refutations = append(result.Refutations, "PROCESS_CELL_STATE_MISMATCH:"+cell.ID)
		}
		result.Cells = append(result.Cells, cellResult)
	}

	if !v010OK {
		result.CurrentGuardDecision = Refuted
	}
	if len(guardRefutations) > 0 {
		result.CurrentGuardDecision = Refuted
	} else if len(guardUnknowns) > 0 {
		result.CurrentGuardDecision = Unknown
	}
	if len(result.Refutations) > 0 {
		result.Decision = Refuted
	} else if len(result.Unknowns) > 0 {
		result.Decision = Unknown
	} else {
		result.Decision = Closed
	}
	sort.Strings(result.HistoricalCounterexamples)
	result.Refutations = uniqueStrings(result.Refutations)
	return result, nil
}

func validateProcessContract(contract ProcessAuthorityContract) error {
	if contract.Name != "ProcessAuthorityContract" || processFieldOne(contract, "schema") != ProcessAuthoritySchema {
		return fmt.Errorf(".gooo process authority contract is missing or has the wrong schema")
	}
	if processFieldOne(contract, "repository") == "" || processFieldOne(contract, "bootstrap_branch") != "main" || processFieldOne(contract, "transition") != "PR_MERGE_ONLY" || processFieldOne(contract, "release_policy") != "TAG_RELEASE_NO_DELETE_NO_REWRITE" {
		return fmt.Errorf(".gooo process authority contract has invalid repository or transition policy")
	}
	for _, key := range []string{"bootstrap_root", "bootstrap_direct_main", "historical_post_bootstrap_direct_main", "guard_branch", "guard_pr_number", "guard_pr_url", "guard_base_parent", "post_guard_direct_main", "durable_release_tag", "utility_global_core_state", "utility_global_core_status"} {
		if processFieldOne(contract, key) == "" {
			return fmt.Errorf(".gooo process authority contract is missing field %q", key)
		}
	}
	if processFieldOne(contract, "utility_global_core_state") != string(Unknown) || processFieldOne(contract, "utility_global_core_status") != "NOT_MADE" {
		return fmt.Errorf("utility/global core must remain UNKNOWN/NOT_MADE")
	}
	if len(contract.Cells) != 8 || len(contract.Cases) != 3 || len(contract.PullRequests) != 1 || len(contract.Releases) != 1 || len(contract.Assets) != 3 {
		return fmt.Errorf("process authority contract denominator is incomplete")
	}
	if parseProcessInt(contract, "bootstrap_direct_main") != 1 || parseProcessInt(contract, "historical_post_bootstrap_direct_main") != 2 || parseProcessInt(contract, "post_guard_direct_main") != 0 {
		return fmt.Errorf("process authority direct-main denominator is invalid")
	}
	seenCells := map[string]bool{}
	for _, cell := range contract.Cells {
		if cell.ID == "" || cell.Rule == "" || seenCells[cell.ID] || (cell.Expected != Closed && cell.Expected != Refuted && cell.Expected != Unknown) {
			return fmt.Errorf("malformed process authority cell %q", cell.ID)
		}
		seenCells[cell.ID] = true
	}
	seenCases := map[string]bool{}
	for _, item := range contract.Cases {
		if item.ID == "" || item.Evidence == "" || seenCases[item.ID] || (item.Expected != Closed && item.Expected != Refuted && item.Expected != Unknown) {
			return fmt.Errorf("malformed process authority case %q", item.ID)
		}
		seenCases[item.ID] = true
	}
	pr := contract.PullRequests[0]
	if pr.Number != 1 || pr.URL == "" || pr.BaseRef != "main" || pr.HeadRef != "implementation" || pr.MergeCommit != "8064430a4cca385ccb79b94a5a4255ac90949694" || !equalStrings(pr.MergeParents, []string{"dfdb08cc18f6c9363090c723abf6f6ef82cedc5b", "d128d384df8e37ebfab513bfd805ae81651e7bdc"}) {
		return fmt.Errorf("PR #1 exact parent/source rule is invalid")
	}
	for _, release := range contract.Releases {
		if release.Tag == "" || release.ReleaseID <= 0 || release.TagObject == "" || release.TargetCommit == "" || !release.Immutable {
			return fmt.Errorf("malformed immutable release rule %q", release.Tag)
		}
	}
	for _, asset := range contract.Assets {
		if asset.Tag == "" || asset.AssetID <= 0 || asset.Name == "" || asset.SizeBytes < 0 || MustDigest(asset.Digest) != nil {
			return fmt.Errorf("malformed immutable release asset rule %q", asset.Name)
		}
	}
	return nil
}

func verifyHistoricalPR1(contract ProcessAuthorityContract, prs map[int]GitHubPullRequestEvidence, commits map[string]GitHubCommitEvidence, refute func(string), unknown func(UnknownDetail)) bool {
	rule := contract.PullRequests[0]
	pr, ok := prs[rule.Number]
	if !ok {
		unknown(processUnknown("PR_MERGE_ONLY", "bind-historical-source-pr", "PR_1_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_PULL_REQUEST_API_EVIDENCE", []string{"pull_request:1"}))
		return false
	}
	merge, ok := commits[rule.MergeCommit]
	if !ok {
		unknown(processUnknown("PR_MERGE_ONLY", "bind-historical-merge-parents", "PR_1_MERGE_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_MAIN_COMMIT_HISTORY", []string{"commit:" + rule.MergeCommit}))
		return false
	}
	ok = pr.URL == rule.URL && pr.State == "closed" && pr.Merged && pr.BaseRef == rule.BaseRef && pr.HeadRef == rule.HeadRef && pr.HeadSHA == rule.MergeParents[1] && pr.MergeCommitSHA == rule.MergeCommit && equalStrings(merge.Parents, rule.MergeParents) && containsInt(pr.Number, merge.PullRequestNumbers)
	if !ok {
		refute("PR_1_EXACT_PARENT_SOURCE_EVIDENCE_REFUTED")
	}
	return ok
}

func verifyGuardPR(contract ProcessAuthorityContract, phase string, prs map[int]GitHubPullRequestEvidence, commits map[string]GitHubCommitEvidence, commitIndexes map[string]int, refute func(string), unknown func(UnknownDetail)) bool {
	number := parseProcessInt(contract, "guard_pr_number")
	pr, ok := prs[number]
	if !ok {
		unknown(processUnknown("PR_MERGE_ONLY", "bind-current-source-pr", "CURRENT_GUARD_PR_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_PULL_REQUEST_API_EVIDENCE", []string{fmt.Sprintf("pull_request:%d", number)}))
		return false
	}
	if pr.URL != processFieldOne(contract, "guard_pr_url") || pr.BaseRef != "main" || pr.HeadRef != processFieldOne(contract, "guard_branch") {
		refute("PR_2_BASE_SOURCE_EVIDENCE_REFUTED")
		return false
	}
	if phase == "pull_request" {
		if pr.Merged || pr.State != "open" || pr.HeadSHA == "" {
			refute("PR_2_OPEN_SOURCE_EVIDENCE_REFUTED")
			return false
		}
		return true
	}
	if !pr.Merged || pr.State != "closed" || pr.MergeCommitSHA == "" {
		refute("PR_2_MERGE_EVIDENCE_NOT_OBSERVED")
		return false
	}
	merge, ok := commits[pr.MergeCommitSHA]
	if !ok {
		unknown(processUnknown("PR_MERGE_ONLY", "bind-current-merge-parents", "PR_2_MERGE_NOT_OBSERVED_ON_MAIN", "DIRECT_MISSING", "OBTAIN_MAIN_COMMIT_HISTORY", []string{"commit:" + pr.MergeCommitSHA}))
		return false
	}
	baseParent := processFieldOne(contract, "guard_base_parent")
	if !containsInt(number, merge.PullRequestNumbers) || len(merge.Parents) != 2 || merge.Parents[0] != baseParent || merge.Parents[1] != pr.HeadSHA || commitIndexes[pr.MergeCommitSHA] == 0 {
		refute("PR_2_EXACT_PARENT_SOURCE_EVIDENCE_REFUTED")
		return false
	}
	return true
}

func verifyHistoricalRelease(contract ProcessAuthorityContract, tags map[string]GitHubTagEvidence, releases map[string]GitHubReleaseEvidence, refute func(string), unknown func(UnknownDetail)) bool {
	rule := contract.Releases[0]
	tag, tagOK := tags[rule.Tag]
	release, releaseOK := releases[rule.Tag]
	if !tagOK {
		unknown(processUnknown("RELEASE_POLICY", "bind-v0-1-0-tag", "V010_TAG_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_TAG_API_EVIDENCE", []string{"tag:" + rule.Tag}))
		return false
	}
	if !releaseOK {
		unknown(processUnknown("RELEASE_POLICY", "bind-v0-1-0-release", "V010_RELEASE_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_RELEASE_API_EVIDENCE", []string{"release:" + rule.Tag}))
		return false
	}
	ok := tag.Name == rule.Tag && tag.ObjectSHA == rule.TagObject && tag.ObjectType == "tag" && tag.TargetSHA == rule.TargetCommit && tag.TargetType == "commit" && release.ID == rule.ReleaseID && release.TagName == rule.Tag && release.Immutable == rule.Immutable && !release.Draft && !release.Prerelease
	assets := make(map[string]GitHubAssetEvidence, len(release.Assets))
	for _, asset := range release.Assets {
		assets[asset.Name] = asset
	}
	for _, expected := range contract.Assets {
		actual, exists := assets[expected.Name]
		if expected.Tag != rule.Tag || !exists || actual.ID != expected.AssetID || actual.Size != expected.SizeBytes || actual.Digest != expected.Digest {
			ok = false
		}
	}
	if !ok {
		refute("V010_IMMUTABLE_TAG_RELEASE_ASSET_EVIDENCE_REFUTED")
	}
	return ok
}

func verifyDurableRelease(contract ProcessAuthorityContract, phase string, tags map[string]GitHubTagEvidence, releases map[string]GitHubReleaseEvidence, prs map[int]GitHubPullRequestEvidence, commits map[string]GitHubCommitEvidence, refute func(string), unknown func(UnknownDetail)) Decision {
	tagName := processFieldOne(contract, "durable_release_tag")
	if phase != "release" {
		unknown(processUnknown("RELEASE_POLICY", "await-durable-release", "DURABLE_RELEASE_NOT_YET_OBSERVED", "DIRECT_MISSING", "CREATE_ANNOTATED_V0_1_1_RELEASE", []string{"release:" + tagName}))
		return Unknown
	}
	tag, tagOK := tags[tagName]
	release, releaseOK := releases[tagName]
	pr := prs[parseProcessInt(contract, "guard_pr_number")]
	if !tagOK || !releaseOK || pr.MergeCommitSHA == "" {
		unknown(processUnknown("RELEASE_POLICY", "bind-durable-release", "DURABLE_RELEASE_EVIDENCE_MISSING", "DIRECT_MISSING", "OBTAIN_TAG_RELEASE_API_EVIDENCE", []string{"release:" + tagName}))
		return Unknown
	}
	merge, mergeOK := commits[pr.MergeCommitSHA]
	if !mergeOK {
		unknown(processUnknown("RELEASE_POLICY", "bind-durable-release-target", "DURABLE_RELEASE_TARGET_NOT_OBSERVED", "DIRECT_MISSING", "OBTAIN_MAIN_COMMIT_HISTORY", []string{"commit:" + pr.MergeCommitSHA}))
		return Unknown
	}
	ok := tag.Name == tagName && tag.ObjectType == "tag" && tag.TargetType == "commit" && tag.TargetSHA == pr.MergeCommitSHA && release.TagName == tagName && release.ID > 0 && release.Immutable && !release.Draft && !release.Prerelease && len(merge.Parents) == 2
	names := map[string]bool{}
	ids := map[int64]bool{}
	for _, asset := range release.Assets {
		if asset.Name == "" || asset.Digest == "" || MustDigest(asset.Digest) != nil || names[asset.Name] || ids[asset.ID] {
			ok = false
		}
		names[asset.Name] = true
		ids[asset.ID] = true
	}
	for _, name := range []string{"gooo-causal-verification-runner-v0.1.1.tar.gz", "SHA256SUMS", "release-manifest.json"} {
		if !names[name] {
			ok = false
		}
	}
	if len(release.Assets) != 3 {
		ok = false
	}
	if !ok {
		refute("V011_DURABLE_RELEASE_TAG_RELEASE_ASSET_EVIDENCE_REFUTED")
		return Refuted
	}
	return Closed
}

func processFieldOne(contract ProcessAuthorityContract, key string) string {
	values := contract.Fields[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func parseProcessInt(contract ProcessAuthorityContract, key string) int {
	value, _ := strconv.Atoi(processFieldOne(contract, key))
	return value
}

func processUnknown(stage, step, reason, class, next string, blocked []string) UnknownDetail {
	return unknownDetail(stage, step, reason, class, next, blocked)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func containsInt(value int, values []int) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
