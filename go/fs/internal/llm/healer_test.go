package llm_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
)

func TestCompositeHealer_TestData(t *testing.T) {
	healer := llm.GetHealerForModel("gemini-4-hydrogen")
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

func TestCompositeHealer_ModelAgnosticAndBackslashSpace(t *testing.T) {
	models := []string{"gemini-4-hydrogen", "gemini-3.8-flash", "custom-model-v1", ""}
	raw := "```json\n{\"thought\":\"Fixing spacing\",\"patches\":[{\"file\":\"src/main.go\",\"action\":\"full_replace\",\"code\":\"package main\\n// comment\\ with backslash space\"}]}\n```"

	for _, model := range models {
		t.Run("model_"+model, func(t *testing.T) {
			healer := llm.GetHealerForModel(model)
			if healer == nil {
				t.Fatalf("expected non-nil healer for model %q", model)
			}

			healed, modified := healer.Heal(raw)
			if !modified {
				t.Errorf("expected healer to sanitize markdown fences and backslash-space escape")
			}

			var parsed map[string]any
			if err := json.Unmarshal([]byte(healed), &parsed); err != nil {
				t.Fatalf("expected healed JSON to unmarshal cleanly, got error: %v\nhealed:\n%s", err, healed)
			}
		})
	}
}

func TestGoEscapeSanitizer_PreservesValidEscapedBackslashes(t *testing.T) {
	sanitizer := &llm.GoEscapeSanitizer{}

	// Valid JSON containing an escaped backslash followed by ')' (represents literal `\)` in Go regex)
	validJSON := `{"pattern":"\\)","unicode":"\u0041","newline":"\n"}`
	healed, modified := sanitizer.Heal(validJSON)
	if modified {
		t.Errorf("expected valid escaped backslash JSON not to be modified, got: %s", healed)
	}
	if healed != validJSON {
		t.Errorf("expected %q, got %q", validJSON, healed)
	}
}
