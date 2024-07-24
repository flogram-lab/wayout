package main

import (
	"context"
	"fmt"
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
		os.Exit(1)
	}

	defer service.Close()

	err = service.Serve()
	if err != nil {
		bootstrap.Logger.Message(gelf.LOG_CRIT, "main", "rpc_service.Serve() returned with an error", map[string]any{
			"err": err,
		})
		LogErrorln("ERR: gRPC server failed to start")
		os.Exit(1)
	}

	// BEGIN queue

	bootstrap.Queue.Initialize(ctx)

	go bootstrap.Queue.Run()

	// BEGIN telegram

	if err := CreateAndRunTelegramClient(ctx, bootstrap); err != nil {
		bootstrap.Queue.Terminate()

		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
			bootstrap.Logger.Message(gelf.LOG_WARNING, "main", "Context cancelled (shutdown)")
			LogErrorln("\rContext cancelled. Done")
			os.Exit(0)
		}

		bootstrap.Logger.Message(gelf.LOG_CRIT, "main", "CreateAndRunTelegramClient() returned an error. "+
			"Exiting with zero for telegram account safety", map[string]any{
			"err": err,
		})

		_, _ = fmt.Fprintf(os.Stderr, "CreateAndRunTelegramClient: %+v\n", err)
		os.Exit(0)
	}

	bootstrap.Logger.Message(gelf.LOG_DEBUG, "main", "main() Done")
	os.Exit(0)
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
