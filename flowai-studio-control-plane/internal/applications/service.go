package applications

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
)

var ErrApplicationNotFound = errors.New("application not found")

type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

type Application struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Icon        *string   `json:"icon,omitempty"`
	Status      Status    `json:"status"`
	ShareLink   *string   `json:"shareLink,omitempty"`
	OwnerID     string    `json:"userId,omitempty"`
	AccessType  string    `json:"accessType,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CreateInput struct {
	Name        string
	Description *string
	Icon        *string
	Status      Status
}

type UpdateInput struct {
	Name           *string
	DescriptionSet bool
	Description    *string
	IconSet        bool
	Icon           *string
	Status         *Status
}

type ErrorKind string

const (
	ErrorInvalidInput ErrorKind = "invalid_input"
	ErrorForbidden    ErrorKind = "forbidden"
	ErrorNotFound     ErrorKind = "not_found"
	ErrorInternal     ErrorKind = "internal"
)

type ServiceError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (err *ServiceError) Error() string {
	return err.Message
}

func (err *ServiceError) Unwrap() error {
	return err.Cause
}

type Store interface {
	CreateApplication(context.Context, string, CreateInput) (Application, error)
	ListApplications(context.Context, string) ([]Application, error)
	GetApplication(context.Context, string) (Application, error)
	UpdateApplication(context.Context, string, UpdateInput) (Application, error)
	DeleteApplication(context.Context, string) error
	SetApplicationStatus(context.Context, string, Status) (Application, error)
}

type Authorizer interface {
	AuthorizeApplication(context.Context, string, string, ...rbac.Permission) error
}

type Service struct {
	store      Store
	authorizer Authorizer
}

func NewService(store Store, authorizer Authorizer) *Service {
	return &Service{store: store, authorizer: authorizer}
}

func (service *Service) Create(ctx context.Context, userID string, input CreateInput) (Application, error) {
	if input.Status == "" {
		input.Status = StatusDraft
	}
	if err := validateCreateInput(input); err != nil {
		return Application{}, err
	}
	application, err := service.store.CreateApplication(ctx, userID, input)
	if err != nil {
		return Application{}, applicationStoreError("Failed to create application", err)
	}
	return application, nil
}

func (service *Service) List(ctx context.Context, userID string) ([]Application, error) {
	applications, err := service.store.ListApplications(ctx, userID)
	if err != nil {
		return nil, applicationStoreError("Failed to list applications", err)
	}
	return applications, nil
}

func (service *Service) Get(ctx context.Context, userID, applicationID string) (Application, error) {
	if err := service.authorize(ctx, userID, applicationID, rbac.PermissionAppRead); err != nil {
		return Application{}, err
	}
	application, err := service.store.GetApplication(ctx, applicationID)
	if err != nil {
		return Application{}, applicationStoreError("Failed to get application", err)
	}
	return application, nil
}

func (service *Service) Update(
	ctx context.Context,
	userID string,
	applicationID string,
	input UpdateInput,
) (Application, error) {
	if err := validateUpdateInput(input); err != nil {
		return Application{}, err
	}
	if err := service.authorize(ctx, userID, applicationID, rbac.PermissionAppUpdate); err != nil {
		return Application{}, err
	}
	application, err := service.store.UpdateApplication(ctx, applicationID, input)
	if err != nil {
		return Application{}, applicationStoreError("Failed to update application", err)
	}
	return application, nil
}

func (service *Service) Delete(ctx context.Context, userID, applicationID string) error {
	if err := service.authorize(ctx, userID, applicationID, rbac.PermissionAppDelete); err != nil {
		return err
	}
	if err := service.store.DeleteApplication(ctx, applicationID); err != nil {
		return applicationStoreError("Failed to delete application", err)
	}
	return nil
}

func (service *Service) Publish(ctx context.Context, userID, applicationID string) (Application, error) {
	return service.setStatus(ctx, userID, applicationID, StatusPublished, rbac.PermissionAppPublish)
}

func (service *Service) Unpublish(ctx context.Context, userID, applicationID string) (Application, error) {
	return service.setStatus(ctx, userID, applicationID, StatusDraft, rbac.PermissionAppPublish)
}

func (service *Service) Archive(ctx context.Context, userID, applicationID string) (Application, error) {
	return service.setStatus(ctx, userID, applicationID, StatusArchived, rbac.PermissionAppDelete)
}

func (service *Service) Unarchive(ctx context.Context, userID, applicationID string) (Application, error) {
	return service.setStatus(ctx, userID, applicationID, StatusDraft, rbac.PermissionAppDelete)
}

func (service *Service) setStatus(
	ctx context.Context,
	userID string,
	applicationID string,
	status Status,
	permission rbac.Permission,
) (Application, error) {
	if err := service.authorize(ctx, userID, applicationID, permission); err != nil {
		return Application{}, err
	}
	application, err := service.store.SetApplicationStatus(ctx, applicationID, status)
	if err != nil {
		return Application{}, applicationStoreError("Failed to update application status", err)
	}
	return application, nil
}

func (service *Service) authorize(
	ctx context.Context,
	userID string,
	applicationID string,
	permissions ...rbac.Permission,
) error {
	err := service.authorizer.AuthorizeApplication(ctx, userID, applicationID, permissions...)
	if errors.Is(err, rbac.ErrResourceNotFound) {
		return &ServiceError{Kind: ErrorNotFound, Message: "Application not found", Cause: err}
	}
	if errors.Is(err, rbac.ErrForbidden) {
		return &ServiceError{Kind: ErrorForbidden, Message: "You do not have permission to access this application", Cause: err}
	}
	if err != nil {
		return &ServiceError{Kind: ErrorInternal, Message: "Failed to authorize application", Cause: err}
	}
	return nil
}

func validateCreateInput(input CreateInput) error {
	if strings.TrimSpace(input.Name) == "" || utf8.RuneCountInString(input.Name) > 100 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Name must contain 1-100 characters"}
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 500 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Description must not exceed 500 characters"}
	}
	if input.Icon != nil && len(*input.Icon) > 2048 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Icon must not exceed 2048 characters"}
	}
	if !validStatus(input.Status) {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Status must be draft, published, or archived"}
	}
	return nil
}

func validateUpdateInput(input UpdateInput) error {
	if input.Name != nil && (strings.TrimSpace(*input.Name) == "" || utf8.RuneCountInString(*input.Name) > 100) {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Name must contain 1-100 characters"}
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 500 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Description must not exceed 500 characters"}
	}
	if input.Icon != nil && len(*input.Icon) > 2048 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Icon must not exceed 2048 characters"}
	}
	if input.Status != nil && !validStatus(*input.Status) {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Status must be draft, published, or archived"}
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusPublished, StatusArchived:
		return true
	default:
		return false
	}
}

func applicationStoreError(message string, err error) *ServiceError {
	if errors.Is(err, ErrApplicationNotFound) {
		return &ServiceError{Kind: ErrorNotFound, Message: "Application not found", Cause: err}
	}
	return &ServiceError{Kind: ErrorInternal, Message: message, Cause: err}
}
