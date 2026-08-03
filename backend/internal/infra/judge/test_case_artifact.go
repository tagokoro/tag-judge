package judgeinfra

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type testCaseArtifact struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
}

func marshalTestCases(params repository.PutTestCasesParams) ([]byte, string, error) {
	if params.ProblemID <= 0 {
		return nil, "", fmt.Errorf("problem id must be positive")
	}
	if len(params.TestCases) == 0 {
		return nil, "", fmt.Errorf("test cases must not be empty")
	}

	artifact := make([]testCaseArtifact, 0, len(params.TestCases))
	for _, testCase := range params.TestCases {
		artifact = append(artifact, newTestCaseArtifact(testCase))
	}
	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, "", fmt.Errorf("marshal test cases: %w", err)
	}
	digest := sha256.Sum256(body)
	key := fmt.Sprintf("problems/%d/testcases/%x.json", params.ProblemID, digest)
	return body, key, nil
}

func newTestCaseArtifact(testCase domain.TestCase) testCaseArtifact {
	return testCaseArtifact{
		Input:          testCase.Input,
		ExpectedOutput: testCase.ExpectedOutput,
	}
}
