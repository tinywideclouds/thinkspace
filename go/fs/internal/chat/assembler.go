package chat

import (
	"strings"

	"google.golang.org/genai"
)

type AssemblyRequest struct {
	Manifest     *ThreadManifest
	ActiveLenses []string
	RecentEvents []Event
}

type Assembler struct{}

func NewAssembler() *Assembler {
	return &Assembler{}
}

func (a *Assembler) Build(request AssemblyRequest) ([]*genai.Content, error) {
	var contextHistory []*genai.Content

	for _, digest := range request.Manifest.Digests {
		if digest.IsSticky {
			contextHistory = append(contextHistory, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: "[Sticky Context]: " + digest.Summary}},
			})
		}
	}

	for _, tag := range request.ActiveLenses {
		if digestIDs, exists := request.Manifest.Lenses[tag]; exists {
			for _, id := range digestIDs {
				if digest, ok := request.Manifest.Digests[id]; ok {
					contextHistory = append(contextHistory, &genai.Content{
						Role:  "user",
						Parts: []*genai.Part{{Text: "[Context - " + tag + "]: " + digest.Summary}},
					})
				}
			}
		}
	}

	// Project the available tags so the LLM knows what to query
	var availableTags []string
	for tag := range request.Manifest.Lenses {
		availableTags = append(availableTags, tag)
	}

	tagMessage := "[No historical tags currently defined in workspace]"
	if len(availableTags) > 0 {
		tagMessage = "[Available Conceptual Tags for memory retrieval: " + strings.Join(availableTags, ", ") + "]"
	}

	contextHistory = append(contextHistory, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: tagMessage}},
	})

	// Append the recent conversational events (capped to the last 5 to avoid bloating)
	windowSize := 5
	start := 0
	if len(request.RecentEvents) > windowSize {
		start = len(request.RecentEvents) - windowSize
	}

	for i := start; i < len(request.RecentEvents); i++ {
		event := request.RecentEvents[i]
		role := "user"
		if event.Type == EventModel {
			role = "model"
		}
		contextHistory = append(contextHistory, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: event.Content}},
		})
	}

	return contextHistory, nil
}
