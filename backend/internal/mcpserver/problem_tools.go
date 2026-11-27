package mcpserver

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type listProblemsInput struct {
	Mine   bool `json:"mine,omitempty" jsonschema:"list the problems you created, private ones included, instead of public problems"`
	Limit  int  `json:"limit,omitempty" jsonschema:"number of problems to return, 1 to 100 (default 20)"`
	Offset int  `json:"offset,omitempty" jsonschema:"number of problems to skip"`
}

type problemSummary struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	TimeLimitMS   int    `json:"time_limit_ms"`
	MemoryLimitKB int    `json:"memory_limit_kb"`
	Visibility    string `json:"visibility"`
	CreatedBy     string `json:"created_by,omitempty" jsonschema:"the creator's handle"`
}

type listProblemsOutput struct {
	Problems []problemSummary `json:"problems"`
}

type problemIDInput struct {
	ProblemID int64 `json:"problem_id" jsonschema:"the problem's id"`
}

type problemSample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type problemOutput struct {
	problemSummary
	Statement string          `json:"statement"`
	Samples   []problemSample `json:"samples,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type createProblemInput struct {
	Title         string `json:"title"`
	Statement     string `json:"statement" jsonschema:"the problem statement in Markdown, including input and output formats and constraints"`
	TimeLimitMS   int    `json:"time_limit_ms" jsonschema:"time limit per test case in milliseconds, such as 2000"`
	MemoryLimitKB int    `json:"memory_limit_kb" jsonschema:"memory limit in KB, such as 262144"`
	Visibility    string `json:"visibility,omitempty" jsonschema:"private (default) or group; a problem becomes public with update_problem after it has a test case"`
}

type updateProblemInput struct {
	ProblemID     int64   `json:"problem_id"`
	Title         *string `json:"title,omitempty"`
	Statement     *string `json:"statement,omitempty"`
	TimeLimitMS   *int    `json:"time_limit_ms,omitempty"`
	MemoryLimitKB *int    `json:"memory_limit_kb,omitempty"`
	Visibility    *string `json:"visibility,omitempty" jsonschema:"public, private, or group; a public problem cannot be made private again"`
}

type addTestCaseInput struct {
	ProblemID      int64  `json:"problem_id"`
	Input          string `json:"input" jsonschema:"standard input for the case"`
	ExpectedOutput string `json:"expected_output" jsonschema:"the exact expected standard output"`
	IsSample       bool   `json:"is_sample,omitempty" jsonschema:"show this case on the problem page as a sample"`
}

type testCaseOutput struct {
	ID             int64  `json:"id"`
	ProblemID      int64  `json:"problem_id"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
	IsSample       bool   `json:"is_sample"`
}

type listTestCasesOutput struct {
	TestCases []testCaseOutput `json:"test_cases"`
}

