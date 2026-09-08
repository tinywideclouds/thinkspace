package chat

import "google.golang.org/genai"

type ContextAssemblyRequest struct {
	Manifest     *ThreadManifest
	ActiveLenses []string
	RecentEvents []Event
}

type ContextAssembler struct{}

func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{}
}

func (assembler *ContextAssembler) Build(request ContextAssemblyRequest) ([]*genai.Content, error) {
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

	// Append the recent conversational events
	for _, event := range request.RecentEvents {
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
