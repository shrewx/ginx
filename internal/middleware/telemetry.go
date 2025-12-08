package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/logx"
	ptrace "github.com/shrewx/ginx/pkg/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	otrace "go.opentelemetry.io/otel/trace"
	"time"
)

const (
	Unknown           = "unknown"
	OperationName     = "x-operation-name"
	TracerKey         = "x-tracer-key"
	RequestContextKey = "x-request-ctx-key"
	TracerName        = "github.com/shrewx/ginx/tracer"
)

func Telemetry(agent *ptrace.Agent) gin.HandlerFunc {
	if agent.TracerProvider == nil {
		agent.TracerProvider = otel.GetTracerProvider()
	}
	tracer := agent.TracerProvider.Tracer(
		TracerName,
		otrace.WithInstrumentationVersion("0.0.1"),
	)
	if agent.Propagators == nil {
		agent.Propagators = otel.GetTextMapPropagator()
	}
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		filterUri := []string{"/health"}
		for i := range filterUri {
			if filterUri[i] == path {
				c.Next()
				return
			}
		}

		c.Set(TracerKey, tracer)
		savedCtx := c.Request.Context()
		defer func() {
			c.Request = c.Request.WithContext(savedCtx)
		}()
		ctx := agent.Propagators.Extract(savedCtx, propagation.HeaderCarrier(c.Request.Header))
		opts := []otrace.SpanStartOption{
			otrace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", c.Request)...),
			otrace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(c.Request)...),
			otrace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(agent.ServiceName, c.FullPath(), c.Request)...),
			otrace.WithSpanKind(otrace.SpanKindServer),
		}

		ctx, span := tracer.Start(ctx, c.Request.URL.Path, opts...)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)

		c.Set(RequestContextKey, c.Request)

		c.Next()

		latency := time.Since(start)

		operationName := Unknown
		if v, ok := c.Get(OperationName); ok {
			operationName = v.(string)
		}

		span.SetName(operationName)
		span.SetAttributes(semconv.HTTPAttributesFromHTTPStatusCode(c.Writer.Status())...)
		span.SetStatus(semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(c.Writer.Status(), otrace.SpanKindServer))
		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.String("gin.errors", c.Errors.String()))
		}

		logx.Info("cost: ", latency, ", path: ", c.Request.URL.Path, ", trace id: ", span.SpanContext().TraceID(),
			", remote ip: ", c.Request.RemoteAddr, ", operator: ", operationName, ", status: ", c.Writer.Status())
	}
}
