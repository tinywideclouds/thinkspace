package llm

import (
	"regexp"
	"strings"
)

// ResponseHealer defines middleware to correct known model-specific hallucination or syntax errors.
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
	// If it already looks like a pure JSON object, skip heavy regex.
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return raw, false
	}

	// Use a strict anchor to our domain keys to avoid crashing on stray brackets in conversational text
	re := regexp.MustCompile(`(?s)\{\s*"(thought|patches)"\s*:.*}`)
	match := re.FindString(raw)
	if match != "" {
		return match, true
	}

	// Absolute fallback
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		return strings.TrimSpace(raw[start : end+1]), true
	}

	return raw, false
}

// GoEscapeSanitizer fixes invalid JSON escapes commonly hallucinated inside Go code strings (e.g., `\)`).
type GoEscapeSanitizer struct{}

func (h *GoEscapeSanitizer) Heal(raw string) (string, bool) {
	re := regexp.MustCompile(`\\([^"\\/bfnrtu])`)
	sanitized := re.ReplaceAllString(raw, "$1")

	if sanitized != raw {
		return sanitized, true
	}
	return raw, false
}

// --- Factory ---

// GetHealerForModel composes a specific pipeline of heuristics based on the model family.
func GetHealerForModel(modelName string) ResponseHealer {
	var pipeline CompositeHealer

	// Generic JSON extraction applies to almost all current instruction-tuned models
	pipeline = append(pipeline, &MarkdownBlockExtractor{})
	pipeline = append(pipeline, &JSONBoundaryExtractor{})

	// Gemini family has specific quirks with Go string escapes
	if strings.Contains(modelName, "gemini") || strings.Contains(modelName, "test-worker") {
		pipeline = append(pipeline, &GoEscapeSanitizer{})
	}

	if len(pipeline) > 0 {
		return pipeline
	}
	return nil
}
