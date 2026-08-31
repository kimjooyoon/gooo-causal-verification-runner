package runner

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var activityPattern = regexp.MustCompile(`^activity\s+([A-Za-z_][A-Za-z0-9_]*)\(([^)]*)\)\s*->\s*([A-Za-z_][A-Za-z0-9_]*)(?:\s+computes\s+"([^"]*)")?\s*$`)

func ParseSource(path string) (SourceSpec, string, error) {
	digest, data, err := DigestFile(path)
	if err != nil {
		return SourceSpec{}, "", err
	}
	var spec SourceSpec
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "package":
			if len(fields) != 2 {
				return SourceSpec{}, "", fmt.Errorf("source line %d: malformed package", lineNumber+1)
			}
			spec.Package = fields[1]
		case "namespace":
			if len(fields) != 2 {
				return SourceSpec{}, "", fmt.Errorf("source line %d: malformed namespace", lineNumber+1)
			}
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
		Schema: IRScheme, Protocol: ProtocolSchema, SourcePath: sourcePath,
		SourceDigest: sourceDigest, Activities: spec.Activities,
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
