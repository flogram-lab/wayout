package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil" // FIXME
	"net"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/pkg/errors"
)

type rpcService struct {
	proto.UnimplementedFlotgServiceServer
	bootstrap Bootstrap
	listener  net.Listener
	server    *grpc.Server
	opts      []grpc.ServerOption

	converter *converter
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

func (service *rpcService) loadTLSCredentials() (credentials.TransportCredentials, error) {

	TLS_AUTHORITY := GetenvStr("TLS_AUTHORITY", "", false)

	service.bootstrap.Logger.Message(gelf.LOG_DEBUG, "rpc_service", "loadTLSCredentials", map[string]any{
		"TLS_AUTHORITY": TLS_AUTHORITY,
	})

	var (
		serverCertFile   = path.Join(TLS_AUTHORITY, "server-cert.pem")
		serverKeyFile    = path.Join(TLS_AUTHORITY, "server-key.pem")
		clientCACertFile = path.Join(TLS_AUTHORITY, "ca-cert.pem")
	)

	// Load certificate of the CA who signed client's certificate
	pemClientCA, err := ioutil.ReadFile(clientCACertFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, errors.New("failed to add client CA's certificate")
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(config), nil
}

func (service *rpcService) Init() error {
	tlsCredentials, err := service.loadTLSCredentials()
	if err != nil {
		return errors.Wrap(err, "cannot load TLS credentials: %w")
	}

	service.opts = []grpc.ServerOption{
		grpc.Creds(tlsCredentials),
	}

	service.server = grpc.NewServer(service.opts...)

	proto.RegisterFlotgServiceServer(service.server, service)
	reflection.Register(service.server)

	address := fmt.Sprintf(":%d", service.bootstrap.ServicePort)

	service.listener, err = net.Listen("tcp", address)

	service.converter = newConverter(service.bootstrap)

	return err
}

func (service *rpcService) Serve() {
	// Install panic handler with logging on this thread/goroutine
	defer LogPanic(service.bootstrap.Logger, "rpc_service")

	service.bootstrap.Logger.Message(gelf.LOG_INFO, "rpc_service", fmt.Sprintf("Running Serve(), listener at %s", service.listener.Addr().String()))

	err := service.server.Serve(service.listener)

	if errors.Is(err, context.Canceled) {
		service.bootstrap.Logger.Message(gelf.LOG_WARNING, "rpc_service", "Serve() context cancelled", map[string]any{
			"err": err,
		})
		return
	}

	if err != nil {
		service.bootstrap.Logger.Message(gelf.LOG_ERR, "rpc_service", "Serve() returned with error", map[string]any{
			"err": err,
		})
		return
	} else {
		service.bootstrap.Logger.Message(gelf.LOG_DEBUG, "rpc_service", "Serve() returned with no error", map[string]any{
			"err": err,
		})
		return
	}
}

func (service rpcService) Ready(ctx context.Context, request *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, service.methodCallWrapper("Ready", ctx, request, func(call rpcCall) error {
		// TODO: check telegram client is still running and healthy?
		if !service.bootstrap.Queue.IsReady() {
			return errors.New("not ready: queue")
		}
		// panic("testing panic 1")
		return nil
	})
}

func (service rpcService) GetSources(request *proto.FlotgGetSourcesRequest, stream proto.FlotgService_GetSourcesServer) error {
	return service.methodCallWrapper("GetSources", stream.Context(), request, func(call rpcCall) error {
		var (
			err    error
			result []storedSource
		)

		op := func(ctx context.Context) {
			read := storageRead{
				storage: service.bootstrap.Storage,
				logger:  call.logger,
			}

			// TODO: request flags check
			// FIXME: filter flags

			result, err = read.Sources(ctx, request.SourceUids...)
		}

		count := 0

		if !service.bootstrap.Queue.Join(call.ctx, time.Second*5, op) {
			return errors.New("queue join failed (overload?)")
		} else if err != nil {
			return errors.Wrap(err, "storage_read failed")
		} else if result == nil {
			return errors.New("storage read operation failed on backend (result is nil)")
		}

		for i := range result {
			err := stream.Send(result[i].Source)
			if err != nil {
				return errors.Wrap(err, "gRPC Stream.Send() fail")
			} else {
				count++
			}
		}

		call.logger.Message(gelf.LOG_DEBUG, "rpc_service", fmt.Sprintf("%s: streamed %d items", call.method, count))

		return nil
	})
}

func (service rpcService) GetMessages(request *proto.FlotgGetMessagesRequest, stream proto.FlotgService_GetMessagesServer) error {
	return service.methodCallWrapper("GetMessages", stream.Context(), request, func(call rpcCall) error {
		var (
			err    error
			result []storedMessage
		)

		op := func(ctx context.Context) {
			read := storageRead{
				storage: service.bootstrap.Storage,
				logger:  call.logger,
			}

			// TODO: request flags check
			// FIXME: filter flags

			result, err = read.Messages(call.ctx, request.SourceUid)
		}

		count := 0

		if !service.bootstrap.Queue.Join(call.ctx, time.Second*5, op) {
			return errors.New("queue join failed (overload?)")
		} else if err != nil {
			return errors.Wrap(err, "storage_read failed")
		} else if result == nil {
			return errors.New("storage read operation failed on backend (result is nil)")
		}

		for i := range result {
			err := stream.Send(result[i].Message)
			if err != nil {
				return errors.Wrap(err, "gRPC Stream.Send() fail")
			} else {
				count++
			}
		}

		call.logger.Message(gelf.LOG_DEBUG, "rpc_service", fmt.Sprintf("%s: streamed %d items", call.method, count))

		return nil
	})
}
