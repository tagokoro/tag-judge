package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

type GroupHandler struct {
	groups *usecase.GroupUsecase
}

func NewGroupHandler(groups *usecase.GroupUsecase) *GroupHandler {
	return &GroupHandler{groups: groups}
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST", "request body must be valid JSON"))
		return
	}

	group, err := h.groups.Create(r.Context(), actor, usecase.CreateGroupInput{
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newGroupResponse(*group))
}

func (h *GroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	groupID, err := parseGroupID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	group, err := h.groups.Get(r.Context(), actor, groupID)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newGroupResponse(*group))
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	groups, err := h.groups.List(r.Context(), actor)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]groupResponse, 0, len(groups))
	for _, group := range groups {
		response = append(response, newGroupResponse(group))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *GroupHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}
	groupID, err := parseGroupID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	var request struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST", "request body must be valid JSON"))
		return
	}

	member, err := h.groups.AddMember(r.Context(), actor, groupID, usecase.AddGroupMemberInput{
		UserID: request.UserID,
		Role:   domain.GroupRole(request.Role),
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newGroupMemberResponse(*member))
}

func parseGroupID(r *http.Request) (int64, error) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "group_id"), 10, 64)
	if err != nil {
		return 0, apperror.InvalidInput("INVALID_GROUP_ID", "group_id must be an integer")
	}
	return groupID, nil
}

func decodeJSONBody(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON object")
		}
		return err
	}
	return nil
}

type groupResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newGroupResponse(group domain.Group) groupResponse {
	return groupResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		CreatedBy:   group.CreatedBy,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}
}

type groupMemberResponse struct {
	GroupID   int64     `json:"group_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func newGroupMemberResponse(member domain.GroupMember) groupMemberResponse {
	return groupMemberResponse{
		GroupID:   member.GroupID,
		UserID:    member.UserID,
		Role:      string(member.Role),
		CreatedAt: member.CreatedAt,
	}
}
