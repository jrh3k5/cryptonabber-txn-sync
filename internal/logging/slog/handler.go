package slog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Handler formats logs like the default slog output: "YYYY/MM/DD HH:MM:SS LEVEL Message"
type Handler struct {
	out   io.Writer
	opts  *slog.HandlerOptions
	attrs []slog.Attr
}

func NewHandler(out io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	return &Handler{out: out, opts: opts}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.opts != nil && h.opts.Level != nil {
		return level >= h.opts.Level.Level()
	}

	return true
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	ts := r.Time.Format("2006/01/02 15:04:05")
	lvl := strings.ToUpper(r.Level.String())
	_, err := fmt.Fprintf(h.out, "%s %s %s\n", ts, lvl, r.Message)
	if err != nil {
		return fmt.Errorf("unable to write log record: %w", err)
	}

	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	copyLogger := *h
	copyLogger.attrs = append(copyLogger.attrs, attrs...)

	return &copyLogger
}

func (h *Handler) WithGroup(name string) slog.Handler { return h }
