package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil" // FIXME
	"log"
	"net"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
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

	log.Printf("Start GRPC server at %s, TLS = %t", service.listener.Addr().String(), true)
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
	defer ctx.Done()

	// TODO: check if queue is initialized and healthy, and telegram client is still running.
	if !service.bootstrap.Queue.IsReady() {
		return &emptypb.Empty{}, errors.New("not ready: queue")
	}

	return &emptypb.Empty{}, nil
}

func (service rpcService) GetSources(request *proto.FlotgGetSourcesRequest, stream proto.FlotgService_GetSourcesServer) error {
	defer stream.Context().Done()

	const method = "GetSources"

	peerAddress := ""
	if peer, ok := peer.FromContext(stream.Context()); ok {
		peerAddress = peer.Addr.String()
	}

	logInfo := map[string]any{
		"debug_rpc_request": service.converter.encodeToJson(request, false),
		"peer_addr":         peerAddress,
		"service":           "flo_tg",
		"method":            method,
	}

	logger := service.bootstrap.Logger.AddRequestID(fmt.Sprintf("rpc-%s", RandStringBytesMaskImprSrcSB(8)))
	logger.Message(gelf.LOG_INFO, "rpc_service", method+"() from peer: "+peerAddress, logInfo)

	read := storageRead{
		storage: service.bootstrap.Storage,
		logger:  logger,
	}

	var result []storedSource
	var err error

	op := func(ctx context.Context) {
		result, err = read.Sources(ctx, request.SourceUids...)
	}

	if !service.bootstrap.Queue.Join(stream.Context(), time.Second*5, op) {
		logger.Message(gelf.LOG_WARNING, "rpc_service", "Queue join did not run", logInfo)
		return errors.New("queue is busy, try again")
	}

	if err != nil {
		logger.Message(gelf.LOG_ERR, "rpc_service", "storage_read.Sources fail", logInfo, map[string]any{
			"err":               err,
			"debug_rpc_request": service.converter.encodeToJson(request, false),
		})
		if result == nil {
			return errors.New("storage read operation failed on backend")
		}
	}

	for i := range result {
		err := stream.Send(result[i].Source)
		if err != nil {
			logger.Message(gelf.LOG_ERR, "rpc_service", "gRPC Stream.Send() fail", logInfo, map[string]any{
				"err": err,
			})
			return errors.New("streaming failed on backend")
		}
	}

	logger.Message(gelf.LOG_DEBUG, "rpc_service", "Request "+method+" completed", logInfo)

	return nil
}

func (service rpcService) GetMessages(request *proto.FlotgGetMessagesRequest, stream proto.FlotgService_GetMessagesServer) error {
	defer stream.Context().Done()

	const method = "GetMessages"

	peerAddress := ""
	if peer, ok := peer.FromContext(stream.Context()); ok {
		peerAddress = peer.Addr.String()
	}

	logInfo := map[string]any{
		"debug_rpc_request": service.converter.encodeToJson(request, false),
		"peer_addr":         peerAddress,
		"service":           "flo_tg",
		"method":            method,
	}

	logger := service.bootstrap.Logger.AddRequestID(fmt.Sprintf("rpc-%s", RandStringBytesMaskImprSrcSB(8)))
	logger.Message(gelf.LOG_INFO, "rpc_service", method+"() from peer: "+peerAddress, logInfo)

	read := storageRead{
		storage: service.bootstrap.Storage,
		logger:  logger,
	}

	var result []storedMessage
	var err error

	op := func(ctx context.Context) {
		result, err = read.Messages(stream.Context(), request.SourceUid)
	}

	if !service.bootstrap.Queue.Join(stream.Context(), time.Second*5, op) {
		logger.Message(gelf.LOG_WARNING, "rpc_service", "Queue join did not run", logInfo)
		return errors.New("queue is busy, try again")
	}

	if err != nil {
		logger.Message(gelf.LOG_ERR, "rpc_service", "storage_read.Messages fail", logInfo, map[string]any{
			"err":               err,
			"debug_rpc_request": service.converter.encodeToJson(request, false),
		})
		if result == nil {
			return errors.New("storage read operation failed on backend")
		}
	}

	for i := range result {
		err := stream.Send(result[i].Message)
		if err != nil {
			logger.Message(gelf.LOG_ERR, "rpc_service", "gRPC Stream.Send() fail", logInfo, map[string]any{
				"err": err,
			})
			return errors.New("streaming failed on backend")
		}
	}

	logger.Message(gelf.LOG_DEBUG, "rpc_service", "Request "+method+" completed", logInfo)

	return nil
}
