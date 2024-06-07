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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	bootstrap := BootstrapFromEnvironment()
	defer bootstrap.Close()

	service := &rpcService{bootstrap: bootstrap}

	err := service.bind()
	if err != nil {
		bootstrap.Logger.Message(gelf.LOG_CRIT, "main", "service.bind() failed, RPC service cannot be started", map[string]any{
			"err": err,
		})
		LogErrorln("ERR: gRPC server failed to start")
		os.Exit(1)
	}

	go service.run()

	if err := CreateAndRunTelegramClient(ctx, bootstrap); err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
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

	LogErrorln("main() Done")
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
