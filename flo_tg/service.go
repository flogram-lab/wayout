package main

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/flogram-lab/wayout/flo_tg/proto"
)

type service struct {
	proto.UnimplementedFlotgServiceServer
	bootstrap Bootstrap
}

func (s service) Chats(req *emptypb.Empty, srv proto.FlotgService_GetChatsServer) error {
	log.Println("Fetch data streaming")
	defer srv.Context().Done()
	return nil
}

func (s service) run() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.bootstrap.ServicePort))
	if err != nil {
		log.Fatalf("failed to listen on port 50051: %v", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterFlotgServiceServer(grpcServer, s)
	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
