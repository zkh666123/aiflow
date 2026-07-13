package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const passwordHashCost = 12

var (
	usernamePattern   = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)
	ErrUserNotFound   = errors.New("user not found")
	ErrUsernameExists = errors.New("username already exists")
)

type ErrorKind string

const (
	ErrorInvalidInput ErrorKind = "invalid_input"
	ErrorConflict     ErrorKind = "conflict"
	ErrorUnauthorized ErrorKind = "unauthorized"
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

type UserRecord struct {
	ID           string
	Username     string
	PasswordHash string
	Avatar       *string
	GlobalRole   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Avatar    *string   `json:"avatar,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type RegisterInput struct {
	Username string
	Password string
}

type LoginInput struct {
	Username string
	Password string
}

type LoginResult struct {
	User  User   `json:"user"`
	Token string `json:"token"`
}

type UpdateProfileInput struct {
	Username  *string
	AvatarSet bool
	Avatar    *string
}

type UserStore interface {
	CreateUser(context.Context, string, string) (UserRecord, error)
	GetUserByUsername(context.Context, string) (UserRecord, error)
	GetUserByID(context.Context, string) (UserRecord, error)
	UpdateUserProfile(context.Context, string, UpdateProfileInput) (UserRecord, error)
}

type AttemptLimiter interface {
	Check(context.Context, string) (LoginState, error)
	RecordFailure(context.Context, string) (LoginState, error)
	Reset(context.Context, string) error
}

type TokenIssuer interface {
	Sign(Principal) (string, error)
}

type Service struct {
	store             UserStore
	limiter           AttemptLimiter
	tokens            TokenIssuer
	dummyPasswordHash []byte
}

func NewService(store UserStore, limiter AttemptLimiter, tokens TokenIssuer) (*Service, error) {
	if store == nil || limiter == nil || tokens == nil {
		return nil, errors.New("user service dependencies are required")
	}
	dummyHash, err := bcrypt.GenerateFromPassword([]byte("flowai-invalid-user-password"), passwordHashCost)
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &Service{store: store, limiter: limiter, tokens: tokens, dummyPasswordHash: dummyHash}, nil
}

func (service *Service) Register(ctx context.Context, input RegisterInput) (User, error) {
	if err := validateUsername(input.Username); err != nil {
		return User{}, err
	}
	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), passwordHashCost)
	if err != nil {
		return User{}, internalError("Registration failed", err)
	}
	record, err := service.store.CreateUser(ctx, input.Username, string(hash))
	if errors.Is(err, ErrUsernameExists) {
		return User{}, &ServiceError{Kind: ErrorConflict, Message: "Username already exists", Cause: err}
	}
	if err != nil {
		return User{}, internalError("Registration failed", err)
	}
	return publicUser(record), nil
}

func (service *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return LoginResult{}, &ServiceError{Kind: ErrorInvalidInput, Message: "Username and password are required"}
	}
	state, err := service.limiter.Check(ctx, input.Username)
	if err != nil {
		return LoginResult{}, internalError("Login failed", err)
	}
	if state.Locked {
		return LoginResult{}, lockedError(state.RetryAfter)
	}

	record, lookupErr := service.store.GetUserByUsername(ctx, input.Username)
	hash := service.dummyPasswordHash
	if lookupErr == nil {
		hash = []byte(record.PasswordHash)
	} else if !errors.Is(lookupErr, ErrUserNotFound) {
		return LoginResult{}, internalError("Login failed", lookupErr)
	}
	passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(input.Password))
	if lookupErr != nil || passwordErr != nil {
		failureState, failureErr := service.limiter.RecordFailure(ctx, input.Username)
		if failureErr != nil {
			return LoginResult{}, internalError("Login failed", failureErr)
		}
		if failureState.Locked {
			return LoginResult{}, lockedError(failureState.RetryAfter)
		}
		return LoginResult{}, &ServiceError{Kind: ErrorUnauthorized, Message: "Invalid username or password"}
	}

	if err := service.limiter.Reset(ctx, input.Username); err != nil {
		return LoginResult{}, internalError("Login failed", err)
	}
	token, err := service.tokens.Sign(Principal{UserID: record.ID, Username: record.Username})
	if err != nil {
		return LoginResult{}, internalError("Login failed", err)
	}
	return LoginResult{User: publicUser(record), Token: token}, nil
}

func (service *Service) Profile(ctx context.Context, userID string) (User, error) {
	record, err := service.store.GetUserByID(ctx, userID)
	if errors.Is(err, ErrUserNotFound) {
		return User{}, &ServiceError{Kind: ErrorUnauthorized, Message: "User does not exist", Cause: err}
	}
	if err != nil {
		return User{}, internalError("Failed to get user profile", err)
	}
	return publicUser(record), nil
}

func (service *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (User, error) {
	if input.Username != nil {
		if err := validateUsername(*input.Username); err != nil {
			return User{}, err
		}
	}
	if input.Avatar != nil && len(*input.Avatar) > 2048 {
		return User{}, &ServiceError{Kind: ErrorInvalidInput, Message: "Avatar URL must not exceed 2048 characters"}
	}
	record, err := service.store.UpdateUserProfile(ctx, userID, input)
	if errors.Is(err, ErrUsernameExists) {
		return User{}, &ServiceError{Kind: ErrorConflict, Message: "Username already exists", Cause: err}
	}
	if errors.Is(err, ErrUserNotFound) {
		return User{}, &ServiceError{Kind: ErrorUnauthorized, Message: "User does not exist", Cause: err}
	}
	if err != nil {
		return User{}, internalError("Failed to update user profile", err)
	}
	return publicUser(record), nil
}

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return &ServiceError{
			Kind:    ErrorInvalidInput,
			Message: "Username must be 3-20 letters, numbers, or underscores",
		}
	}
	return nil
}

func validatePassword(password string) error {
	size := len([]byte(password))
	if utf8.RuneCountInString(password) < 6 || size > 72 {
		return &ServiceError{Kind: ErrorInvalidInput, Message: "Password must contain 6-72 bytes"}
	}
	return nil
}

func lockedError(retryAfter time.Duration) *ServiceError {
	minutes := int((retryAfter + time.Minute - 1) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	return &ServiceError{
		Kind:    ErrorUnauthorized,
		Message: fmt.Sprintf("\u8d26\u6237\u5df2\u88ab\u9501\u5b9a\uff0c\u8bf7 %d \u5206\u949f\u540e\u518d\u8bd5", minutes),
	}
}

func internalError(message string, cause error) *ServiceError {
	return &ServiceError{Kind: ErrorInternal, Message: message, Cause: cause}
}

func publicUser(record UserRecord) User {
	return User{
		ID:        record.ID,
		Username:  record.Username,
		Avatar:    record.Avatar,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}
