package httpapi

import (
	"context"
	"log/slog"
)

type Event struct {
	Name       string
	Message    string
	Level      slog.Level
	Attributes []slog.Attr
}

// LogEvent records a business decision using the request logger and correlation ID.
// Propagated errors remain owned by the request error boundary.
func LogEvent(ctx context.Context, event Event) {
	request, _ := ctx.Value(requestContextKey{}).(requestContext)
	if request.gin == nil || request.logger == nil {
		return
	}
	attributes := append([]slog.Attr{
		slog.String("event", event.Name),
		slog.String("request_id", RequestID(request.gin)),
	}, event.Attributes...)
	request.logger.LogAttrs(ctx, event.Level, event.Message, attributes...)
}
