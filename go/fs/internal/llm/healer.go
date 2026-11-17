package llm

import (
	"log/slog"
	"regexp"
	"strings"
)

var (
	jsonBoundaryAnchoredRegex = regexp.MustCompile(`(?s)\{\s*"(thought|patches|digests)"\s*:.*}`)
	jsonEscapeTokenRegex      = regexp.MustCompile(`\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}|(.))`)
)

// ResponseHealer defines middleware to correct known model hallucination or syntax errors.
type ResponseHealer interface {
	Heal(raw string) (string, bool)
}

// CompositeHealer chains multiple heuristics together.
type CompositeHealer []ResponseHealer

func (ch CompositeHealer) Heal(raw string) (string, bool) {
	current := raw
	anyModified := false

	for _, healer := range ch {
		healed, modified := healer.Heal(current)
		if modified {
			current = healed
			anyModified = true
		}
	}

	return current, anyModified
}

// --- Granular Heuristic Library ---

// MarkdownBlockExtractor strips conversational wrappers if the LLM output the JSON inside a markdown code block.
type MarkdownBlockExtractor struct{}

func (h *MarkdownBlockExtractor) Heal(raw string) (string, bool) {
	start := strings.Index(raw, "```json")
	if start != -1 {
		start += 7
		end := strings.Index(raw[start:], "```")
		if end != -1 {
			return strings.TrimSpace(raw[start : start+end]), true
		}
	}
	return raw, false
}

// JSONBoundaryExtractor anchors to expected JSON keys to safely extract objects buried in stray text.
type JSONBoundaryExtractor struct{}

func (h *JSONBoundaryExtractor) Heal(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return raw, false
	}

	match := jsonBoundaryAnchoredRegex.FindString(raw)
	if match != "" {
		return match, true
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(raw[start : end+1]), true
	}

	return raw, false
}

// GoEscapeSanitizer fixes invalid JSON escapes commonly hallucinated inside Go code strings (e.g., `\)` or `\ `)
// while preserving valid escapes such as `\\`, `\n`, `\t`, `\"`, and `\uXXXX`.
type GoEscapeSanitizer struct{}

func (h *GoEscapeSanitizer) Heal(raw string) (string, bool) {
	modified := false
	sanitized := jsonEscapeTokenRegex.ReplaceAllStringFunc(raw, func(match string) string {
		// Valid 2-char escape (\\, \", \/, \b, \f, \n, \r, \t) or \uXXXX
		if len(match) == 6 || strings.ContainsRune(`"\/bfnrt`, rune(match[1])) {
			return match
		}
		modified = true
		return match[1:]
	})

	return sanitized, modified
}

// DefaultJSONHealer returns the standard model-agnostic JSON response healing pipeline.
func DefaultJSONHealer() ResponseHealer {
	return CompositeHealer{
		&MarkdownBlockExtractor{},
		&JSONBoundaryExtractor{},
		&GoEscapeSanitizer{},
	}
}

// --- Factory ---

// GetHealerForModel composes the JSON response healing pipeline for a configured model.
// If no model name is provided, it logs a warning and still returns the default JSON healer.
func GetHealerForModel(modelName string) ResponseHealer {
	if strings.TrimSpace(modelName) == "" {
		slog.Warn("GetHealerForModel called with empty model name; falling back to default JSON healer")
	}
	return DefaultJSONHealer()
}
