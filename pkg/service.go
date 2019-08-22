package hello

import (
	"context"
	"fmt"
	pb "grpc-zipkin-with-grpc-gateway/pb"

	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
)

type helloService struct {
	Logger *logrus.Entry
}

//NewHelloService is a service
func NewHelloService(l *logrus.Entry) pb.HelloServer {
	return helloService{Logger: l}
}
func (s helloService) SayHello(ctx context.Context, req *pb.HelloRequest) (res *pb.HelloResponse, err error) {
	res = &pb.HelloResponse{
		ResponseMessage: req.Message,
	}
	spanCtx := opentracing.SpanFromContext(ctx).Context().(zipkinot.SpanContext)
	traceID := spanCtx.TraceID.String()
	spanID := spanCtx.ID.String()
	defer func() {
		if r := recover(); r != nil {
			s.Logger.WithFields(logrus.Fields{
				"traceId": traceID,
				"spanId":  spanID,
				"error":   fmt.Sprintf("panic, no picnic: %s", r),
				"request": req,
			}).Error("Panic occurred in client request.")

		}
	}()
	return res, nil
}
