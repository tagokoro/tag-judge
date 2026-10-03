package judgeinfra

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// This check is opt-in because LocalStack is not part of a normal go test run.
// It validates the actual S3 adapter used by the API against the local service.
func TestS3SourceCodeStorageWritesTestCaseSnapshotToLocalStack(t *testing.T) {
	if os.Getenv("LOCALSTACK_S3_INTEGRATION") != "1" {
		t.Skip("set LOCALSTACK_S3_INTEGRATION=1 to run against LocalStack")
	}

	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}
	bucket := os.Getenv("JUDGE_STORAGE_BUCKET")
	if bucket == "" {
		bucket = "judge-storage"
	}
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}

	cfg, err := LoadAWSConfig(context.Background(), region, endpoint)
	if err != nil {
		t.Fatalf("LoadAWSConfig returned error: %v", err)
	}
	storage := NewS3SourceCodeStorage(cfg, bucket, endpoint)
	params := repository.PutTestCasesParams{
		ProblemID: 42,
		TestCases: []domain.TestCase{{
			ID: 999001, ProblemID: 42, Input: "7 8\\n", ExpectedOutput: "15\\n", Score: 1,
		}},
	}
	key, err := storage.PutTestCases(context.Background(), params)
	if err != nil {
		t.Fatalf("PutTestCases returned error: %v", err)
	}
	object, err := storage.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject(%q) returned error: %v", key, err)
	}
	defer object.Body.Close()
	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	want := `[{"input":"7 8\\n","expectedOutput":"15\\n","testCaseId":999001,"score":1}]`
	if string(body) != want {
		t.Fatalf("S3 body = %s, want %s", body, want)
	}
	if object.ContentType == nil || *object.ContentType != "application/json; charset=utf-8" {
		t.Fatalf("ContentType = %v, want application/json; charset=utf-8", object.ContentType)
	}
	if key == "" {
		t.Fatal("snapshot key is empty")
	}

	// A later author edit creates another object and leaves the first
	// submission's referenced object unchanged.
	params.TestCases[0].ExpectedOutput = "16\\n"
	changedKey, err := storage.PutTestCases(context.Background(), params)
	if err != nil {
		t.Fatalf("PutTestCases(after edit) returned error: %v", err)
	}
	if changedKey == key {
		t.Fatalf("key after test case edit = %q, want a new content-addressed key", changedKey)
	}
}
