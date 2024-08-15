package main

import (
	"context"
	"fmt"
	"log"
	"myorg/lib/otel"
	pb "myorg/lib/proto/gen/go/echo"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

const (
	grpcServerPort = 9091
	serviceName    = "serviceb"
)

type server struct {
	pb.UnimplementedEchoServiceServer
}

func (s *server) Echo(ctx context.Context, in *pb.StringMessage) (*pb.StringMessage, error) {
	otel.LogWithTraceID(ctx, "grpc request: "+in.String())
	return in, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, otelShutdown := otel.InitTracerMeter(ctx, serviceName)
	defer otelShutdown()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcServerPort))
	if err != nil {
		log.Fatalf("grpc failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterEchoServiceServer(s, &server{})
	log.Printf("grpc server listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("grpc failed to serve: %v", err)
	}
}
