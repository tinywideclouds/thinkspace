package llm_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
)

func TestCompositeHealer_TestData(t *testing.T) {
	// We test the full pipeline assembled for the gemini-3.5-flash family
	healer := llm.GetHealerForModel("gemini-3.5-flash")
	if healer == nil {
		t.Fatalf("Expected a configured composite healer, got nil")
	}

	matchPath := filepath.Join("testdata", "healers", "gemini", "*_raw.txt")
	files, err := filepath.Glob(matchPath)
	if err != nil {
		t.Fatalf("Failed to glob testdata: %v", err)
	}

	if len(files) == 0 {
		t.Skip("No testdata found. Create files in internal/llm/testdata/healers/gemini/ to run.")
	}

	for _, rawFile := range files {
		baseName := strings.TrimSuffix(filepath.Base(rawFile), "_raw.txt")
		t.Run(baseName, func(t *testing.T) {
			expectedFile := filepath.Join(filepath.Dir(rawFile), baseName+"_expected.json")

			rawBytes, err := os.ReadFile(rawFile)
			if err != nil {
				t.Fatalf("Failed to read raw file: %v", err)
			}

			expectedBytes, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Fatalf("Failed to read expected file: %v", err)
			}

			rawStr := string(rawBytes)
			expectedStr := strings.TrimSpace(string(expectedBytes))

			healedStr, modified := healer.Heal(rawStr)

			if strings.TrimSpace(rawStr) != expectedStr && !modified {
				t.Errorf("Expected healer to flag as modified, but it returned false")
			}

			if healedStr != expectedStr {
				t.Errorf("Healer output mismatch.\nExpected:\n%s\nGot:\n%s", expectedStr, healedStr)
			}
		})
	}
}
