package mcpserver

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type submitCodeInput struct {
	ProblemID  int64  `json:"problem_id"`
	SourceCode string `json:"source_code" jsonschema:"the Python 3 program, which reads standard input and writes standard output"`
}

type submissionIDInput struct {
	SubmissionID int64 `json:"submission_id"`
}

type caseResult struct {
	TestCaseID      int64  `json:"test_case_id"`
	Status          string `json:"status"`
	ExecutionTimeMS *int   `json:"execution_time_ms,omitempty"`
	MemoryKB        *int   `json:"memory_kb,omitempty"`
}

type submissionOutput struct {
	ID        int64  `json:"id"`
	ProblemID int64  `json:"problem_id"`
	Status    string `json:"status" jsonschema:"pending or running until judged, then the verdict"`
	// Judged is false while the verdict is pending; call get_submission later.
	Judged      bool         `json:"judged"`
	Score       int          `json:"score"`
	CreatedAt   time.Time    `json:"created_at"`
	JudgedAt    *time.Time   `json:"judged_at,omitempty"`
	CaseResults []caseResult `json:"case_results,omitempty"`
}

func addSubmissionTools(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "submit_code",
		Description: "Submit Python 3 code to a problem and wait briefly for the verdict. " +
			"If judging takes longer, the result has judged=false; call get_submission with its id later.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in submitCodeInput) (*mcp.CallToolResult, submissionOutput, error) {
		actor := actorFromContext(ctx)
		submission, err := deps.Submissions.Create(ctx, usecase.CreateSubmissionInput{
			Actor:      actor,
			ProblemID:  in.ProblemID,
			Language:   domain.SubmissionLanguagePython,
			SourceCode: in.SourceCode,
		})
		if err != nil {
			return nil, submissionOutput{}, toolError(ctx, err)
		}

		out := newSubmissionOutput(domain.SubmissionDetail{Submission: *submission})
		detail, err := waitForVerdict(ctx, deps, usecase.GetSubmissionInput{Actor: actor, SubmissionID: submission.ID})
		if err != nil {
			// The submission is accepted and will be judged; report it as
			// pending rather than failing the call.
			return nil, out, nil
		}
		return nil, newSubmissionOutput(*detail), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_submission",
		Description: "Get a submission's status, verdict, score, and per-case results.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in submissionIDInput) (*mcp.CallToolResult, submissionOutput, error) {
		detail, err := deps.Submissions.Get(ctx, usecase.GetSubmissionInput{
			Actor:        actorFromContext(ctx),
			SubmissionID: in.SubmissionID,
		})
		if err != nil {
			return nil, submissionOutput{}, toolError(ctx, err)
		}
		return nil, newSubmissionOutput(*detail), nil
	})
}

// waitForVerdict polls until the submission is judged, SubmitWait passes, or
// the client goes away, and returns the last state it read.
func waitForVerdict(ctx context.Context, deps Deps, input usecase.GetSubmissionInput) (*domain.SubmissionDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, deps.SubmitWait)
	defer cancel()
	ticker := time.NewTicker(deps.PollInterval)
	defer ticker.Stop()

	var last *domain.SubmissionDetail
	for {
		select {
		case <-ctx.Done():
			if last != nil {
				return last, nil
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
		detail, err := deps.Submissions.Get(ctx, input)
		if err != nil {
			if last != nil {
				return last, nil
			}
			return nil, err
		}
		last = detail
		if judged(detail.Status) {
			return detail, nil
		}
	}
}

func judged(status domain.SubmissionStatus) bool {
	return status != domain.SubmissionStatusPending && status != domain.SubmissionStatusRunning
}

func newSubmissionOutput(detail domain.SubmissionDetail) submissionOutput {
	out := submissionOutput{
		ID:        detail.ID,
		ProblemID: detail.ProblemID,
		Status:    string(detail.Status),
		Judged:    judged(detail.Status),
		Score:     detail.Score,
		CreatedAt: detail.CreatedAt,
		JudgedAt:  detail.JudgedAt,
	}
	for _, result := range detail.CaseResults {
		out.CaseResults = append(out.CaseResults, caseResult{
			TestCaseID:      result.TestCaseID,
			Status:          string(result.Status),
			ExecutionTimeMS: result.ExecutionTimeMS,
			MemoryKB:        result.MemoryKB,
		})
	}
	return out
}
