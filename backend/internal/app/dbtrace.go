package app

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"strings"
)

type dbTracer struct{}

func (dbTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	op := "query"
	fields := strings.Fields(data.SQL)
	if len(fields) > 0 {
		op = strings.ToUpper(fields[0])
	}
	ctx, _ = otel.Tracer("strefa.postgres").Start(ctx, "postgres."+op, trace.WithAttributes(attribute.String("db.system.name", "postgresql")))
	return ctx
}
func (dbTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		span.RecordError(errors.New("database query failed"))
	}
	span.End()
}
