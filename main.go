package main

import (
	"context"
	"grpc-zipkin-with-grpc-gateway/middleware"
	pb "grpc-zipkin-with-grpc-gateway/pb"
	hello "grpc-zipkin-with-grpc-gateway/pkg"
	"net"
	"net/http"
	"os"
	"runtime"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	gruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	opentracing "github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	zipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
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
	logger := log.WithFields(logrus.Fields{"service": "hello"})

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		logger.WithFields(logrus.Fields{
			"transport": "gRPC",
			"during":    "Listen",
			"error":     err,
		}).Error("Unable to start grpc Listener")
	}

	reporter := zipkinhttp.NewReporter("http://localhost:9411/api/v2/spans")
	defer reporter.Close()

	// create our local service endpoint
	endpoint, err := zipkin.NewEndpoint("hello", "127.0.0.1:8080")
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
	var server *grpc.Server
	// grpc dial options to be used by REST proxy server
	var opts []grpc.DialOption
	server = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(tracer)),
		middleware.LoggingInterceptor(logger))), grpc.MaxSendMsgSize(50000000))
	opts = append(opts, grpc.WithInsecure())

	srv := hello.NewHelloService(logger)
	pb.RegisterHelloServer(server, srv)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	go func() {
		if err := server.Serve(lis); err != nil {
			logger.WithFields(logrus.Fields{
				"transport": "gRPC",
				"during":    "serve",
				"error":     err,
			}).Error("Unable to start hello gRPC service")
		}
	}()

	go func() {
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		mux := gruntime.NewServeMux()
		pb.RegisterHelloHandlerFromEndpoint(ctx, mux, ":8080", opts)
		if err = http.ListenAndServe(":8081", mux); err != nil {
			logger.WithFields(logrus.Fields{
				"transport": "REST",
				"during":    "Listen",
				"error":     err,
			}).Error("Unable to start hello REST server")
		}
	}()
	logger.WithFields(logrus.Fields{
		"transport":                 "gRPC",
		"service listening on port": ":8080",
	}).Info("hello gRPC service started")

	logger.WithFields(logrus.Fields{
		"transport":                      "REST",
		"REST gateway listening on port": ":8081",
	}).Info("hello REST service started")

	runtime.Goexit()
}
