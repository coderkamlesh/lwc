package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/akrylysov/algnhsa"
	"github.com/aws/aws-lambda-go/lambda"

	"example.com/go-lambda-monolith/internal/app"
	"example.com/go-lambda-monolith/internal/platform/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	application, err := app.New(initCtx, cfg)
	if err != nil {
		slog.Error("initialize application", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" || os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(algnhsa.New(application.Handler(), nil))
		return
	}

	serveHTTP(cfg, application.Handler())
}

func serveHTTP(cfg config.Config, handler http.Handler) {
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	case <-serverCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown http server", "error", err)
		}
	}
}
