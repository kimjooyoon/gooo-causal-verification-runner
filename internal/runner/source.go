package runner

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var activityPattern = regexp.MustCompile(`^activity\s+([A-Za-z_][A-Za-z0-9_]*)\(([^)]*)\)\s*->\s*([A-Za-z_][A-Za-z0-9_]*)(?:\s+computes\s+"([^"]*)")?\s*$`)

func ParseSource(path string) (SourceSpec, string, error) {
	digest, data, err := DigestFile(path)
	if err != nil {
		return SourceSpec{}, "", err
	}
	var spec SourceSpec
	seenPackage := false
	seenNamespace := false
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "package":
			if len(fields) != 2 || seenPackage {
				return SourceSpec{}, "", fmt.Errorf("source line %d: malformed package", lineNumber+1)
			}
			seenPackage = true
			spec.Package = fields[1]
		case "namespace":
			if len(fields) != 2 || seenNamespace {
				return SourceSpec{}, "", fmt.Errorf("source line %d: malformed namespace", lineNumber+1)
			}
			seenNamespace = true
			spec.Namespace = fields[1]
		case "entity":
			continue
		case "activity":
			match := activityPattern.FindStringSubmatch(line)
			if match == nil {
				return SourceSpec{}, "", fmt.Errorf("source line %d: malformed activity", lineNumber+1)
			}
			inputs := make([]string, 0)
			if strings.TrimSpace(match[2]) != "" {
				for _, input := range strings.Split(match[2], ",") {
					input = strings.TrimSpace(input)
					if input == "" {
						return SourceSpec{}, "", fmt.Errorf("source line %d: empty activity input", lineNumber+1)
					}
					inputs = append(inputs, input)
				}
			}
			spec.Activities = append(spec.Activities, SourceActivity{
				Ordinal: len(spec.Activities) + 1, Name: match[1], Inputs: inputs,
				Output: match[3], Computes: match[4], SourceLine: lineNumber + 1,
			})
		case "process_contract", "process_field", "process_cell", "process_case", "process_pr", "process_release", "process_asset":
			if err := parseProcessRecord(line, lineNumber+1, &spec.ProcessAuthority); err != nil {
				return SourceSpec{}, "", err
			}
		default:
			return SourceSpec{}, "", fmt.Errorf("source line %d: unknown record %q", lineNumber+1, fields[0])
		}
	}
	if spec.Package == "" || spec.Namespace == "" || len(spec.Activities) == 0 {
		return SourceSpec{}, "", fmt.Errorf("source must declare package, namespace, and activities")
	}
	seen := map[string]bool{}
	for _, activity := range spec.Activities {
		if seen[activity.Name] {
			return SourceSpec{}, "", fmt.Errorf("duplicate activity %q", activity.Name)
		}
		seen[activity.Name] = true
	}
	return spec, digest, nil
}

func BuildSemanticIR(sourcePath string, spec SourceSpec, sourceDigest string) (SemanticIR, error) {
	ir := SemanticIR{
		Schema: IRSchema, Protocol: ProtocolSchema, SourcePath: sourcePath,
		SourceDigest: sourceDigest, Activities: spec.Activities, ProcessAuthority: spec.ProcessAuthority,
	}
	copyValue, err := canonicalCopy(ir, func(item *SemanticIR) { item.Digest = "" })
	if err != nil {
		return SemanticIR{}, err
	}
	ir.Digest, err = DigestJSON(copyValue)
	if err != nil {
		return SemanticIR{}, err
	}
	return ir, nil
}

