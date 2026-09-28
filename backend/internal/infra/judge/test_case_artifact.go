package judgeinfra

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// testCaseArtifact contains the Worker TestCase fields first.  serde ignores
// the database identity and score today, while retaining them makes a snapshot
// self-describing for result persistence added later.
type testCaseArtifact struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
	TestCaseID     int64  `json:"testCaseId"`
	Score          int    `json:"score"`
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
		if testCase.ID <= 0 || testCase.Score <= 0 {
			return nil, "", fmt.Errorf("test case id and score must be positive")
		}
		artifact = append(artifact, testCaseArtifact{
			Input:          testCase.Input,
			ExpectedOutput: testCase.ExpectedOutput,
			TestCaseID:     testCase.ID,
			Score:          testCase.Score,
		})
	}

	body, err := json.Marshal(artifact)
	if err != nil {
		return nil, "", fmt.Errorf("marshal test cases: %w", err)
	}
	digest := sha256.Sum256(body)
	return body, fmt.Sprintf("problems/%d/testcases/%x.json", params.ProblemID, digest), nil
}
