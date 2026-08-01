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

func (d *SQSDispatcher) Dispatch(ctx context.Context, job repository.JudgeJob) error {
	body, err := marshalJudgeJob(job)
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

func marshalJudgeJob(job repository.JudgeJob) ([]byte, error) {
	message := judgeJobMessage{
		SubmissionID:  job.SubmissionID,
		Language:      job.Language,
		SourceCodeKey: job.SourceCodeKey,
		TestCasesKey:  job.TestCasesKey,
		TimeLimitMS:   job.TimeLimitMS,
	}
	return json.Marshal(message)
}

type judgeJobMessage struct {
	SubmissionID  string                    `json:"submissionId"`
	Language      domain.SubmissionLanguage `json:"language"`
	SourceCodeKey string                    `json:"sourceCodeKey"`
	TestCasesKey  string                    `json:"testCasesKey"`
	TimeLimitMS   int                       `json:"timeLimitMs"`
}
