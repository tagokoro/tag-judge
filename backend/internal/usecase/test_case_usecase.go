package usecase

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type TestCaseUsecase struct {
	problems  repository.ProblemRepository
	testCases repository.TestCaseRepository
}

func NewTestCaseUsecase(problems repository.ProblemRepository, testCases repository.TestCaseRepository) *TestCaseUsecase {
	return &TestCaseUsecase{problems: problems, testCases: testCases}
}

type CreateTestCaseInput struct {
	Input          string
	ExpectedOutput string
	IsSample       bool
}

type UpdateTestCaseInput struct {
	Input          *string
	ExpectedOutput *string
	IsSample       *bool
}

func (u *TestCaseUsecase) List(ctx context.Context, actor auth.Actor, problemID int64) ([]domain.TestCase, error) {
	if _, err := u.managedProblem(ctx, actor, problemID); err != nil {
		return nil, err
	}
	return u.testCases.ListByProblemID(ctx, problemID)
}

func (u *TestCaseUsecase) Create(ctx context.Context, actor auth.Actor, problemID int64, input CreateTestCaseInput) (*domain.TestCase, error) {
	if _, err := u.managedProblem(ctx, actor, problemID); err != nil {
		return nil, err
	}
	return u.testCases.Create(ctx, repository.CreateTestCaseParams{
		ProblemID:      problemID,
		Input:          input.Input,
		ExpectedOutput: input.ExpectedOutput,
		IsSample:       input.IsSample,
		Score:          1,
	})
}

func (u *TestCaseUsecase) Update(ctx context.Context, actor auth.Actor, problemID, id int64, input UpdateTestCaseInput) (*domain.TestCase, error) {
	if _, err := u.managedProblem(ctx, actor, problemID); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, apperror.InvalidInput("INVALID_TEST_CASE_ID", "test_case_id must be a positive integer")
	}
	if input.Input == nil && input.ExpectedOutput == nil && input.IsSample == nil {
		return nil, apperror.InvalidInput("EMPTY_UPDATE", "at least one field is required")
	}
	testCase, err := u.testCases.FindByID(ctx, problemID, id)
	if err != nil {
		return nil, err
	}
	updatedInput := testCase.Input
	updatedOutput := testCase.ExpectedOutput
	updatedSample := testCase.IsSample
	if input.Input != nil {
		updatedInput = *input.Input
	}
	if input.ExpectedOutput != nil {
		updatedOutput = *input.ExpectedOutput
	}
	if input.IsSample != nil {
		updatedSample = *input.IsSample
	}
	return u.testCases.Update(ctx, repository.UpdateTestCaseParams{
		ID:             id,
		ProblemID:      problemID,
		Input:          updatedInput,
		ExpectedOutput: updatedOutput,
		IsSample:       updatedSample,
	})
}

func (u *TestCaseUsecase) Delete(ctx context.Context, actor auth.Actor, problemID, id int64) error {
	problem, err := u.managedProblem(ctx, actor, problemID)
	if err != nil {
		return err
	}
	if id <= 0 {
		return apperror.InvalidInput("INVALID_TEST_CASE_ID", "test_case_id must be a positive integer")
	}
	if _, err := u.testCases.FindByID(ctx, problemID, id); err != nil {
		return err
	}
	if problem.Visibility == domain.ProblemVisibilityPublic {
		count, err := u.testCases.CountByProblemID(ctx, problemID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return apperror.Conflict("PUBLIC_PROBLEM_REQUIRES_TEST_CASE", "cannot delete the last test case from a public problem")
		}
	}
	return u.testCases.Delete(ctx, problemID, id)
}

func (u *TestCaseUsecase) managedProblem(ctx context.Context, actor auth.Actor, problemID int64) (*domain.Problem, error) {
	if err := validateProblemManager(actor); err != nil {
		return nil, err
	}
	if problemID <= 0 {
		return nil, apperror.InvalidInput("INVALID_PROBLEM_ID", "problem_id must be a positive integer")
	}
	problem, err := u.problems.FindByID(ctx, problemID)
	if err != nil {
		return nil, err
	}
	if err := authorizeProblemManager(actor, *problem); err != nil {
		return nil, err
	}
	return problem, nil
}
