package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tagokoro/tag-judge/internal/handler"
)

type Handlers struct {
	Problems *handler.ProblemHandler
}

func New(handlers Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/problems", func(r chi.Router) {
			r.Get("/", handlers.Problems.List)
			r.Get("/{problem_id}", handlers.Problems.Get)
		})
	})

	return r
}
