package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strefa/internal/app"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	mode := "api"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "migrate":
		err = a.Migrate(ctx)
	case "worker":
		err = a.Worker(ctx)
	case "seed":
		err = a.Seed(ctx)
	default:
		err = a.Serve(ctx)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
