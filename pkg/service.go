package hello

import (
	"context"
	pb "grpc-zipkin-with-grpc-gateway/pb"

	"github.com/sirupsen/logrus"
)

type helloService struct {
	Logger *logrus.Entry
}

//NewHelloService is a service
func NewHelloService(l *logrus.Entry) pb.HelloServer {
	return helloService{Logger: l}
}
func (s helloService) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	res := &pb.HelloResponse{
		ResponseMessage: req.Message,
	}
	return res, nil
}
