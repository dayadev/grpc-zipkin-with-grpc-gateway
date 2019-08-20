package middleware

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	"github.com/sirupsen/logrus"

	"time"

	"google.golang.org/grpc"
)

// LoggingInterceptor returns a new unary server interceptor for Logging.
func LoggingInterceptor(logger *logrus.Entry) grpc.UnaryServerInterceptor {

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func(begin time.Time) {
			spanCtx := opentracing.SpanFromContext(ctx).Context().(zipkinot.SpanContext)
			traceID := spanCtx.TraceID.String()
			spanID := spanCtx.ID.String()

			if err != nil {
				logger.WithFields(logrus.Fields{
					"traceId": traceID,
					"error":   err,
					"spanId":  spanID,
					"method":  info.FullMethod,
					"took":    time.Since(begin).String(),
					"input":   req,
				}).Error("Error executing client request.")
			} else {
				logger.WithFields(logrus.Fields{
					"traceId": traceID,
					"spanId":  spanID,
					"method":  info.FullMethod,
					"took":    time.Since(begin).String(),
					"input":   req,
				}).Info("Client request completed successfully.")
			}
		}(time.Now())

		return handler(ctx, req)
	}
}
