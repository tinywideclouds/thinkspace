package thinkgraph

import (
	"fmt"
	"strings"
	"uuid"
)

type GraphPrefix string

const (
	PrefixChat    GraphPrefix = "c"
	PrefixFlow    GraphPrefix = "f"
	PrefixFeature GraphPrefix = "ft"
	PrefixSystem  GraphPrefix = "s"
)

const SystemSubGraphName = "system"

// GraphID strongly types our DAG identities while retaining native UUID v7 sorting and operations.
type GraphID struct {
	Prefix GraphPrefix
	UUID   uuid.UUID
}

// NewGraphID generates a time-ordered v7 UUID under a specific domain prefix.
func NewGraphID(prefix GraphPrefix) GraphID {
	return GraphID{
		Prefix: prefix,
		UUID:   uuid.NewV7(),
	}
}

// ParseGraphID safely reconstructs a GraphID from its string representation.
func ParseGraphID(s string) (GraphID, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return GraphID{}, fmt.Errorf("invalid GraphID format: missing prefix delimiter")
	}

	parsedUUID, err := uuid.Parse(parts[1])
	if err != nil {
		return GraphID{}, fmt.Errorf("invalid UUID payload in GraphID: %w", err)
	}

	return GraphID{
		Prefix: GraphPrefix(parts[0]),
		UUID:   parsedUUID,
	}, nil
}

// String implements the fmt.Stringer interface (e.g., "s:018e6b...").
func (g GraphID) String() string {
	return fmt.Sprintf("%s:%s", g.Prefix, g.UUID.String())
}

// IsZero allows for easy empty-state checking.
func (g GraphID) IsZero() bool {
	var empty uuid.UUID
	return g.UUID == empty
}

func (g GraphID) MarshalText() ([]byte, error) {
	return []byte(g.String()), nil
}

func (g *GraphID) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	parsed, err := ParseGraphID(string(data))
	if err != nil {
		return err
	}
	*g = parsed
	return nil
}
