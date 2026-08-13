package config

import "testing"

func TestLoadCodeRunTimeLimit(t *testing.T) {
	t.Setenv("CODE_RUN_TIME_LIMIT_MS", "2500")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.CodeRunTimeLimitMS != 2500 {
		t.Fatalf("CodeRunTimeLimitMS = %d, want 2500", cfg.CodeRunTimeLimitMS)
	}
}

func TestLoadCodeRunTimeLimitDefault(t *testing.T) {
	t.Setenv("CODE_RUN_TIME_LIMIT_MS", "invalid")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.CodeRunTimeLimitMS != 5000 {
		t.Fatalf("CodeRunTimeLimitMS = %d, want 5000", cfg.CodeRunTimeLimitMS)
	}
}

func TestLoadSourceCodeRoot(t *testing.T) {
	t.Setenv("SOURCE_CODE_ROOT", "/tmp/source-code")
	t.Setenv("SUBMISSION_SOURCE_ROOT", "/tmp/legacy-submissions")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SourceCodeRoot != "/tmp/source-code" {
		t.Fatalf("SourceCodeRoot = %q", cfg.SourceCodeRoot)
	}
}
