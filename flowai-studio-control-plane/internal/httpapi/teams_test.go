package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/teams"
)

type stubTeamOperations struct {
	team            teams.Team
	list            []teams.Team
	member          teams.Member
	teamApplication teams.TeamApplication
	err             error
	memberInput     teams.AddMemberInput
	memberRole      rbac.TeamRole
	appInput        teams.AddApplicationInput
	appPermission   rbac.TeamAppPermission
}

func (operations *stubTeamOperations) Create(context.Context, string, teams.CreateInput) (teams.Team, error) {
	return operations.team, operations.err
}

func (operations *stubTeamOperations) List(context.Context, string) ([]teams.Team, error) {
	return operations.list, operations.err
}

func (operations *stubTeamOperations) Get(context.Context, string, string) (teams.Team, error) {
	return operations.team, operations.err
}

func (operations *stubTeamOperations) Update(context.Context, string, string, teams.UpdateInput) (teams.Team, error) {
	return operations.team, operations.err
}

func (operations *stubTeamOperations) Delete(context.Context, string, string) error {
	return operations.err
}

func (operations *stubTeamOperations) AddMember(_ context.Context, _, _ string, input teams.AddMemberInput) (teams.Member, error) {
	operations.memberInput = input
	return operations.member, operations.err
}

func (operations *stubTeamOperations) UpdateMemberRole(_ context.Context, _, _, _ string, role rbac.TeamRole) (teams.Member, error) {
	operations.memberRole = role
	member := operations.member
	member.Role = role
	return member, operations.err
}

func (operations *stubTeamOperations) RemoveMember(context.Context, string, string, string) error {
	return operations.err
}

func (operations *stubTeamOperations) Leave(context.Context, string, string) error {
	return operations.err
}

func (operations *stubTeamOperations) AddApplication(_ context.Context, _, _ string, input teams.AddApplicationInput) (teams.TeamApplication, error) {
	operations.appInput = input
	return operations.teamApplication, operations.err
}

func (operations *stubTeamOperations) UpdateApplicationPermission(_ context.Context, _, _, _ string, permission rbac.TeamAppPermission) (teams.TeamApplication, error) {
	operations.appPermission = permission
	application := operations.teamApplication
	application.Permission = permission
	return application, operations.err
}

func (operations *stubTeamOperations) RemoveApplication(context.Context, string, string, string) error {
	return operations.err
}

func teamRouter(operations TeamOperations) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterTeamRoutes(router, NewTeamHandler(operations), &stubTokenVerifier{principal: auth.Principal{
		UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice",
	}})
	return router
}

func sampleTeamHTTP() teams.Team {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	member := teams.Member{
		ID: "83a55a85-0c52-42f8-ac67-d6be4ba41be1", TeamID: "0b44ed01-327e-412a-a79e-63ef34d981fe",
		UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Role: rbac.TeamRoleOwner, JoinedAt: now,
	}
	application := teams.TeamApplication{
		ID: "f32384a3-22b1-4913-ab40-e68a980dafd9", TeamID: member.TeamID,
		ApplicationID: "7a611d9a-b555-4469-a289-f1672daefce3", Permission: rbac.TeamAppCanView, AddedAt: now,
	}
	return teams.Team{
		ID: member.TeamID, Name: "Team", OwnerID: member.UserID, MyRole: rbac.TeamRoleOwner,
		MemberCount: 1, AppCount: 1, Members: []teams.Member{member}, Applications: []teams.TeamApplication{application},
		CreatedAt: now, UpdatedAt: now,
	}
}

