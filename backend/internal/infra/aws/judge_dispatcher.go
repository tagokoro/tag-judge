package awsinfra

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type JudgeDispatcher struct {
	client    *sqs.Client
	queueName string
}

func NewJudgeDispatcher(ctx context.Context, region, endpointURL, queueName string) (*JudgeDispatcher, error) {
	cfg, err := loadConfig(ctx, region, endpointURL)
	if err != nil {
		return nil, err
	}

	client := sqs.NewFromConfig(cfg, func(options *sqs.Options) {
		if endpointURL != "" {
			options.BaseEndpoint = aws.String(endpointURL)
		}
	})

	return &JudgeDispatcher{client: client, queueName: queueName}, nil
}

func (d *JudgeDispatcher) Dispatch(ctx context.Context, job repository.JudgeJob) error {
	queue, err := d.client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(d.queueName),
	})
	if err != nil {
		return err
	}

	body, err := json.Marshal(judgeJobMessage{
		SubmissionID:  job.SubmissionID,
		Language:      string(job.Language),
		SourceCodeKey: job.SourceCodeKey,
		TestCasesKey:  job.TestCasesKey,
		TimeLimitMS:   job.TimeLimitMS,
	})
	if err != nil {
		return err
	}

	_, err = d.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    queue.QueueUrl,
		MessageBody: aws.String(string(body)),
	})
	return err
}

type judgeJobMessage struct {
	SubmissionID  string `json:"submissionId"`
	Language      string `json:"language"`
	SourceCodeKey string `json:"sourceCodeKey"`
	TestCasesKey  string `json:"testCasesKey"`
	TimeLimitMS   int    `json:"timeLimitMs"`
}
