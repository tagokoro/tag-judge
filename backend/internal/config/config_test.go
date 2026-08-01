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
