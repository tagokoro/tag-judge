package config

import "testing"

func TestLoadAWSJudgeInfraConfig(t *testing.T) {
	t.Setenv("AWS_DEFAULT_REGION", "us-west-2")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	t.Setenv("JUDGE_STORAGE_BUCKET", "judge-storage")
	t.Setenv("JUDGE_SUBMISSIONS_QUEUE_NAME", "judge-submissions")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AWSRegion != "us-west-2" {
		t.Fatalf("AWSRegion = %q, want us-west-2", cfg.AWSRegion)
	}
	if cfg.AWSEndpointURL != "http://localhost:4566" {
		t.Fatalf("AWSEndpointURL = %q", cfg.AWSEndpointURL)
	}
	if cfg.JudgeStorageBucket != "judge-storage" || cfg.JudgeSubmissionsQueue != "judge-submissions" {
		t.Fatalf("judge infra config = bucket %q queue %q", cfg.JudgeStorageBucket, cfg.JudgeSubmissionsQueue)
	}
	if !cfg.AWSJudgeInfraEnabled() {
		t.Fatal("AWSJudgeInfraEnabled = false, want true")
	}
}

func TestLoadAWSJudgeInfraDisabledByDefault(t *testing.T) {
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("JUDGE_STORAGE_BUCKET", "")
	t.Setenv("JUDGE_SUBMISSIONS_QUEUE_NAME", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AWSRegion != "ap-northeast-1" {
		t.Fatalf("AWSRegion = %q, want ap-northeast-1", cfg.AWSRegion)
	}
	if cfg.AWSJudgeInfraEnabled() {
		t.Fatal("AWSJudgeInfraEnabled = true, want false")
	}
}

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

func TestLoadAdmissionDefaults(t *testing.T) {
	t.Setenv("ADMISSION_USER_RATE_LIMIT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AdmissionUserRateLimit != 10 {
		t.Fatalf("AdmissionUserRateLimit = %d, want 10", cfg.AdmissionUserRateLimit)
	}
	if cfg.AdmissionUserRateWindowSeconds != 60 {
		t.Fatalf("AdmissionUserRateWindowSeconds = %d, want 60", cfg.AdmissionUserRateWindowSeconds)
	}
	if cfg.AdmissionUserOutstandingLimit != 2 {
		t.Fatalf("AdmissionUserOutstandingLimit = %d, want 2", cfg.AdmissionUserOutstandingLimit)
	}
	if cfg.AdmissionGlobalOutstandingLimit != 100 {
		t.Fatalf("AdmissionGlobalOutstandingLimit = %d, want 100", cfg.AdmissionGlobalOutstandingLimit)
	}
	if cfg.AdmissionLockTimeoutMS != 5000 {
		t.Fatalf("AdmissionLockTimeoutMS = %d, want 5000", cfg.AdmissionLockTimeoutMS)
	}
	if cfg.IdempotencyKeyMaxLength != 255 {
		t.Fatalf("IdempotencyKeyMaxLength = %d, want 255", cfg.IdempotencyKeyMaxLength)
	}
}

func TestLoadAdmissionCustomValues(t *testing.T) {
	t.Setenv("ADMISSION_USER_RATE_LIMIT", "5")
	t.Setenv("ADMISSION_USER_RATE_WINDOW_SECONDS", "30")
	t.Setenv("ADMISSION_USER_OUTSTANDING_LIMIT", "3")
	t.Setenv("ADMISSION_GLOBAL_OUTSTANDING_LIMIT", "50")
	t.Setenv("ADMISSION_LOCK_TIMEOUT_MS", "2000")
	t.Setenv("IDEMPOTENCY_KEY_MAX_LENGTH", "128")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AdmissionUserRateLimit != 5 {
		t.Fatalf("AdmissionUserRateLimit = %d, want 5", cfg.AdmissionUserRateLimit)
	}
	if cfg.AdmissionUserRateWindowSeconds != 30 {
		t.Fatalf("AdmissionUserRateWindowSeconds = %d, want 30", cfg.AdmissionUserRateWindowSeconds)
	}
	if cfg.AdmissionUserOutstandingLimit != 3 {
		t.Fatalf("AdmissionUserOutstandingLimit = %d, want 3", cfg.AdmissionUserOutstandingLimit)
	}
	if cfg.AdmissionGlobalOutstandingLimit != 50 {
		t.Fatalf("AdmissionGlobalOutstandingLimit = %d, want 50", cfg.AdmissionGlobalOutstandingLimit)
	}
	if cfg.AdmissionLockTimeoutMS != 2000 {
		t.Fatalf("AdmissionLockTimeoutMS = %d, want 2000", cfg.AdmissionLockTimeoutMS)
	}
	if cfg.IdempotencyKeyMaxLength != 128 {
		t.Fatalf("IdempotencyKeyMaxLength = %d, want 128", cfg.IdempotencyKeyMaxLength)
	}
}

func TestLoadAdmissionRejectsInvalidValues(t *testing.T) {
	t.Setenv("ADMISSION_USER_RATE_LIMIT", "0")
	t.Setenv("ADMISSION_USER_RATE_WINDOW_SECONDS", "-1")

	_, err := Load()
	if err == nil {
		t.Fatal("Load expected error for invalid admission values")
	}
}
