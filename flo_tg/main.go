package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/go-faster/errors"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

func main() {

	// BEGIN bootstrap

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	bootstrap := BootstrapFromEnvironment()
	defer bootstrap.Close()

	bootstrap.Logger.Message(gelf.LOG_INFO, "main", "Bootstrap OK, following launch sequence")

	// BEGIN rpc_service

	service := &rpcService{bootstrap: bootstrap}

	err := service.Init()
	
	if err != nil {
		bootstrap.Logger.Message(gelf.LOG_CRIT, "main", "rpc_service.Init() failed, RPC service cannot be started", map[string]any{
			"err": err,
		})
		LogErrorln("ERR: gRPC server failed to start")
		os.Exit(1) // FIXME: defer calls?
	}

	defer service.Close()

	go service.Serve()

	// BEGIN queue

	bootstrap.Queue.Initialize(ctx)

	go bootstrap.Queue.Run()

	// BEGIN telegram
	// TODO: make telegram goroutine, rpc_service synced

	err = CreateAndRunTelegramClient(ctx, bootstrap)

	if err != nil {
		bootstrap.Queue.Terminate()

		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
			LogErrorln("\rContext cancelled. Done")
			bootstrap.Logger.Message(gelf.LOG_WARNING, "main", "Context cancelled (shutdown)")
		} else {
			bootstrap.Logger.Message(gelf.LOG_CRIT, "main", "CreateAndRunTelegramClient() returned an error. "+
				"Exiting with zero for telegram account safety", map[string]any{
				"err": err,
			})
		}
	}
}

func sessionFolder(phone string) string {
	var out []rune
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	return "phone-" + string(out)
}
