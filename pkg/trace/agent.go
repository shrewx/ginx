package trace

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/shrewx/ginx/pkg/logx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	JaegerExporter = "jaeger"
	ZipkinExporter = "zipkin"
)

type Agent struct {
	endpoint       string
	exporter       string
	ServiceName    string
	TracerProvider trace.TracerProvider
	Propagators    propagation.TextMapPropagator
}

func NewAgent(serviceName, endpoint, exporter string) *Agent {
	return &Agent{
		ServiceName: serviceName,
		endpoint:    endpoint,
		exporter:    exporter,
	}
}

func (a *Agent) Init() error {
	var (
		exporter sdktrace.SpanExporter
		err      error
	)
	switch a.endpoint {
	case JaegerExporter:
		exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(a.endpoint)))
	case ZipkinExporter:
		exporter, err = zipkin.New(a.endpoint)
	default:
		exporter, err = stdouttrace.New()
	}

	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to new %s exporter", a.exporter))
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service", a.ServiceName))),
	}
	if a.endpoint != "" {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	a.TracerProvider = sdktrace.NewTracerProvider(opts...)
	a.Propagators = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

	otel.SetTracerProvider(a.TracerProvider)
	otel.SetTextMapPropagator(a.Propagators)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) { logx.Errorf("[otel agent] error: %v", err) }))

	return nil
}
