package main

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/pkg/errors"
)

type rpcService struct {
	proto.UnimplementedFlotgServiceServer
	bootstrap Bootstrap
	listener  net.Listener
	server    *grpc.Server
}

func (service *rpcService) Close() error {
	if service.server != nil {
		service.server.Stop()
	}

	if service.listener != nil {
		if err := service.listener.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (service *rpcService) bind() error {
	var err error

	service.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", service.bootstrap.ServicePort))

	if err != nil {

		service.bootstrap.Logger.Message(gelf.LOG_ERR, "rpc_service", "Failed to net.Listen() on tcp port", map[string]any{
			"port": service.bootstrap.ServicePort,
			"err":  err,
		})

		return errors.Wrap(err, "failed to listen")
	}

	return nil
}

func (service *rpcService) run() error {
	var err error

	var opts []grpc.ServerOption
	service.server = grpc.NewServer(opts...)
	proto.RegisterFlotgServiceServer(service.server, service)

	service.bootstrap.Logger.Message(gelf.LOG_INFO, "rpc_service", "Running grpcServer.Serve()", map[string]any{
		"err":          err,
		"service_port": service.bootstrap.ServicePort,
	})

	err = service.server.Serve(service.listener)
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		service.bootstrap.Logger.Message(gelf.LOG_WARNING, "rpc_service", "grpcServer.Server() context cancelled", map[string]any{
			"err": err,
		})
		return nil
	}

	service.bootstrap.Logger.Message(gelf.LOG_ERR, "rpc_service", "grpcServer.Serve() returned with error", map[string]any{
		"err": err,
	})

	return errors.Wrap(err, "grpcServer.Serve() returned with error")
}

func (service rpcService) GetChats(request *proto.FlotgGetChatsRequest, stream proto.FlotgService_GetChatsServer) error {
	defer stream.Context().Done()
	return errors.New("not implemented function") // FIXME
}

func (service rpcService) SetMonitoring(ctx context.Context, request *proto.FlotgMonitor) (*proto.FlotgMonitor, error) {
	defer ctx.Done()
	return &proto.FlotgMonitor{}, errors.New("not implemented function") // FIXME
}

func (service rpcService) GetMessages(request *proto.FlotgGetMessagesRequest, stream proto.FlotgService_GetMessagesServer) error {
	defer stream.Context().Done()
	return errors.New("not implemented function") // FIXME
}
