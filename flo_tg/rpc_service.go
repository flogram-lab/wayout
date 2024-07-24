package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil" // FIXME
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"

	"github.com/flogram-lab/wayout/flo_tg/proto"
	"github.com/pkg/errors"
)

const (
	serverCertFile   = "cert/server-cert.pem"
	serverKeyFile    = "cert/server-key.pem"
	clientCACertFile = "cert/ca-cert.pem"
)

type rpcService struct {
	proto.UnimplementedFlotgServiceServer
	bootstrap Bootstrap
	listener  net.Listener
	server    *grpc.Server
	opts      []grpc.ServerOption
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

	return err
}

func (service *rpcService) Serve() error {

	log.Printf("Start GRPC server at %s, TLS = %t", service.listener.Addr().String(), true)
	err := service.server.Serve(service.listener)

	if errors.Is(err, context.Canceled) {
		service.bootstrap.Logger.Message(gelf.LOG_WARNING, "rpc_service", "ListenAndServeTLS context cancelled", map[string]any{
			"err": err,
		})
		return nil
	}

	if err != nil {
		service.bootstrap.Logger.Message(gelf.LOG_ERR, "rpc_service", "ListenAndServeTLS returned with error", map[string]any{
			"err": err,
		})
		return err
	} else {
		service.bootstrap.Logger.Message(gelf.LOG_INFO, "rpc_service", "ListenAndServeTLS returned with no error", map[string]any{
			"err": err,
		})
		return nil
	}
}

func (service rpcService) Ready(ctx context.Context, request *emptypb.Empty) (*emptypb.Empty, error) {
	defer ctx.Done()

	// TODO: check if queue is initialized and healthy, and telegram client is still running.

	return &emptypb.Empty{}, nil // no error means ready
}

func (service rpcService) GetChats(request *proto.FlotgGetChatsRequest, stream proto.FlotgService_GetChatsServer) error {
	defer stream.Context().Done()
	return errors.New("REALLY not implemented function") // FIXME
}

func (service rpcService) SetMonitoring(ctx context.Context, request *proto.FlotgMonitor) (*proto.FlotgMonitor, error) {
	defer ctx.Done()
	return &proto.FlotgMonitor{}, errors.New("REALLY not implemented function") // FIXME
}

func (service rpcService) GetMessages(request *proto.FlotgGetMessagesRequest, stream proto.FlotgService_GetMessagesServer) error {
	defer stream.Context().Done()
	return errors.New("REALLY not implemented function") // FIXME
}
