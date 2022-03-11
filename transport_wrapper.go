package goreq

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

var (
	logger *zap.Logger
)

func initTrace() {
	ctx := context.Background()
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("server"),
		),
	)
	if err != nil {
		logger.With(zap.Error(err)).
			Fatal("failed to New resource")
	}
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	}
	if traceEndpoint := os.Getenv("TRACE_ENDPOINT"); traceEndpoint != "" {
		conn, err := grpc.DialContext(ctx, traceEndpoint, grpc.WithInsecure())
		if err != nil {
			logger.With(zap.Error(err)).
				Fatal("failed to DialContext")
		}
		traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			logger.With(zap.Error(err)).
				Fatal("failed to New traceExporter")
		}
		bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
		opts = append(opts, sdktrace.WithSpanProcessor(bsp))
	}

	tracerProvider := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	go func() {
		<-ch
		err := tracerProvider.Shutdown(ctx)
		if err != nil {
			log.Println(err)
		}
	}()
}
func initLog() {
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:       "ts",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			time := t.Format("2006-01-02 15:04:05")
			enc.AppendString(time)
		},
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
	core := zapcore.NewCore(encoder, zapcore.Lock(zapcore.AddSync(os.Stdout)), zapcore.InfoLevel)
	logger = zap.New(core)
}

func init() {
	initLog()
	initTrace()
}

func TraceTransport(name string) func(next http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		tracer := otel.Tracer(name)
		return Transport(func(r *http.Request) (resp *http.Response, err error) {
			ctx, span := tracer.Start(
				r.Context(),
				"client_request")
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))
			sc := span.SpanContext()
			traceID := sc.TraceID()
			ctx = contextWithTraceID(ctx, traceID.String())
			r = r.WithContext(ctx)
			resp, err = next.RoundTrip(r)
			if err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
}

func LoggingTransport(name string) func(next http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return Transport(func(r *http.Request) (resp *http.Response, err error) {
			start := time.Now()
			defer func() {
				logger = logger.With(zap.String("host", r.Host)).
					With(zap.String("host", r.URL.Path)).
					With(zap.String("service", name)).
					With(zap.String("start", start.Format("2006-01-02 15:04:05"))).
					With(zap.Int64("elapsed", time.Since(start).Milliseconds())).
					With(zap.String("trace_id", traceIDFromContext(r.Context())))
				if resp != nil {
					logger = logger.With(zap.Int("statuscode", resp.StatusCode))
				}
				logger.Info("client_request")
			}()
			resp, err = next.RoundTrip(r)
			if err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
}

func TimeoutTransport(duration time.Duration) func(next http.RoundTripper) http.RoundTripper {
	if duration <= 0 {
		duration = time.Second * 5
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return Transport(func(r *http.Request) (*http.Response, error) {
			var cancel context.CancelFunc
			ctx := r.Context()
			deadline, ok := ctx.Deadline()
			if !ok || deadline.After(time.Now().Add(duration)) {
				ctx, cancel = context.WithTimeout(r.Context(), duration)
			}

			defer cancel()
			r = r.WithContext(ctx)
			resp, err := next.RoundTrip(r)
			if err != nil {
				return nil, err
			}
			return resp, nil
		})
	}
}

type traceIDKey struct{}

func contextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func traceIDFromContext(ctx context.Context) string {
	traceID, ok := ctx.Value(traceIDKey{}).(string)
	if ok {
		return traceID
	}
	return ""
}
