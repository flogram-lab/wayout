package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc/peer"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type rpcCall struct {
	logger      Logger
	method      string
	peerAddress string
	ctx         context.Context
}

type rpcMethodHandler func(rpcCall) error

func (service rpcService) methodCallWrapper(method string, ctx context.Context, request any, handler rpcMethodHandler) error {
	peerAddress := ""
	if peer, ok := peer.FromContext(ctx); ok {
		peerAddress = peer.Addr.String()
	}

	logger := service.bootstrap.Logger.AddRequestID(fmt.Sprintf("rpc-%s", RandStringBytesMaskImprSrcSB(8)), map[string]any{
		"peer_addr": peerAddress,
		"service":   "flo_tg",
		"method":    method,
	})

	var err error

	defer LogPanicErr(&err, logger, "rpc_call_wrapper", "methodCallWrapper: "+method)

	logger.Message(gelf.LOG_INFO, "rpc_service", method+"() from peer: "+peerAddress, map[string]any{
		"debug_rpc_request": service.converter.encodeToJson(request, false),
	})

	defer ctx.Done()

	err = handler(rpcCall{
		logger:      logger,
		method:      method,
		peerAddress: peerAddress,
		ctx:         ctx,
	})

	if err != nil {
		logger.Message(gelf.LOG_INFO, "rpc_service", "Request "+method+" failed", map[string]any{
			"err": err,
		})
		return err
	}

	logger.Message(gelf.LOG_DEBUG, "rpc_service", "Request "+method+" completed")
	return err
}
