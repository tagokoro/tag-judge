package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/config"
	"github.com/tagokoro/tag-judge/backend/internal/handler"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/router"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	problemRepo := postgres.NewProblemRepository(db)
	problemUsecase := usecase.NewProblemUsecase(problemRepo)
	problemHandler := handler.NewProblemHandler(problemUsecase)
	meHandler := handler.NewMeHandler()
	authMiddleware := appmiddleware.NewAuthMiddleware()

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: router.New(router.Handlers{
			Problems: problemHandler,
			Me:       meHandler,
			Auth:     authMiddleware,
		}),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api server listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
