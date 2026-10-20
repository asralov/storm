package main

import (
	"context"
	"github.com/asralov/storm/internal/app"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, app.Options{Port: 8080}); err != nil {
		log.Fatal(err)
	}
}
