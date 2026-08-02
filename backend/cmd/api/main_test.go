package main

import (
	"context"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/config"
	judgeinfra "github.com/tagokoro/tag-judge/backend/internal/infra/judge"
)

func TestBuildJudgeInfraUsesLocalAdaptersWhenAWSIsUnset(t *testing.T) {
	storage, dispatcher, err := buildJudgeInfra(context.Background(), config.Config{
		SubmissionSourceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("buildJudgeInfra returned error: %v", err)
	}
	if _, ok := storage.(*judgeinfra.FileSourceCodeStorage); !ok {
		t.Fatalf("storage type = %T, want *FileSourceCodeStorage", storage)
	}
	if _, ok := dispatcher.(*judgeinfra.LoggingDispatcher); !ok {
		t.Fatalf("dispatcher type = %T, want *LoggingDispatcher", dispatcher)
	}
}

func TestBuildJudgeInfraUsesLocalAdaptersWhenAWSIsPartial(t *testing.T) {
	storage, dispatcher, err := buildJudgeInfra(context.Background(), config.Config{
		SubmissionSourceRoot:  t.TempDir(),
		JudgeStorageBucket:    "judge-storage",
		JudgeSubmissionsQueue: "",
	})
	if err != nil {
		t.Fatalf("buildJudgeInfra returned error: %v", err)
	}
	if _, ok := storage.(*judgeinfra.FileSourceCodeStorage); !ok {
		t.Fatalf("storage type = %T, want *FileSourceCodeStorage", storage)
	}
	if _, ok := dispatcher.(*judgeinfra.LoggingDispatcher); !ok {
		t.Fatalf("dispatcher type = %T, want *LoggingDispatcher", dispatcher)
	}
}