func TestTeamCRUDRoutesPreservePOSTAndPayloadContracts(t *testing.T) {
	team := sampleTeamHTTP()
	operations := &stubTeamOperations{team: team, list: []teams.Team{team}}
	router := teamRouter(operations)

	created := performJSONRequest(t, router, http.MethodPost, "/api/teams", `{"name":"Team"}`, "valid")
	if created.Code != http.StatusCreated || responseData(t, created)["ownerId"] != team.OwnerID {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}

	listed := performJSONRequest(t, router, http.MethodGet, "/api/teams", "", "valid")
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Code, listed.Body.String())
	}

	detail := performJSONRequest(t, router, http.MethodGet, "/api/teams/"+team.ID, "", "valid")
	if detail.Code != http.StatusOK || responseData(t, detail)["myRole"] != "owner" {
		t.Fatalf("detail status = %d, body = %s", detail.Code, detail.Body.String())
	}

	updated := performJSONRequest(t, router, http.MethodPatch, "/api/teams/"+team.ID, `{"description":null}`, "valid")
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updated.Code, updated.Body.String())
	}

	deleted := performJSONRequest(t, router, http.MethodDelete, "/api/teams/"+team.ID, "", "valid")
	if deleted.Code != http.StatusOK || responseData(t, deleted)["success"] != true {
		t.Fatalf("delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}
}

func TestTeamMemberRoutesPreservePOST201AndEnums(t *testing.T) {
	team := sampleTeamHTTP()
	member := team.Members[0]
	member.ID = "83a55a85-0c52-42f8-ac67-d6be4ba41be1"
	member.UserID = "4dc3923c-331f-4a80-9ea6-cf392e991821"
	member.Role = rbac.TeamRoleViewer
	operations := &stubTeamOperations{team: team, member: member}
	router := teamRouter(operations)

	added := performJSONRequest(t, router, http.MethodPost, "/api/teams/"+team.ID+"/members", `{"userId":"`+member.UserID+`","role":"viewer"}`, "valid")
	if added.Code != http.StatusCreated || operations.memberInput.Role != rbac.TeamRoleViewer || responseData(t, added)["userId"] != member.UserID {
		t.Fatalf("add status = %d, input = %#v, body = %s", added.Code, operations.memberInput, added.Body.String())
	}

	role := performJSONRequest(t, router, http.MethodPatch, "/api/teams/"+team.ID+"/members/"+member.ID, `{"role":"editor"}`, "valid")
	if role.Code != http.StatusOK || operations.memberRole != rbac.TeamRoleEditor || responseData(t, role)["role"] != "editor" {
		t.Fatalf("role status = %d, role = %q, body = %s", role.Code, operations.memberRole, role.Body.String())
	}

	removed := performJSONRequest(t, router, http.MethodDelete, "/api/teams/"+team.ID+"/members/"+member.ID, "", "valid")
	if removed.Code != http.StatusOK {
		t.Fatalf("remove status = %d", removed.Code)
	}

	left := performJSONRequest(t, router, http.MethodPost, "/api/teams/"+team.ID+"/leave", "", "valid")
	if left.Code != http.StatusCreated || responseData(t, left)["success"] != true {
		t.Fatalf("leave status = %d, body = %s", left.Code, left.Body.String())
	}
}

func TestTeamApplicationRoutesPreservePOST201AndPermissionPayloads(t *testing.T) {
	team := sampleTeamHTTP()
	application := team.Applications[0]
	operations := &stubTeamOperations{team: team, teamApplication: application}
	router := teamRouter(operations)

	added := performJSONRequest(t, router, http.MethodPost, "/api/teams/"+team.ID+"/apps", `{"applicationId":"`+application.ApplicationID+`","permission":"can_view"}`, "valid")
	if added.Code != http.StatusCreated || operations.appInput.Permission != rbac.TeamAppCanView {
		t.Fatalf("add status = %d, input = %#v, body = %s", added.Code, operations.appInput, added.Body.String())
	}

	updated := performJSONRequest(t, router, http.MethodPatch, "/api/teams/"+team.ID+"/apps/"+application.ID, `{"permission":"full_access"}`, "valid")
	if updated.Code != http.StatusOK || operations.appPermission != rbac.TeamAppFullAccess || responseData(t, updated)["permission"] != "full_access" {
		t.Fatalf("update status = %d, permission = %q, body = %s", updated.Code, operations.appPermission, updated.Body.String())
	}

	removed := performJSONRequest(t, router, http.MethodDelete, "/api/teams/"+team.ID+"/apps/"+application.ID, "", "valid")
	if removed.Code != http.StatusOK {
		t.Fatalf("remove status = %d", removed.Code)
	}
}

func TestTeamRoutesRequireJWTRejectUnknownFieldsAndMapErrors(t *testing.T) {
	team := sampleTeamHTTP()
	unauthorized := performJSONRequest(t, teamRouter(&stubTeamOperations{team: team}), http.MethodGet, "/api/teams", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	invalid := performJSONRequest(t, teamRouter(&stubTeamOperations{team: team}), http.MethodPost, "/api/teams", `{"name":"Team","ownerId":"other"}`, "valid")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}

	tests := []struct {
		kind   teams.ErrorKind
		status int
		code   string
	}{
		{kind: teams.ErrorInvalidInput, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{kind: teams.ErrorForbidden, status: http.StatusForbidden, code: "FORBIDDEN"},
		{kind: teams.ErrorNotFound, status: http.StatusNotFound, code: "NOT_FOUND"},
		{kind: teams.ErrorConflict, status: http.StatusConflict, code: "CONFLICT"},
		{kind: teams.ErrorInternal, status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}
	for _, test := range tests {
		operations := &stubTeamOperations{team: team, err: &teams.ServiceError{Kind: test.kind, Message: "public", Cause: errors.New("private")}}
		recorder := performJSONRequest(t, teamRouter(operations), http.MethodGet, "/api/teams/"+team.ID, "", "valid")
		if recorder.Code != test.status || !containsJSONCode(recorder.Body.Bytes(), test.code) {
			t.Fatalf("kind %s: status = %d, body = %s", test.kind, recorder.Code, recorder.Body.String())
		}
	}
}
