package buslog

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
)

// BusHandler is a slog.Handler that writes log records to a bus.Bus.
// It also wraps another slog.Handler to continue writing to the original destination.
type BusHandler struct {
	bus bus.Bus
}

// NewBusHandler creates a new BusHandler.
func NewBusHandler(bus bus.Bus) *BusHandler {
	return &BusHandler{
		bus: bus,
	}
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *BusHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Handle handles the Record.
// It broadcasts the log message to the bus and then passes the record
// to the wrapped handler.
func (h *BusHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format the record into a simple string.
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("[%s] %s", r.Level, r.Message))

	return h.bus.Broadcast(&message.Message{
		Text: buf.String(),
		At:   time.Now(),
		Kind: message.KindLog,
	})
}

// WithAttrs returns a new BusHandler whose attributes consist of
// the handler's attributes followed by attrs.
func (h *BusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BusHandler{
		bus: h.bus,
	}
}

// WithGroup returns a new BusHandler with the given group name.
func (h *BusHandler) WithGroup(name string) slog.Handler {
	return &BusHandler{
		bus: h.bus,
	}
}
