package usecase

import (
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

// Problem and submission access policy. A private problem is a draft: only its
// creator (and an admin) may see it, and only its creator may submit to it to
// check the test cases before publishing. Group membership is not modelled
// yet, so a group problem is treated like a private one. Whatever an actor
// may not see is reported as missing, so a hidden resource cannot be told
// apart from one that does not exist. An anonymous actor is the zero Actor.

func canViewProblem(actor auth.Actor, problem domain.Problem) bool {
	if problem.Visibility == domain.ProblemVisibilityPublic {
		return true
	}
	if actor.UserID == "" {
		return false
	}
	return problem.CreatedBy == actor.UserID || actor.Role == domain.UserRoleAdmin
}

func canSubmitProblem(actor auth.Actor, problem domain.Problem) bool {
	return problem.Visibility == domain.ProblemVisibilityPublic || problem.CreatedBy == actor.UserID
}

// canViewSubmission uses the visibility fixed when the submission was
// accepted, not the problem's current one.
func canViewSubmission(actor auth.Actor, submission domain.Submission) bool {
	if submission.IsPublic {
		return true
	}
	if actor.UserID == "" {
		return false
	}
	return submission.UserID == actor.UserID || actor.Role.CanViewAnySubmission()
}

func problemNotFound() error {
	return apperror.NotFound("PROBLEM_NOT_FOUND", "problem not found")
}

func submissionNotFound() error {
	return apperror.NotFound("SUBMISSION_NOT_FOUND", "submission not found")
}
