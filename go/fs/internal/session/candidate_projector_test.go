package session

import (
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

type stubGoSpace struct {
	spaces.ThinkState
}

func (s *stubGoSpace) Config() spaces.ThinkStateConfig {
	return spaces.ThinkStateConfig{Name: "golang"}
}

func TestProjectCandidateForSpace_GoASTAndPlainFiles(t *testing.T) {
	cand := flows.Candidate{
		ID:      "chat-1-flow-1-agent-1",
		AgentID: "agent-1",
		Passed:  true,
		Files: map[string]string{
			"src/go.mod": "module fixtureapi\n\ngo 1.24\n",
			"src/internal/server/server.go": `package server

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Status string ` + "`json:\"status\"`" + `
}

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Response{Status: "ok"})
	})
	return mux
}
`,
		},
	}

	projected := ProjectCandidateForSpace(&stubGoSpace{}, cand)

	for _, want := range []string{
		"#### File `src/go.mod`",
		"module fixtureapi",
		"#### File `src/internal/server/server.go` (`package server`)",
		"`struct Response`",
		"`struct Server`",
		"`func NewServer() *Server`",
		"`func (s *Server) Routes() http.Handler`",
		`mux.HandleFunc("GET /{$}"`,
	} {
		if !strings.Contains(projected, want) {
			t.Errorf("expected Go AST candidate projection to contain %q, got:\n%s", want, projected)
		}
	}

	if strings.Contains(projected, "+package server") {
		t.Errorf("expected clean AST source blocks without unified diff '+' prefixes")
	}
}

func TestExtractSynthesisReasoning(t *testing.T) {
	jsonPayload := `{"thought": "Adopted Server struct from candidate 2 and resilient header check from candidate 1.", "patches": []}`
	if got := extractSynthesisReasoning(jsonPayload); got != "Adopted Server struct from candidate 2 and resilient header check from candidate 1." {
		t.Errorf("unexpected extracted JSON thought: %q", got)
	}

	markdownPayload := "Merged intro structure from candidate 1 with examples from candidate 2.\n\n### docs/spec.md\n<<<< SEARCH\nold\n====\nnew\n>>>> REPLACE"
	if got := extractSynthesisReasoning(markdownPayload); got != "Merged intro structure from candidate 1 with examples from candidate 2." {
		t.Errorf("unexpected extracted Markdown rationale: %q", got)
	}
}
