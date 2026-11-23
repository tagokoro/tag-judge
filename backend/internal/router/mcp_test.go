package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

func TestMCPListsTools(t *testing.T) {
	session := connectMCP(t, newTestRouter(), bearer(routerAccessToken))

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	var names []string
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{"list_problems", "get_problem", "create_problem", "update_problem", "list_test_cases", "add_test_case", "submit_code", "get_submission"} {
		if !slices.Contains(names, want) {
			t.Fatalf("tools = %v, missing %s", names, want)
		}
	}
}

func TestMCPGetProblemActsForTokenOwner(t *testing.T) {
	// fakeProblemRepository serves routerUserID's private problem, so only a
	// request acting for that user can see it.
	withToken := connectMCP(t, newTestRouter(), bearer(routerAccessToken))
	var problem struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Statement string `json:"statement"`
	}
	callTool(t, withToken, "get_problem", map[string]any{"problem_id": 1}, &problem)
	if problem.ID != 1 || problem.Title != "A + B" || problem.Statement != "Compute." {
		t.Fatalf("unexpected problem: %+v", problem)
	}

	other := connectMCP(t, newTestRouter(), func(req *http.Request) {
		setAuthAsUser(req, "00000000-0000-4000-8000-000000000009", "author")
	})
	if msg := callToolError(t, other, "get_problem", map[string]any{"problem_id": 1}); !strings.Contains(msg, "PROBLEM_NOT_FOUND") {
		t.Fatalf("other user's error = %q, want PROBLEM_NOT_FOUND", msg)
	}
}

func TestMCPRefusesWithoutToken(t *testing.T) {
	cases := map[string]func(*http.Request){
		"anonymous": nil,
		// A browser session must not drive the tools with the user's cookie.
		"session cookie": func(req *http.Request) {
			req.AddCookie(&http.Cookie{Name: "tag_judge_session", Value: "valid-session"})
		},
		"revoked token": bearer("tja_revoked"),
	}
	for name, edit := range cases {
		server := httptest.NewServer(newTestRouter())
		t.Cleanup(server.Close)
		httpClient := http.DefaultClient
		if edit != nil {
			httpClient = &http.Client{Transport: headerTransport{edit: edit}}
		}
		client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)

		_, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
			Endpoint:   server.URL + "/mcp",
			HTTPClient: httpClient,
			MaxRetries: -1,
		}, nil)
		if err == nil {
			t.Fatalf("%s: connect succeeded", name)
		}
	}
}

func TestMCPSubmitCodeWaitsForVerdict(t *testing.T) {
	judgedAt := time.Now()
	timeMS := 12
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{
		submission: &domain.Submission{ID: 1, UserID: routerUserID, ProblemID: 1, Status: domain.SubmissionStatusWA, Score: 1, JudgedAt: &judgedAt},
		caseResults: []domain.SubmissionCaseResult{
			{TestCaseID: 10, Status: domain.SubmissionStatusAC, ExecutionTimeMS: &timeMS},
			{TestCaseID: 11, Status: domain.SubmissionStatusWA},
		},
	})
	options.mcpSubmitWait = time.Second
	session := connectMCP(t, newTestRouterWithHandlers(options), bearer(routerAccessToken))

	var result struct {
		ID          int64  `json:"id"`
		Status      string `json:"status"`
		Judged      bool   `json:"judged"`
		Score       int    `json:"score"`
		CaseResults []struct {
			TestCaseID      int64  `json:"test_case_id"`
			Status          string `json:"status"`
			ExecutionTimeMS *int   `json:"execution_time_ms"`
		} `json:"case_results"`
	}
	callTool(t, session, "submit_code", map[string]any{"problem_id": 1, "source_code": "print(1)"}, &result)

	if result.ID != 1 || result.Status != "wa" || !result.Judged || result.Score != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.CaseResults) != 2 || result.CaseResults[1].Status != "wa" || *result.CaseResults[0].ExecutionTimeMS != 12 {
		t.Fatalf("case results = %+v", result.CaseResults)
	}
}

func TestMCPSubmitCodeReturnsPendingAfterWait(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{
		submission: &domain.Submission{ID: 1, UserID: routerUserID, ProblemID: 1, Status: domain.SubmissionStatusRunning},
	})
	options.mcpSubmitWait = 20 * time.Millisecond
	session := connectMCP(t, newTestRouterWithHandlers(options), bearer(routerAccessToken))

	var result struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
		Judged bool   `json:"judged"`
	}
	callTool(t, session, "submit_code", map[string]any{"problem_id": 1, "source_code": "print(1)"}, &result)

	if result.ID != 1 || result.Status != "running" || result.Judged {
		t.Fatalf("result = %+v, want the running submission", result)
	}
}

func TestMCPCreateProblemRequiresAuthorRole(t *testing.T) {
	session := connectMCP(t, newTestRouter(), func(req *http.Request) { setAuth(req, "user") })

	msg := callToolError(t, session, "create_problem", map[string]any{
		"title": "A + B", "statement": "Add.", "time_limit_ms": 1000, "memory_limit_kb": 262144,
	})
	if !strings.Contains(msg, "FORBIDDEN") {
		t.Fatalf("error = %q, want FORBIDDEN", msg)
	}
}

func TestMCPCreateProblemDefaultsToPrivate(t *testing.T) {
	session := connectMCP(t, newTestRouter(), bearer(routerAccessToken))

	var problem struct {
		ID         int64  `json:"id"`
		Visibility string `json:"visibility"`
	}
	callTool(t, session, "create_problem", map[string]any{
		"title": "A + B", "statement": "Add.", "time_limit_ms": 1000, "memory_limit_kb": 262144,
	}, &problem)
	if problem.ID != 42 || problem.Visibility != "private" {
		t.Fatalf("problem = %+v, want a private problem", problem)
	}
}

func bearer(token string) func(*http.Request) {
	return func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+token) }
}

type headerTransport struct {
	edit func(*http.Request)
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	t.edit(req)
	return http.DefaultTransport.RoundTrip(req)
}

// connectMCP connects an MCP client to router's /mcp; edit, when set, adds
// credentials to every request.
func connectMCP(t *testing.T, router http.Handler, edit func(*http.Request)) *mcp.ClientSession {
	t.Helper()
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	httpClient := http.DefaultClient
	if edit != nil {
		httpClient = &http.Client{Transport: headerTransport{edit: edit}}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   server.URL + "/mcp",
		HTTPClient: httpClient,
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s failed: %s", name, toolText(result))
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("%s: marshal structured content: %v", name, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: decode structured content: %v", name, err)
	}
}

func callToolError(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !result.IsError {
		t.Fatalf("%s succeeded, want a tool error", name)
	}
	return toolText(result)
}

func toolText(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}
