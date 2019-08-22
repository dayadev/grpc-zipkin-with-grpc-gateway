package main

import (
	"context"
	"fmt"
	"grpc-zipkin-with-grpc-gateway/middleware"
	pb "grpc-zipkin-with-grpc-gateway/pb"
	service "grpc-zipkin-with-grpc-gateway/pkg"
	"net"
	"net/http"
	"os"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	gruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/oklog/run"
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

	reporter := zipkinhttp.NewReporter("http://localhost:9411/api/v2/spans")
	defer reporter.Close()

	// create our local service endpoint
	endpoint, err := zipkin.NewEndpoint("hello", "hello:8080")
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
	srv := service.NewHelloService(logger)

	// grpc dial options to be used by REST proxy server
	var opts []grpc.DialOption
	server = grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(tracer)),
		middleware.LoggingInterceptor(logger))), grpc.MaxSendMsgSize(50000000))

	opts = append(opts, grpc.WithInsecure())

	pb.RegisterHelloServer(server, srv)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	var g run.Group
	{
		lis, err := net.Listen("tcp", ":8080")
		if err != nil {
			logger.WithFields(logrus.Fields{
				"transport": "gRPC",
				"during":    "Listen",
				"error":     err,
			}).Error("Unable to start grpc Listener")
		}
		g.Add(func() error {
			defer fmt.Printf("http.Serve returned\n")
			return server.Serve(lis)
		}, func(error) {
			lis.Close()
		})
	}
	{
		g.Add(func() error {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			mux := gruntime.NewServeMux()
			pb.RegisterHelloHandlerFromEndpoint(ctx, mux, ":8080", opts)
			return http.ListenAndServe(":8081", mux)
		}, func(error) {
			fmt.Println("error")
		})
	}
	fmt.Printf("The group was terminated with: %v\n", g.Run())
}
