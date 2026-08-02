package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	testOwnerID  = "00000000-0000-4000-8000-000000000001"
	testMemberID = "00000000-0000-4000-8000-000000000002"
)

func TestCreateGroup(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	group, err := uc.Create(context.Background(), testActor(domain.UserRoleAuthor), CreateGroupInput{
		Name:        "  Team Alpha  ",
		Description: "contest team",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if group.ID != 1 {
		t.Fatalf("group ID = %d, want 1", group.ID)
	}
	if repo.createParams.Name != "Team Alpha" {
		t.Fatalf("name = %q, want Team Alpha", repo.createParams.Name)
	}
	if repo.createParams.CreatedBy != testOwnerID {
		t.Fatalf("created_by = %q", repo.createParams.CreatedBy)
	}
}

func TestCreateGroupForbidden(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.Create(context.Background(), testActor(domain.UserRoleUser), CreateGroupInput{Name: "Team"})
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if repo.createCalled {
		t.Fatal("repository should not be called")
	}
}

func TestCreateGroupAdmin(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.Create(context.Background(), testActor(domain.UserRoleAdmin), CreateGroupInput{Name: "Team"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestCreateGroupValidation(t *testing.T) {
	tests := []CreateGroupInput{
		{Name: "   "},
		{Name: strings.Repeat("a", maxGroupNameLength+1)},
		{Name: "Team", Description: strings.Repeat("a", maxGroupDescriptionLength+1)},
	}

	for _, input := range tests {
		repo := &fakeGroupRepository{}
		uc := NewGroupUsecase(repo)
		_, err := uc.Create(context.Background(), testActor(domain.UserRoleAuthor), input)
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Fatalf("input = %+v, err = %v, want ErrInvalidInput", input, err)
		}
		if repo.createCalled {
			t.Fatal("repository should not be called")
		}
	}
}

func TestGetGroup(t *testing.T) {
	repo := &fakeGroupRepository{member: &domain.GroupMember{GroupID: 1, UserID: testOwnerID, Role: domain.GroupRoleMember}}
	uc := NewGroupUsecase(repo)

	group, err := uc.Get(context.Background(), testActor(domain.UserRoleUser), 1)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if group.ID != 1 || !repo.findMemberCalled || !repo.findCalled {
		t.Fatalf("unexpected access path: group=%+v member=%v find=%v", group, repo.findMemberCalled, repo.findCalled)
	}
}

func TestGetGroupForbidden(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.Get(context.Background(), testActor(domain.UserRoleUser), 1)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if !repo.findCalled {
		t.Fatal("group should be fetched")
	}
}

func TestGetGroupNotFound(t *testing.T) {
	repo := &fakeGroupRepository{groupErr: apperror.NotFound("GROUP_NOT_FOUND", "group not found")}
	uc := NewGroupUsecase(repo)

	_, err := uc.Get(context.Background(), testActor(domain.UserRoleUser), 1)
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetGroupAdmin(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.Get(context.Background(), testActor(domain.UserRoleAdmin), 1)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if repo.findMemberCalled {
		t.Fatal("admin membership should not be checked")
	}
}

func TestListGroups(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.List(context.Background(), testActor(domain.UserRoleAdmin))
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if repo.listUserID != testOwnerID {
		t.Fatalf("user ID = %q", repo.listUserID)
	}
}

func TestAddMember(t *testing.T) {
	repo := &fakeGroupRepository{member: &domain.GroupMember{GroupID: 1, UserID: testOwnerID, Role: domain.GroupRoleOwner}}
	uc := NewGroupUsecase(repo)

	member, err := uc.AddMember(context.Background(), testActor(domain.UserRoleAuthor), 1, AddGroupMemberInput{
		UserID: testMemberID,
		Role:   domain.GroupRoleMember,
	})
	if err != nil {
		t.Fatalf("AddMember returned error: %v", err)
	}
	if member.UserID != testMemberID || repo.addParams.GroupID != 1 {
		t.Fatalf("unexpected member: %+v", member)
	}
}

func TestAddMemberForbidden(t *testing.T) {
	repo := &fakeGroupRepository{member: &domain.GroupMember{GroupID: 1, UserID: testOwnerID, Role: domain.GroupRoleMember}}
	uc := NewGroupUsecase(repo)

	_, err := uc.AddMember(context.Background(), testActor(domain.UserRoleAuthor), 1, AddGroupMemberInput{
		UserID: testMemberID,
		Role:   domain.GroupRoleMember,
	})
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if repo.addCalled {
		t.Fatal("member should not be added")
	}
}

func TestAddMemberNoMembership(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.AddMember(context.Background(), testActor(domain.UserRoleAuthor), 1, AddGroupMemberInput{
		UserID: testMemberID,
		Role:   domain.GroupRoleMember,
	})
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if !repo.findCalled {
		t.Fatal("group should be fetched")
	}
	if repo.addCalled {
		t.Fatal("member should not be added")
	}
}

func TestAddMemberGroupNotFound(t *testing.T) {
	repo := &fakeGroupRepository{groupErr: apperror.NotFound("GROUP_NOT_FOUND", "group not found")}
	uc := NewGroupUsecase(repo)

	_, err := uc.AddMember(context.Background(), testActor(domain.UserRoleAuthor), 1, AddGroupMemberInput{
		UserID: testMemberID,
		Role:   domain.GroupRoleMember,
	})
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if repo.addCalled {
		t.Fatal("member should not be added")
	}
}

func TestAddMemberAdmin(t *testing.T) {
	repo := &fakeGroupRepository{}
	uc := NewGroupUsecase(repo)

	_, err := uc.AddMember(context.Background(), testActor(domain.UserRoleAdmin), 1, AddGroupMemberInput{
		UserID: testMemberID,
		Role:   domain.GroupRoleOwner,
	})
	if err != nil {
		t.Fatalf("AddMember returned error: %v", err)
	}
	if repo.findMemberCalled {
		t.Fatal("admin membership should not be checked")
	}
}

func TestAddMemberValidation(t *testing.T) {
	tests := []struct {
		groupID int64
		input   AddGroupMemberInput
	}{
		{groupID: 0, input: AddGroupMemberInput{UserID: testMemberID, Role: domain.GroupRoleMember}},
		{groupID: 1, input: AddGroupMemberInput{UserID: "invalid", Role: domain.GroupRoleMember}},
		{groupID: 1, input: AddGroupMemberInput{UserID: testMemberID, Role: domain.GroupRole("manager")}},
	}

	for _, test := range tests {
		repo := &fakeGroupRepository{}
		uc := NewGroupUsecase(repo)
		_, err := uc.AddMember(context.Background(), testActor(domain.UserRoleAdmin), test.groupID, test.input)
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Fatalf("input = %+v, err = %v, want ErrInvalidInput", test, err)
		}
		if repo.addCalled {
			t.Fatal("member should not be added")
		}
	}
}

func testActor(role domain.UserRole) auth.Actor {
	return auth.Actor{UserID: testOwnerID, Role: role}
}

type fakeGroupRepository struct {
	createCalled     bool
	createParams     repository.CreateGroupParams
	findCalled       bool
	groupErr         error
	listUserID       string
	findMemberCalled bool
	member           *domain.GroupMember
	addCalled        bool
	addParams        repository.AddGroupMemberParams
}

func (r *fakeGroupRepository) CreateWithOwner(_ context.Context, params repository.CreateGroupParams) (*domain.Group, error) {
	r.createCalled = true
	r.createParams = params
	return &domain.Group{ID: 1, Name: params.Name, Description: params.Description, CreatedBy: params.CreatedBy}, nil
}

func (r *fakeGroupRepository) FindByID(_ context.Context, id int64) (*domain.Group, error) {
	r.findCalled = true
	if r.groupErr != nil {
		return nil, r.groupErr
	}
	return &domain.Group{ID: id, Name: "Team"}, nil
}

func (r *fakeGroupRepository) ListByUserID(_ context.Context, userID string) ([]domain.Group, error) {
	r.listUserID = userID
	return []domain.Group{{ID: 1, Name: "Team"}}, nil
}

func (r *fakeGroupRepository) FindMember(_ context.Context, _ int64, _ string) (*domain.GroupMember, error) {
	r.findMemberCalled = true
	return r.member, nil
}

func (r *fakeGroupRepository) AddMember(_ context.Context, params repository.AddGroupMemberParams) (*domain.GroupMember, error) {
	r.addCalled = true
	r.addParams = params
	return &domain.GroupMember{GroupID: params.GroupID, UserID: params.UserID, Role: params.Role}, nil
}
