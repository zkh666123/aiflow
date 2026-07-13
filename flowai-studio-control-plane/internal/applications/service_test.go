package applications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
)

type fakeApplicationStore struct {
	application  Application
	applications []Application
	err          error
	created      CreateInput
	updated      UpdateInput
	status       Status
	deletedID    string
}

func (store *fakeApplicationStore) CreateApplication(_ context.Context, _ string, input CreateInput) (Application, error) {
	store.created = input
	return store.application, store.err
}

func (store *fakeApplicationStore) ListApplications(context.Context, string) ([]Application, error) {
	return store.applications, store.err
}

func (store *fakeApplicationStore) GetApplication(context.Context, string) (Application, error) {
	return store.application, store.err
}

func (store *fakeApplicationStore) UpdateApplication(_ context.Context, _ string, input UpdateInput) (Application, error) {
	store.updated = input
	return store.application, store.err
}

func (store *fakeApplicationStore) DeleteApplication(_ context.Context, id string) error {
	store.deletedID = id
	return store.err
}

func (store *fakeApplicationStore) SetApplicationStatus(_ context.Context, _ string, status Status) (Application, error) {
	store.status = status
	application := store.application
	application.Status = status
	return application, store.err
}

type fakeApplicationAuthorizer struct {
	err         error
	permissions []rbac.Permission
	userID      string
	appID       string
}

func (authorizer *fakeApplicationAuthorizer) AuthorizeApplication(
	_ context.Context,
	userID string,
	appID string,
	permissions ...rbac.Permission,
) error {
	authorizer.userID = userID
	authorizer.appID = appID
	authorizer.permissions = append([]rbac.Permission(nil), permissions...)
	return authorizer.err
}

func sampleApplication() Application {
	description := "description"
	icon := "icon"
	return Application{
		ID:          "7a611d9a-b555-4469-a289-f1672daefce3",
		Name:        "App",
		Description: &description,
		Icon:        &icon,
		Status:      StatusDraft,
		OwnerID:     "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		CreatedAt:   time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 13, 16, 0, 0, 0, time.UTC),
	}
}

func TestCreateApplicationValidatesInputAndDefaultsStatus(t *testing.T) {
	store := &fakeApplicationStore{application: sampleApplication()}
	service := NewService(store, &fakeApplicationAuthorizer{})

	created, err := service.Create(context.Background(), "user", CreateInput{Name: "App"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" || store.created.Status != StatusDraft {
		t.Fatalf("created = %#v, input = %#v", created, store.created)
	}

	tests := []CreateInput{
		{Name: ""},
		{Name: string(make([]byte, 101))},
		{Name: "App", Description: stringPointer(string(make([]byte, 501)))},
		{Name: "App", Status: "invalid"},
	}
	for _, input := range tests {
		_, err := service.Create(context.Background(), "user", input)
		assertApplicationError(t, err, ErrorInvalidInput)
	}
}

func TestListApplicationsPreservesOrderedAccessTypes(t *testing.T) {
	applications := []Application{
		{ID: "owned", AccessType: "owner"},
		{ID: "team", AccessType: "full_access"},
	}
	service := NewService(&fakeApplicationStore{applications: applications}, &fakeApplicationAuthorizer{})

	result, err := service.List(context.Background(), "user")
	if err != nil || len(result) != 2 || result[0].AccessType != "owner" || result[1].AccessType != "full_access" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestApplicationOperationsRequireTheCorrectPermissions(t *testing.T) {
	application := sampleApplication()
	tests := []struct {
		name       string
		permission rbac.Permission
		operation  func(*Service) error
	}{
		{name: "get", permission: rbac.PermissionAppRead, operation: func(service *Service) error {
			_, err := service.Get(context.Background(), "user", application.ID)
			return err
		}},
		{name: "update", permission: rbac.PermissionAppUpdate, operation: func(service *Service) error {
			name := "Updated"
			_, err := service.Update(context.Background(), "user", application.ID, UpdateInput{Name: &name})
			return err
		}},
		{name: "delete", permission: rbac.PermissionAppDelete, operation: func(service *Service) error { return service.Delete(context.Background(), "user", application.ID) }},
		{name: "publish", permission: rbac.PermissionAppPublish, operation: func(service *Service) error {
			_, err := service.Publish(context.Background(), "user", application.ID)
			return err
		}},
		{name: "unpublish", permission: rbac.PermissionAppPublish, operation: func(service *Service) error {
			_, err := service.Unpublish(context.Background(), "user", application.ID)
			return err
		}},
		{name: "archive", permission: rbac.PermissionAppDelete, operation: func(service *Service) error {
			_, err := service.Archive(context.Background(), "user", application.ID)
			return err
		}},
		{name: "unarchive", permission: rbac.PermissionAppDelete, operation: func(service *Service) error {
			_, err := service.Unarchive(context.Background(), "user", application.ID)
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeApplicationStore{application: application}
			authorizer := &fakeApplicationAuthorizer{}
			service := NewService(store, authorizer)
			if err := test.operation(service); err != nil {
				t.Fatalf("operation error = %v", err)
			}
			if len(authorizer.permissions) != 1 || authorizer.permissions[0] != test.permission {
				t.Fatalf("permissions = %#v", authorizer.permissions)
			}
		})
	}
}

func TestApplicationStatusTransitionsUseCanonicalValues(t *testing.T) {
	store := &fakeApplicationStore{application: sampleApplication()}
	service := NewService(store, &fakeApplicationAuthorizer{})
	tests := []struct {
		name      string
		operation func() (Application, error)
		want      Status
	}{
		{name: "publish", operation: func() (Application, error) { return service.Publish(context.Background(), "user", "app") }, want: StatusPublished},
		{name: "unpublish", operation: func() (Application, error) { return service.Unpublish(context.Background(), "user", "app") }, want: StatusDraft},
		{name: "archive", operation: func() (Application, error) { return service.Archive(context.Background(), "user", "app") }, want: StatusArchived},
		{name: "unarchive", operation: func() (Application, error) { return service.Unarchive(context.Background(), "user", "app") }, want: StatusDraft},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.operation()
			if err != nil || store.status != test.want || result.Status != test.want {
				t.Fatalf("result = %#v, status = %q, error = %v", result, store.status, err)
			}
		})
	}
}

func TestApplicationServiceMapsAuthorizationAndStoreFailures(t *testing.T) {
	service := NewService(&fakeApplicationStore{application: sampleApplication()}, &fakeApplicationAuthorizer{err: rbac.ErrForbidden})
	_, err := service.Get(context.Background(), "user", "app")
	assertApplicationError(t, err, ErrorForbidden)

	service = NewService(&fakeApplicationStore{application: sampleApplication()}, &fakeApplicationAuthorizer{err: rbac.ErrResourceNotFound})
	_, err = service.Get(context.Background(), "user", "app")
	assertApplicationError(t, err, ErrorNotFound)

	service = NewService(&fakeApplicationStore{err: ErrApplicationNotFound}, &fakeApplicationAuthorizer{})
	_, err = service.Get(context.Background(), "user", "app")
	assertApplicationError(t, err, ErrorNotFound)
}

func stringPointer(value string) *string {
	return &value
}

func assertApplicationError(t *testing.T, err error, kind ErrorKind) *ServiceError {
	t.Helper()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("error = %v, want ServiceError", err)
	}
	if serviceErr.Kind != kind {
		t.Fatalf("kind = %q, want %q", serviceErr.Kind, kind)
	}
	return serviceErr
}