func parseProcessRecord(line string, lineNumber int, contract *ProcessAuthorityContract) error {
	fields, err := parseTokens(line)
	if err != nil {
		return fmt.Errorf("source line %d: %w", lineNumber, err)
	}
	if len(fields) == 0 {
		return fmt.Errorf("source line %d: empty process record", lineNumber)
	}
	if contract.Fields == nil {
		contract.Fields = map[string][]string{}
	}
	switch fields[0] {
	case "process_contract":
		if len(fields) != 2 || contract.Name != "" {
			return fmt.Errorf("source line %d: malformed process_contract", lineNumber)
		}
		contract.Name = fields[1]
	case "process_field":
		if len(fields) < 3 || contract.Fields[fields[1]] != nil {
			return fmt.Errorf("source line %d: malformed or duplicate process_field %q", lineNumber, fields[1])
		}
		contract.Fields[fields[1]] = append([]string(nil), fields[2:]...)
	case "process_cell":
		if len(fields) != 4 {
			return fmt.Errorf("source line %d: malformed process_cell", lineNumber)
		}
		contract.Cells = append(contract.Cells, ProcessGuardCell{ID: fields[1], Rule: fields[2], Expected: Decision(fields[3])})
	case "process_case":
		if len(fields) != 4 {
			return fmt.Errorf("source line %d: malformed process_case", lineNumber)
		}
		contract.Cases = append(contract.Cases, ProcessGuardCase{ID: fields[1], Evidence: fields[2], Expected: Decision(fields[3])})
	case "process_pr":
		if len(fields) != 9 {
			return fmt.Errorf("source line %d: malformed process_pr", lineNumber)
		}
		number, err := strconv.Atoi(fields[1])
		if err != nil {
			return fmt.Errorf("source line %d: malformed process_pr number", lineNumber)
		}
		contract.PullRequests = append(contract.PullRequests, ProcessPRRule{Number: number, URL: fields[2], BaseRef: fields[3], HeadRef: fields[4], MergeCommit: fields[5], MergeParents: []string{fields[6], fields[7]}})
	case "process_release":
		if len(fields) != 6 {
			return fmt.Errorf("source line %d: malformed process_release", lineNumber)
		}
		releaseID, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return fmt.Errorf("source line %d: malformed process_release id", lineNumber)
		}
		immutable, err := strconv.ParseBool(fields[5])
		if err != nil {
			return fmt.Errorf("source line %d: malformed process_release immutable", lineNumber)
		}
		contract.Releases = append(contract.Releases, ProcessReleaseRule{Tag: fields[1], ReleaseID: releaseID, TagObject: fields[3], TargetCommit: fields[4], Immutable: immutable})
	case "process_asset":
		if len(fields) != 6 {
			return fmt.Errorf("source line %d: malformed process_asset", lineNumber)
		}
		assetID, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return fmt.Errorf("source line %d: malformed process_asset id", lineNumber)
		}
		sizeBytes, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return fmt.Errorf("source line %d: malformed process_asset size", lineNumber)
		}
		contract.Assets = append(contract.Assets, ProcessAssetRule{Tag: fields[1], AssetID: assetID, Name: fields[3], SizeBytes: sizeBytes, Digest: fields[5]})
	default:
		return fmt.Errorf("source line %d: unknown process record %q", lineNumber, fields[0])
	}
	return nil
}

func parseTokens(line string) ([]string, error) {
	result := make([]string, 0)
	for index := 0; index < len(line); {
		for index < len(line) && (line[index] == ' ' || line[index] == '\t') {
			index++
		}
		if index == len(line) {
			break
		}
		if line[index] == '"' {
			start := index
			index++
			escaped := false
			closed := false
			for index < len(line) {
				if escaped {
					escaped = false
					index++
					continue
				}
				if line[index] == '\\' {
					escaped = true
					index++
					continue
				}
				if line[index] == '"' {
					index++
					closed = true
					break
				}
				index++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted token")
			}
			value, err := strconv.Unquote(line[start:index])
			if err != nil {
				return nil, fmt.Errorf("malformed quoted token: %w", err)
			}
			result = append(result, value)
			continue
		}
		start := index
		for index < len(line) && line[index] != ' ' && line[index] != '\t' {
			index++
		}
		result = append(result, line[start:index])
	}
	return result, nil
}

func metadataValue(computes, key string) string {
	for _, part := range strings.Split(computes, ";") {
		name, value, ok := strings.Cut(part, "=")
		if ok && name == key {
			return value
		}
	}
	return ""
}

func LoadJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return decodeJSON(data, target)
}
