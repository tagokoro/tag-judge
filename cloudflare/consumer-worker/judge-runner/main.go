package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type testCase struct {
	TestCaseID     int64  `json:"testCaseId"`
	Score          int    `json:"score"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
}

type caseResult struct {
	TestCaseID int64  `json:"testCaseId"`
	CaseIndex  int    `json:"caseIndex"`
	Verdict    string `json:"verdict"`
	RuntimeMS  int64  `json:"runtimeMs"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Message    string `json:"message,omitempty"`
}

type judgeResult struct {
	Verdict     string       `json:"verdict"`
	Score       int          `json:"score"`
	CaseResults []caseResult `json:"caseResults"`
	Message     string       `json:"message,omitempty"`
}

func main() {
	language := flag.String("language", "", "execution language")
	sourcePath := flag.String("source", "", "source file path")
	testCasesPath := flag.String("testcases", "", "test cases JSON path")
	timeLimitMS := flag.Int("time-limit-ms", 1000, "time limit per test case")
	flag.Parse()

	result, err := run(*language, *sourcePath, *testCasesPath, *timeLimitMS)
	if err != nil {
		result = judgeResult{
			Verdict: "ie",
			Score:   0,
			Message: err.Error(),
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode result: %v\n", err)
		os.Exit(1)
	}
}

func run(language, sourcePath, testCasesPath string, timeLimitMS int) (judgeResult, error) {
	if language != "python" {
		return judgeResult{}, fmt.Errorf("unsupported language: %s", language)
	}
	if strings.TrimSpace(sourcePath) == "" || strings.TrimSpace(testCasesPath) == "" {
		return judgeResult{}, fmt.Errorf("source and testcases are required")
	}
	if timeLimitMS <= 0 {
		return judgeResult{}, fmt.Errorf("time limit must be positive")
	}

	testCasesBytes, err := os.ReadFile(testCasesPath)
	if err != nil {
		return judgeResult{}, fmt.Errorf("read testcases: %w", err)
	}
	var testCases []testCase
	if err := json.Unmarshal(testCasesBytes, &testCases); err != nil {
		return judgeResult{}, fmt.Errorf("parse testcases: %w", err)
	}
	if len(testCases) == 0 {
		return judgeResult{}, fmt.Errorf("testcases must not be empty")
	}

	results := make([]caseResult, 0, len(testCases))
	totalScore := 0
	for index, tc := range testCases {
		result := runCase(index, tc, sourcePath, time.Duration(timeLimitMS)*time.Millisecond)
		results = append(results, result)
		if result.Verdict == "ac" {
			totalScore += tc.Score
		}
	}

	verdict := "ac"
	score := totalScore
	for _, result := range results {
		if result.Verdict != "ac" {
			verdict = result.Verdict
			score = 0
			break
		}
	}

	return judgeResult{
		Verdict:     verdict,
		Score:       score,
		CaseResults: results,
	}, nil
}

func runCase(index int, tc testCase, sourcePath string, timeout time.Duration) caseResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", sourcePath)
	cmd.Stdin = strings.NewReader(tc.Input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startedAt := time.Now()
	err := cmd.Run()
	runtimeMS := time.Since(startedAt).Milliseconds()
	if ctx.Err() == context.DeadlineExceeded {
		return caseResult{
			TestCaseID: tc.TestCaseID,
			CaseIndex:  index,
			Verdict:    "tle",
			RuntimeMS:  runtimeMS,
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			Message:    "time limit exceeded",
		}
	}
	if err != nil {
		return caseResult{
			TestCaseID: tc.TestCaseID,
			CaseIndex:  index,
			Verdict:    "re",
			RuntimeMS:  runtimeMS,
			Stdout:     stdout.String(),
			Stderr:     stderr.String(),
			Message:    err.Error(),
		}
	}

	actual := normalizeOutput(stdout.String())
	expected := normalizeOutput(tc.ExpectedOutput)
	verdict := "ac"
	if actual != expected {
		verdict = "wa"
	}
	return caseResult{
		TestCaseID: tc.TestCaseID,
		CaseIndex:  index,
		Verdict:    verdict,
		RuntimeMS:  runtimeMS,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
	}
}

func normalizeOutput(value string) string {
	return strings.TrimRight(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
}
