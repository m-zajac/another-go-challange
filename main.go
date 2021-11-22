package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var (
	addr = flag.String("addr", "127.0.0.1:8080", "the TCP address for the server to listen on, in the form 'host:port'")
)

const shutdownTimeout = 15 * time.Second

func main() {
	flag.Parse()

	service, err := NewDefaultService()
	if err != nil {
		log.Fatalf("failed to create service: %v", err)
	}
	handler := &Handler{
		service: service,
	}
	httpServer := http.Server{
		Addr:    *addr,
		Handler: handler,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("starting server on %s", *addr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed
	log.Print("server closed")
}
