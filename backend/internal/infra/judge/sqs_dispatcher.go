package judgeinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type SQSDispatcher struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSDispatcher(ctx context.Context, cfg aws.Config, queueName string, endpointURL string) (*SQSDispatcher, error) {
	if strings.TrimSpace(queueName) == "" {
		return nil, fmt.Errorf("judge submissions queue name must not be empty")
	}

	client := sqs.NewFromConfig(cfg, func(options *sqs.Options) {
		if endpointURL != "" {
			options.BaseEndpoint = aws.String(endpointURL)
		}
	})
	response, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		return nil, fmt.Errorf("resolve judge submissions queue URL: %w", err)
	}
	if response.QueueUrl == nil || *response.QueueUrl == "" {
		return nil, fmt.Errorf("SQS get-queue-url response did not include queue URL")
	}

	return &SQSDispatcher{client: client, queueURL: *response.QueueUrl}, nil
}

func (d *SQSDispatcher) Dispatch(ctx context.Context, job repository.ExecutionJob) error {
	body, err := marshalExecutionJob(job)
	if err != nil {
		return err
	}

	_, err = d.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(d.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("send judge job to SQS: %w", err)
	}
	return nil
}

func marshalExecutionJob(job repository.ExecutionJob) ([]byte, error) {
	if err := job.Validate(); err != nil {
		return nil, err
	}

	reference := job.Reference()
	mode := job.Mode()
	message := executionJobMessage{
		Reference: executionReferenceMessage{
			Kind: reference.Kind(),
			ID:   reference.ID(),
		},
		ExecutionMode: string(mode.Kind()),
		Language:      job.Language(),
		SourceCodeKey: job.SourceCodeKey(),
	}
	switch mode.Kind() {
	case repository.ExecutionModeProblem:
		problem, _ := mode.Problem()
		message.SubmissionID = reference.ID()
		message.TestCasesKey = problem.TestCasesKey
		message.TimeLimitMS = problem.TimeLimitMS
		message.Problem = &problemExecutionMessage{
			TestCasesKey: problem.TestCasesKey,
			TimeLimitMS:  problem.TimeLimitMS,
		}
	case repository.ExecutionModeStdin:
		stdin, _ := mode.Stdin()
		message.CodeRunID = reference.ID()
		message.Stdin = stdin.Stdin
		message.TimeLimitMS = stdin.TimeLimitMS
		message.StdinExecution = &stdinExecutionMessage{
			Stdin:       stdin.Stdin,
			TimeLimitMS: stdin.TimeLimitMS,
		}
	}
	return json.Marshal(message)
}

type executionJobMessage struct {
	Reference      executionReferenceMessage `json:"reference"`
	ExecutionMode  string                    `json:"executionMode"`
	SubmissionID   string                    `json:"submissionId,omitempty"`
	CodeRunID      string                    `json:"codeRunId,omitempty"`
	Language       domain.ExecutionLanguage  `json:"language"`
	SourceCodeKey  string                    `json:"sourceCodeKey"`
	TestCasesKey   string                    `json:"testCasesKey,omitempty"`
	Stdin          string                    `json:"stdin,omitempty"`
	TimeLimitMS    int                       `json:"timeLimitMs"`
	Problem        *problemExecutionMessage  `json:"problemExecution,omitempty"`
	StdinExecution *stdinExecutionMessage    `json:"stdinExecution,omitempty"`
}

type executionReferenceMessage struct {
	Kind repository.ExecutionReferenceKind `json:"kind"`
	ID   string                            `json:"id"`
}

type problemExecutionMessage struct {
	TestCasesKey string `json:"testCasesKey"`
	TimeLimitMS  int    `json:"timeLimitMs"`
}

type stdinExecutionMessage struct {
	Stdin       string `json:"stdin"`
	TimeLimitMS int    `json:"timeLimitMs"`
}
