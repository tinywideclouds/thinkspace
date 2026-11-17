package telemetry_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"uuid"
)

type mockTelemetryPayload struct {
	text string
}

func (p mockTelemetryPayload) Summary() string {
	return p.text
}

type recordingEmitter struct {
	events []telemetry.Event
}

func (r *recordingEmitter) Emit(channel string, event telemetry.Event) {
	r.events = append(r.events, event)
}

func TestSlogEmitter_LevelsSummaryAndNilLoggerSafety(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	emitter := telemetry.NewSlogEmitter(logger)

	// Info event derives summary from Payload when Summary field is empty
	emitter.Emit("agent-1", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_status",
		Timestamp: time.Now(),
		Message:   "Running build",
		Payload:   mockTelemetryPayload{text: "step 1/2"},
	})

	output := buf.String()
	if !strings.Contains(output, "level=INFO") || !strings.Contains(output, `msg="Running build"`) || !strings.Contains(output, `summary="step 1/2"`) {
		t.Errorf("unexpected INFO log output: %s", output)
	}

	buf.Reset()

	// Error event logs at WARN level and preserves explicit Summary over Payload
	emitter.Emit("agent-1", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_error",
		Timestamp: time.Now(),
		Message:   "Verification failed",
		Summary:   "explicit summary",
		Payload:   mockTelemetryPayload{text: "payload summary"},
	})

	errOutput := buf.String()
	if !strings.Contains(errOutput, "level=WARN") || !strings.Contains(errOutput, `summary="explicit summary"`) {
		t.Errorf("unexpected WARN log output: %s", errOutput)
	}

	// Nil logger must not panic
	nilLoggerEmitter := telemetry.NewSlogEmitter(nil)
	nilLoggerEmitter.Emit("manager", telemetry.Event{ID: uuid.NewV7(), Type: "info", Message: "safe"})
}

func TestMultiEmitter_BroadcastsToNonNilEmitters(t *testing.T) {
	rec1 := &recordingEmitter{}
	rec2 := &recordingEmitter{}
	multi := telemetry.MultiEmitter{rec1, nil, rec2}

	ev := telemetry.Event{ID: uuid.NewV7(), Type: "flow_complete", Message: "Done"}
	multi.Emit("manager", ev)

	if len(rec1.events) != 1 || len(rec2.events) != 1 {
		t.Errorf("expected both recording emitters to receive 1 event, got %d and %d", len(rec1.events), len(rec2.events))
	}
}
