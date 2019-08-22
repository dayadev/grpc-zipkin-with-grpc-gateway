package main

import (
	"net"
	"os"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	opentracing "github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	zipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	middleware "grpc-zipkin-with-grpc-gateway/middleware"
	pb "grpc-zipkin-with-grpc-gateway/pb"
	service "grpc-zipkin-with-grpc-gateway/pkg"
)

func main() {
	var log = logrus.New()
	log.Out = os.Stdout
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyMsg: "message",
		},
		TimestampFormat: "2006-01-02T15:04:05.000Z",
	}
	logger := log.WithFields(logrus.Fields{"service": "helloservice"})

	grpcListener, err := net.Listen("tcp", ":9090")
	if err != nil {
		logger.WithFields(logrus.Fields{
			"transport": "gRPC",
			"during":    "serve",
			"error":     err,
		}).Error("Unable to start grpc Listener")
	}

	reporter := zipkinhttp.NewReporter("http://localhost:9411/api/v2/spans")
	defer reporter.Close()

	// create our local service endpoint
	endpoint, err := zipkin.NewEndpoint("hello", "hello:9090")
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("unable to create local endpoint")
	}

	// initialize our tracer
	nativeTracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(endpoint))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Unable to create Zipkin tracer")
	}

	// use zipkin-go-opentracing to wrap our tracer
	tracer := zipkinot.Wrap(nativeTracer)

	opentracing.SetGlobalTracer(tracer)
	srv := service.NewHelloService(logger)

	var s *grpc.Server

	s = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(tracer)),
		middleware.LoggingInterceptor(logger))), grpc.MaxSendMsgSize(50000000))

	pb.RegisterHelloServer(s, srv)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	logger.WithFields(logrus.Fields{
		"transport":                 "gRPC",
		"service listening on port": ":9090",
	}).Info("hello service started")

	if err := s.Serve(grpcListener); err != nil {
		logger.WithFields(logrus.Fields{
			"transport": "gRPC",
			"during":    "serve",
			"error":     err,
		}).Error("Unable to start hello service")
	}
}
