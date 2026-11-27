// Package mcpserver exposes Tag Judge to MCP clients, such as Claude, over
// Streamable HTTP. Tools call the usecases directly, so they share the HTTP
// API's validation and authorization.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const instructions = `Tag Judge is an online judge for Python. Browse problems with list_problems and get_problem, submit Python 3 code with submit_code, and check a verdict with get_submission.
Authors can create a problem with create_problem, add test cases with add_test_case, and publish it with update_problem.
Verdicts: ac (accepted), wa (wrong answer), tle (time limit exceeded), mle (memory limit exceeded), re (runtime error), ce (compile error), ie (internal error).
Every tool acts as the Tag Judge user who approved this connection.`

type Deps struct {
	Problems    *usecase.ProblemUsecase
	TestCases   *usecase.TestCaseUsecase
	Submissions *usecase.SubmissionUsecase
	// SubmitWait bounds how long submit_code waits for a verdict before it
	// returns the pending submission. Keep it below the server's write timeout.
	SubmitWait   time.Duration
	PollInterval time.Duration
}

// NewHandler returns the /mcp endpoint. It is stateless, so any API instance
// can serve any request and nothing outlives a request.
func NewHandler(deps Deps) http.Handler {
	server := NewServer(deps)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}

func NewServer(deps Deps) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "tag-judge", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	addProblemTools(server, deps)
	addSubmissionTools(server, deps)
	return server
}

// actorFromContext returns who the MCP request acts for. The router admits
// only token-authenticated requests to /mcp, so a session actor never gets
// here; it is refused again in case the handler is mounted elsewhere.
func actorFromContext(ctx context.Context) auth.Actor {
	actor, ok := auth.ActorFromContext(ctx)
	if !ok || actor.Method == auth.MethodSession {
		return auth.Actor{}
	}
	return actor
}

// toolError turns a usecase error into the message the model reads. Internal
// errors are logged and kept vague, as the HTTP API does.
func toolError(ctx context.Context, err error) error {
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		slog.ErrorContext(ctx, "mcp tool failed", "error", err)
		return errors.New("INTERNAL_ERROR: internal server error")
	}
	message := fmt.Sprintf("%s: %s", appErr.Code, appErr.Message)
	if errors.Is(appErr, apperror.ErrUnauthorized) {
		message += " (reconnect Tag Judge to sign in again)"
	}
	if appErr.RetryAfter > 0 {
		message += fmt.Sprintf(" (retry after %d seconds)", appErr.RetryAfter)
	}
	return errors.New(message)
}
