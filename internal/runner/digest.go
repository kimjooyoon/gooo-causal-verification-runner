package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestFile(path string) (string, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return DigestBytes(data), data, nil
}

func DigestJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(data), nil
}

func MustDigest(value string) error {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return fmt.Errorf("invalid digest %q", value)
	}
	for _, char := range value[len("sha256:"):] {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return fmt.Errorf("invalid digest %q", value)
		}
	}
	return nil
}

func TreeDigest(root string) (string, Inventory, error) {
	type entry struct {
		path string
		data []byte
	}
	entries := make([]entry, 0)
	inventory := Inventory{RootReadmeExcluded: true}
	err := filepath.WalkDir(root, func(path string, dirent fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirent.IsDir() {
			if dirent.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !dirent.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "README.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{path: relative, data: data})
		inventory.RegularFiles++
		inventory.TreeBytes += int64(len(data))
		switch {
		case strings.HasSuffix(relative, ".go"):
			inventory.GoFiles++
			inventory.GoLines += int64(countLines(data))
		case strings.HasSuffix(relative, ".gooo"):
			inventory.GoooFiles++
			inventory.GoooLines += int64(countLines(data))
		}
		return nil
	})
	if err != nil {
		return "", Inventory{}, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	h := sha256.New()
	for _, entry := range entries {
		_, _ = h.Write([]byte(entry.path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(entry.data)
		_, _ = h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), inventory, nil
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := 1
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if data[len(data)-1] == '\n' {
		lines--
	}
	return lines
}

func canonicalCopy[T any](value T, clear func(*T)) (T, error) {
	data, err := json.Marshal(value)
	if err != nil {
		var zero T
		return zero, err
	}
	var copyValue T
	if err := json.Unmarshal(data, &copyValue); err != nil {
		var zero T
		return zero, err
	}
	clear(&copyValue)
	return copyValue, nil
}

func ScenarioDigest(value Case) (string, error) {
	copyValue, err := canonicalCopy(value, func(item *Case) {
		item.ScenarioDigest = ""
		item.SemanticGraph.GraphDigest = ""
		item.FullOracle.Digest = ""
	})
	if err != nil {
		return "", err
	}
	return DigestJSON(copyValue)
}

func GraphDigest(value SemanticGraph) (string, error) {
	copyValue := value
	copyValue.GraphDigest = ""
	return DigestJSON(copyValue)
}

func OracleDigest(value FullOracle) (string, error) {
	copyValue := value
	copyValue.Digest = ""
	return DigestJSON(copyValue)
}