func addProblemTools(server *mcp.Server, deps Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_problems",
		Description: "List public problems, or with mine=true the problems you created.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listProblemsInput) (*mcp.CallToolResult, listProblemsOutput, error) {
		input := usecase.ListProblemsInput{Limit: in.Limit, LimitProvided: in.Limit != 0, Offset: in.Offset}
		var problems []domain.Problem
		var err error
		if in.Mine {
			problems, err = deps.Problems.ListMyProblems(ctx, actorFromContext(ctx), input)
		} else {
			problems, err = deps.Problems.ListPublicProblems(ctx, input)
		}
		if err != nil {
			return nil, listProblemsOutput{}, toolError(ctx, err)
		}
		out := listProblemsOutput{Problems: make([]problemSummary, 0, len(problems))}
		for _, problem := range problems {
			out.Problems = append(out.Problems, newProblemSummary(problem))
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_problem",
		Description: "Get a problem's statement, limits, and sample cases.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in problemIDInput) (*mcp.CallToolResult, problemOutput, error) {
		detail, err := deps.Problems.GetProblem(ctx, actorFromContext(ctx), in.ProblemID)
		if err != nil {
			return nil, problemOutput{}, toolError(ctx, err)
		}
		out := newProblemOutput(detail.Problem)
		for _, sample := range detail.Samples {
			out.Samples = append(out.Samples, problemSample{Input: sample.Input, Output: sample.Output})
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_problem",
		Description: "Create a problem. Requires the author or admin role. Add test cases with add_test_case before anyone can submit.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createProblemInput) (*mcp.CallToolResult, problemOutput, error) {
		visibility := domain.ProblemVisibility(in.Visibility)
		if visibility == "" {
			visibility = domain.ProblemVisibilityPrivate
		}
		problem, err := deps.Problems.CreateProblem(ctx, actorFromContext(ctx), usecase.CreateProblemInput{
			Title:         in.Title,
			Statement:     in.Statement,
			TimeLimitMS:   in.TimeLimitMS,
			MemoryLimitKB: in.MemoryLimitKB,
			Visibility:    visibility,
		})
		if err != nil {
			return nil, problemOutput{}, toolError(ctx, err)
		}
		return nil, newProblemOutput(*problem), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_problem",
		Description: "Update a problem you manage. Only the given fields change. Set visibility to public to publish it.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateProblemInput) (*mcp.CallToolResult, problemOutput, error) {
		var visibility *domain.ProblemVisibility
		if in.Visibility != nil {
			v := domain.ProblemVisibility(*in.Visibility)
			visibility = &v
		}
		problem, err := deps.Problems.UpdateProblem(ctx, actorFromContext(ctx), in.ProblemID, usecase.UpdateProblemInput{
			Title:         in.Title,
			Statement:     in.Statement,
			TimeLimitMS:   in.TimeLimitMS,
			MemoryLimitKB: in.MemoryLimitKB,
			Visibility:    visibility,
		})
		if err != nil {
			return nil, problemOutput{}, toolError(ctx, err)
		}
		return nil, newProblemOutput(*problem), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_test_cases",
		Description: "List every test case of a problem you manage, hidden ones included.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in problemIDInput) (*mcp.CallToolResult, listTestCasesOutput, error) {
		testCases, err := deps.TestCases.List(ctx, actorFromContext(ctx), in.ProblemID)
		if err != nil {
			return nil, listTestCasesOutput{}, toolError(ctx, err)
		}
		out := listTestCasesOutput{TestCases: make([]testCaseOutput, 0, len(testCases))}
		for _, testCase := range testCases {
			out.TestCases = append(out.TestCases, newTestCaseOutput(testCase))
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_test_case",
		Description: "Add a test case to a problem you manage.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addTestCaseInput) (*mcp.CallToolResult, testCaseOutput, error) {
		testCase, err := deps.TestCases.Create(ctx, actorFromContext(ctx), in.ProblemID, usecase.CreateTestCaseInput{
			Input:          in.Input,
			ExpectedOutput: in.ExpectedOutput,
			IsSample:       in.IsSample,
		})
		if err != nil {
			return nil, testCaseOutput{}, toolError(ctx, err)
		}
		return nil, newTestCaseOutput(*testCase), nil
	})
}

func newProblemSummary(problem domain.Problem) problemSummary {
	return problemSummary{
		ID:            problem.ID,
		Title:         problem.Title,
		TimeLimitMS:   problem.TimeLimitMS,
		MemoryLimitKB: problem.MemoryLimitKB,
		Visibility:    string(problem.Visibility),
		CreatedBy:     problem.CreatedByHandle,
	}
}

func newProblemOutput(problem domain.Problem) problemOutput {
	return problemOutput{
		problemSummary: newProblemSummary(problem),
		Statement:      problem.Statement,
		CreatedAt:      problem.CreatedAt,
		UpdatedAt:      problem.UpdatedAt,
	}
}

func newTestCaseOutput(testCase domain.TestCase) testCaseOutput {
	return testCaseOutput{
		ID:             testCase.ID,
		ProblemID:      testCase.ProblemID,
		Input:          testCase.Input,
		ExpectedOutput: testCase.ExpectedOutput,
		IsSample:       testCase.IsSample,
	}
}
