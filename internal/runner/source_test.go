package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSourceRejectsDuplicateSingletons(t *testing.T) {
	cases := map[string]string{
		"package": `package one
package two
namespace gooo://causal-verification/v1
activity Observe() -> Result
`,
		"namespace": `package one
namespace gooo://first
namespace gooo://second
activity Observe() -> Result
`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.gooo")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := ParseSource(path); err == nil {
				t.Fatalf("ParseSource accepted duplicate %s", name)
			}
		})
	}
}
