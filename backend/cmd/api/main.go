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
	judgeinfra "github.com/tagokoro/tag-judge/backend/internal/infra/judge"
	"github.com/tagokoro/tag-judge/backend/internal/infra/oauth"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres"
	appmiddleware "github.com/tagokoro/tag-judge/backend/internal/middleware"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
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
	testCaseRepo := postgres.NewTestCaseRepository(db)
	codeRunRepo := postgres.NewCodeRunRepository(db)
	submissionRepo := postgres.NewSubmissionRepository(db)
	authRepo := postgres.NewAuthRepository(db)
	groupRepo := postgres.NewGroupRepository(db)
	sourceCodeStorage, executionDispatcher, err := buildJudgeInfra(ctx, cfg)
	if err != nil {
		return err
	}
	googleProvider := oauth.NewGoogleProvider(
		cfg.GoogleOAuthClientID,
		cfg.GoogleOAuthSecret,
		cfg.GoogleOAuthRedirectURL,
	)

	problemUsecase := usecase.NewProblemUsecase(problemRepo, testCaseRepo)
	testCaseUsecase := usecase.NewTestCaseUsecase(problemRepo, testCaseRepo)
	codeRunUsecase := usecase.NewCodeRunUsecase(codeRunRepo, sourceCodeStorage, executionDispatcher, cfg.CodeRunTimeLimitMS)
	submissionUsecase := usecase.NewSubmissionUsecase(submissionRepo, problemRepo, testCaseRepo, sourceCodeStorage, executionDispatcher)
	authUsecase := usecase.NewAuthUsecase(authRepo, googleProvider, cfg.SessionTTL)
	groupUsecase := usecase.NewGroupUsecase(groupRepo)

	problemHandler := handler.NewProblemHandler(problemUsecase)
	testCaseHandler := handler.NewTestCaseHandler(testCaseUsecase)
	codeRunHandler := handler.NewCodeRunHandler(codeRunUsecase)
	submissionHandler := handler.NewSubmissionHandler(submissionUsecase)
	groupHandler := handler.NewGroupHandler(groupUsecase)
	meHandler := handler.NewMeHandler()
	authHandler := handler.NewAuthHandler(authUsecase, cfg.CookieSecure, cfg.AuthSuccessRedirectURL)
	authMiddleware := appmiddleware.NewAuthMiddleware(authRepo, cfg.DevHeaderAuth)

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: router.New(router.Handlers{
			Problems:    problemHandler,
			TestCases:   testCaseHandler,
			CodeRuns:    codeRunHandler,
			Groups:      groupHandler,
			Submissions: submissionHandler,
			Me:          meHandler,
			AuthHTTP:    authHandler,
			Auth:        authMiddleware,
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

func buildJudgeInfra(ctx context.Context, cfg config.Config) (repository.ArtifactStorage, repository.ExecutionDispatcher, error) {
	if !cfg.AWSJudgeInfraEnabled() {
		if cfg.JudgeStorageBucket != "" || cfg.JudgeSubmissionsQueue != "" {
			slog.Warn(
				"AWS judge infra is partially configured; using local judge adapters",
				"bucket_configured", cfg.JudgeStorageBucket != "",
				"queue_configured", cfg.JudgeSubmissionsQueue != "",
			)
		}
		return judgeinfra.NewFileSourceCodeStorage(cfg.SourceCodeRoot),
			judgeinfra.NewLoggingDispatcher(slog.Default()),
			nil
	}

	awsConfig, err := judgeinfra.LoadAWSConfig(ctx, cfg.AWSRegion, cfg.AWSEndpointURL)
	if err != nil {
		return nil, nil, err
	}
	dispatcher, err := judgeinfra.NewSQSDispatcher(ctx, awsConfig, cfg.JudgeSubmissionsQueue, cfg.AWSEndpointURL)
	if err != nil {
		return nil, nil, err
	}
	slog.Info(
		"using AWS judge infra adapters",
		"region", cfg.AWSRegion,
		"endpoint_configured", cfg.AWSEndpointURL != "",
		"bucket", cfg.JudgeStorageBucket,
		"queue", cfg.JudgeSubmissionsQueue,
	)
	return judgeinfra.NewS3SourceCodeStorage(awsConfig, cfg.JudgeStorageBucket, cfg.AWSEndpointURL), dispatcher, nil
}
