package workspace

import "google.golang.org/genai"

type ContextAssembler struct {
	// dependencies like the active workspace file reader
}

type AssemblyRequest struct {
	Manifest     *ThreadManifest
	ActiveLenses []string // e.g., ["performance", "auth"]
	RecencyCount int      // Tail length of raw messages
}

func (a *ContextAssembler) Build(req AssemblyRequest) ([]*genai.Content, error) {
	var history []*genai.Content

	// 1. Add Sticky Digests (Always active)
	for _, digest := range req.Manifest.Digests {
		if digest.IsSticky {
			history = append(history, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: "[Sticky Context]: " + digest.Summary}},
			})
		}
	}

	// 2. Add Lens-Specific Context (The subgraph)
	for _, tag := range req.ActiveLenses {
		if uuids, exists := req.Manifest.Lenses[tag]; exists {
			for _, id := range uuids {
				if digest, ok := req.Manifest.Digests[id]; ok {
					history = append(history, &genai.Content{
						Role:  "user",
						Parts: []*genai.Part{{Text: "[Context - " + tag + "]: " + digest.Summary}},
					})
				}
				// (Later: we can also fetch raw file summaries here based on the UUID)
			}
		}
	}

	// 3. (TODO) Tail the active .jsonl shard for the `RecencyCount` of raw messages...

	return history, nil
}
