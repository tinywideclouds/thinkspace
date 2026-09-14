package assembler

import "context"

// MapbookLayer represents a distinct slice of the workspace context.
type MapbookLayer string

const (
	LayerStructural    MapbookLayer = "Structural"
	LayerConceptual    MapbookLayer = "Conceptual"
	LayerMacroTemporal MapbookLayer = "MacroTemporal"
)

// Mapbook generates context layers. It is implemented by specific ThinkSpaces.
type Mapbook interface {
	// GenerateLayers returns a map of layer names to their compiled string representation.
	GenerateLayers(ctx context.Context, workspaceRoot string) (map[MapbookLayer]string, error)
}
