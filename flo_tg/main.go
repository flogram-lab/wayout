package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/go-faster/errors"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	bootstrap := BootstrapFromEnvironment()

	svc := &service{bootstrap: bootstrap}
	go svc.run()

	if err := CreateAndRunTelegramClient(ctx, bootstrap); err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
			log.Println("\rContext cancelled. Done")
			os.Exit(0)
		}
		_, _ = fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		os.Exit(1)
	}

	log.Println("main() Done")
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
