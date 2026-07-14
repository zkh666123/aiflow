package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/shares"
)

type stubShareOperations struct {
	share       shares.Share
	public      shares.PublicApplication
	embed       shares.EmbedCode
	err         error
	userID      string
	appID       string
	shareLink   string
	updateInput shares.UpdateInput
	revoked     bool
}

func (operations *stubShareOperations) Generate(_ context.Context, userID, appID string) (shares.Share, error) {
	operations.userID, operations.appID = userID, appID
	return operations.share, operations.err
}

func (operations *stubShareOperations) Get(_ context.Context, userID, appID string) (shares.Share, error) {
	operations.userID, operations.appID = userID, appID
	return operations.share, operations.err
}

func (operations *stubShareOperations) Update(_ context.Context, userID, appID string, input shares.UpdateInput) (shares.Share, error) {
	operations.userID, operations.appID, operations.updateInput = userID, appID, input
	return operations.share, operations.err
}

func (operations *stubShareOperations) Revoke(_ context.Context, userID, appID string) error {
	operations.userID, operations.appID, operations.revoked = userID, appID, true
	return operations.err
}

func (operations *stubShareOperations) Embed(_ context.Context, userID, appID string) (shares.EmbedCode, error) {
	operations.userID, operations.appID = userID, appID
	return operations.embed, operations.err
}

func (operations *stubShareOperations) GetPublic(_ context.Context, shareLink string) (shares.PublicApplication, error) {
	operations.shareLink = shareLink
	return operations.public, operations.err
}

func shareRouter(operations ShareOperations) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterShareRoutes(router, NewShareHandler(operations), &stubTokenVerifier{principal: auth.Principal{
		UserID: "e9f6332d-da39-44b2-917c-da5ff30aca9d", Username: "alice",
	}})
	return router
}

func sampleShareHTTP() shares.Share {
	now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	showHeader := true
	return shares.Share{
		ID: "516f794a-eb09-4e7d-bdbd-d4e2bc5f14da", ApplicationID: "7a611d9a-b555-4469-a289-f1672daefce3",
		ShareLink: "share-00112233445566778899aabbccddeeff", IsPublic: true, AccessCount: 3,
		EmbedConfig: &shares.EmbedConfig{Theme: shares.ThemeAuto, Width: "100%", Height: "600px", ShowHeader: &showHeader},
		CreatedAt:   now, UpdatedAt: now,
	}
}

func TestShareRoutesPreserveManagementAndPublicContracts(t *testing.T) {
	share := sampleShareHTTP()
	operations := &stubShareOperations{
		share: share,
		public: shares.PublicApplication{
			ID: share.ID, ApplicationID: share.ApplicationID, ShareLink: share.ShareLink, IsPublic: true, Name: "Demo", Status: "published",
		},
		embed: shares.EmbedCode{
			ShareURL:   "http://127.0.0.1:5173/share/" + share.ShareLink,
			IframeCode: "<iframe></iframe>", ScriptTag: "<script></script>", ScriptCode: "<script></script>",
		},
	}
	router := shareRouter(operations)

	created := performJSONRequest(t, router, http.MethodPost, "/api/apps/"+share.ApplicationID+"/share", "", "valid")
	if created.Code != http.StatusCreated || responseData(t, created)["shareLink"] != share.ShareLink {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}

	got := performJSONRequest(t, router, http.MethodGet, "/api/apps/"+share.ApplicationID+"/share", "", "valid")
	if got.Code != http.StatusOK || responseData(t, got)["accessCount"] != float64(3) {
		t.Fatalf("get status = %d, body = %s", got.Code, got.Body.String())
	}

	updated := performJSONRequest(t, router, http.MethodPatch, "/api/apps/"+share.ApplicationID+"/share", `{
		"isPublic":false,
		"embedConfig":{"enabled":true,"width":"100%","height":"600px","theme":"dark","showHeader":false,"allowedOrigins":["https://example.com"]}
	}`, "valid")
	if updated.Code != http.StatusOK || !operations.updateInput.SetIsPublic || operations.updateInput.IsPublic ||
		!operations.updateInput.SetEmbedConfig || operations.updateInput.EmbedConfig.Theme != shares.ThemeDark {
		t.Fatalf("update status = %d, input = %#v, body = %s", updated.Code, operations.updateInput, updated.Body.String())
	}

	embed := performJSONRequest(t, router, http.MethodGet, "/api/apps/"+share.ApplicationID+"/embed", "", "valid")
	if embed.Code != http.StatusOK || responseData(t, embed)["scriptCode"] != "<script></script>" || responseData(t, embed)["scriptTag"] != "<script></script>" {
		t.Fatalf("embed status = %d, body = %s", embed.Code, embed.Body.String())
	}

	deleted := performJSONRequest(t, router, http.MethodDelete, "/api/apps/"+share.ApplicationID+"/share", "", "valid")
	if deleted.Code != http.StatusOK || responseData(t, deleted)["success"] != true || !operations.revoked {
		t.Fatalf("delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}

	public := performJSONRequest(t, router, http.MethodGet, "/api/share/"+share.ShareLink, "", "")
	if public.Code != http.StatusOK || responseData(t, public)["id"] != share.ApplicationID || operations.shareLink != share.ShareLink {
		t.Fatalf("public status = %d, body = %s", public.Code, public.Body.String())
	}
}

func TestShareManagementRequiresJWTAndStrictInput(t *testing.T) {
	share := sampleShareHTTP()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
	}{
		{name: "JWT required", method: http.MethodGet, path: "/api/apps/" + share.ApplicationID + "/share"},
		{name: "invalid app UUID", method: http.MethodPost, path: "/api/apps/not-a-uuid/share", token: "valid"},
		{name: "unknown setting", method: http.MethodPatch, path: "/api/apps/" + share.ApplicationID + "/share", body: `{"isPublic":true,"rawHtml":"x"}`, token: "valid"},
		{name: "missing setting", method: http.MethodPatch, path: "/api/apps/" + share.ApplicationID + "/share", body: `{}`, token: "valid"},
	}
	for _, test := range tests {
		recorder := performJSONRequest(t, shareRouter(&stubShareOperations{share: share}), test.method, test.path, test.body, test.token)
		want := http.StatusBadRequest
		if test.name == "JWT required" {
			want = http.StatusUnauthorized
		}
		if recorder.Code != want {
			t.Fatalf("%s: status = %d, body = %s", test.name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestShareRoutesMapServiceErrors(t *testing.T) {
	share := sampleShareHTTP()
	tests := []struct {
		kind   shares.ErrorKind
		status int
		code   string
	}{
		{kind: shares.ErrorInvalidInput, status: http.StatusBadRequest, code: "BAD_REQUEST"},
		{kind: shares.ErrorForbidden, status: http.StatusForbidden, code: "FORBIDDEN"},
		{kind: shares.ErrorNotFound, status: http.StatusNotFound, code: "NOT_FOUND"},
		{kind: shares.ErrorInternal, status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	}
	for _, test := range tests {
		operations := &stubShareOperations{share: share, err: &shares.ServiceError{Kind: test.kind, Message: "public", Cause: errors.New("private")}}
		recorder := performJSONRequest(t, shareRouter(operations), http.MethodGet, "/api/apps/"+share.ApplicationID+"/share", "", "valid")
		if recorder.Code != test.status || !containsJSONCode(recorder.Body.Bytes(), test.code) {
			t.Fatalf("kind %s: status = %d, body = %s", test.kind, recorder.Code, recorder.Body.String())
		}
	}
}
