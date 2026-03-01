package vllmipc

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

// ExtractTraceContext creates a map of trace context headers from the current context.
// This allows tracing spans to cross the process boundary into the Python vLLM router.
func ExtractTraceContext(ctx context.Context) map[string]string {
	propagator := propagation.TraceContext{}
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	return carrier
}
