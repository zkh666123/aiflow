package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryUserStore struct {
	users       map[string]UserRecord
	byID        map[string]string
	createCalls int
	lookupCalls int
}

func newMemoryUserStore() *memoryUserStore {
	return &memoryUserStore{users: map[string]UserRecord{}, byID: map[string]string{}}
}

func (store *memoryUserStore) CreateUser(_ context.Context, username, passwordHash string) (UserRecord, error) {
	store.createCalls++
	if _, exists := store.users[username]; exists {
		return UserRecord{}, ErrUsernameExists
	}
	user := UserRecord{
		ID:           "e9f6332d-da39-44b2-917c-da5ff30aca9d",
		Username:     username,
		PasswordHash: passwordHash,
		GlobalRole:   "member",
		CreatedAt:    time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
	}
	store.users[username] = user
	store.byID[user.ID] = username
	return user, nil
}

func (store *memoryUserStore) GetUserByUsername(_ context.Context, username string) (UserRecord, error) {
	store.lookupCalls++
	user, exists := store.users[username]
	if !exists {
		return UserRecord{}, ErrUserNotFound
	}
	return user, nil
}

func (store *memoryUserStore) GetUserByID(_ context.Context, id string) (UserRecord, error) {
	username, exists := store.byID[id]
	if !exists {
		return UserRecord{}, ErrUserNotFound
	}
	return store.users[username], nil
}

func (store *memoryUserStore) UpdateUserProfile(_ context.Context, id string, input UpdateProfileInput) (UserRecord, error) {
	username, exists := store.byID[id]
	if !exists {
		return UserRecord{}, ErrUserNotFound
	}
	user := store.users[username]
	if input.Username != nil && *input.Username != username {
		if _, conflict := store.users[*input.Username]; conflict {
			return UserRecord{}, ErrUsernameExists
		}
		delete(store.users, username)
		user.Username = *input.Username
		store.byID[id] = user.Username
	}
	if input.AvatarSet {
		user.Avatar = input.Avatar
	}
	store.users[user.Username] = user
	return user, nil
}

type fakeAttemptLimiter struct {
	checkState   LoginState
	failureState LoginState
	checkErr     error
	failureErr   error
	resetErr     error
	checks       int
	failures     int
	resets       int
}

func (limiter *fakeAttemptLimiter) Check(context.Context, string) (LoginState, error) {
	limiter.checks++
	return limiter.checkState, limiter.checkErr
}

func (limiter *fakeAttemptLimiter) RecordFailure(context.Context, string) (LoginState, error) {
	limiter.failures++
	return limiter.failureState, limiter.failureErr
}

func (limiter *fakeAttemptLimiter) Reset(context.Context, string) error {
	limiter.resets++
	return limiter.resetErr
}

type fakeTokenIssuer struct {
	token     string
	principal Principal
	err       error
}

func (issuer *fakeTokenIssuer) Sign(principal Principal) (string, error) {
	issuer.principal = principal
	return issuer.token, issuer.err
}

func newTestService(t *testing.T, store UserStore, limiter AttemptLimiter, issuer TokenIssuer) *Service {
	t.Helper()
	service, err := NewService(store, limiter, issuer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestRegisterValidatesAndHashesPasswordsAtCostTwelve(t *testing.T) {
	store := newMemoryUserStore()
	service := newTestService(t, store, &fakeAttemptLimiter{}, &fakeTokenIssuer{})

	user, err := service.Register(context.Background(), RegisterInput{Username: "alice_1", Password: "secret1"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Username != "alice_1" || user.ID == "" {
		t.Fatalf("user = %#v", user)
	}
	stored := store.users["alice_1"]
	if stored.PasswordHash == "secret1" {
		t.Fatal("password was stored in plaintext")
	}
	cost, err := bcrypt.Cost([]byte(stored.PasswordHash))
	if err != nil || cost != 12 {
		t.Fatalf("bcrypt cost = %d, error = %v", cost, err)
	}
}

func TestRegisterRejectsInvalidAndDuplicateUsernames(t *testing.T) {
	tests := []RegisterInput{
		{Username: "ab", Password: "secret1"},
		{Username: "contains-hyphen", Password: "secret1"},
		{Username: "alice", Password: "short"},
		{Username: "alice", Password: string(make([]byte, 73))},
	}
	for _, input := range tests {
		t.Run(input.Username, func(t *testing.T) {
			store := newMemoryUserStore()
			service := newTestService(t, store, &fakeAttemptLimiter{}, &fakeTokenIssuer{})
			_, err := service.Register(context.Background(), input)
			assertServiceErrorKind(t, err, ErrorInvalidInput)
			if store.createCalls != 0 {
				t.Fatalf("CreateUser() calls = %d", store.createCalls)
			}
		})
	}

	store := newMemoryUserStore()
	service := newTestService(t, store, &fakeAttemptLimiter{}, &fakeTokenIssuer{})
	if _, err := service.Register(context.Background(), RegisterInput{Username: "alice", Password: "secret1"}); err != nil {
		t.Fatal(err)
	}
	_, err := service.Register(context.Background(), RegisterInput{Username: "alice", Password: "secret2"})
	assertServiceErrorKind(t, err, ErrorConflict)
}

func TestLoginUsesOneCredentialErrorAndResetsAttemptsOnSuccess(t *testing.T) {
	store := newMemoryUserStore()
	service := newTestService(t, store, &fakeAttemptLimiter{failureState: LoginState{RemainingAttempts: 4}}, &fakeTokenIssuer{token: "jwt"})
	if _, err := service.Register(context.Background(), RegisterInput{Username: "alice", Password: "secret1"}); err != nil {
		t.Fatal(err)
	}

	for _, input := range []LoginInput{
		{Username: "missing", Password: "secret1"},
		{Username: "alice", Password: "wrong-password"},
	} {
		_, err := service.Login(context.Background(), input)
		serviceErr := assertServiceErrorKind(t, err, ErrorUnauthorized)
		if serviceErr.Message != "Invalid username or password" {
			t.Fatalf("message = %q", serviceErr.Message)
		}
	}

	limiter := &fakeAttemptLimiter{}
	issuer := &fakeTokenIssuer{token: "jwt"}
	service = newTestService(t, store, limiter, issuer)
	result, err := service.Login(context.Background(), LoginInput{Username: "alice", Password: "secret1"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token != "jwt" || result.User.Username != "alice" || limiter.resets != 1 {
		t.Fatalf("result = %#v, resets = %d", result, limiter.resets)
	}
	if issuer.principal.UserID != result.User.ID || issuer.principal.Username != "alice" {
		t.Fatalf("principal = %#v", issuer.principal)
	}
}

func TestLoginChecksLockBeforeLookingUpTheUser(t *testing.T) {
	store := newMemoryUserStore()
	limiter := &fakeAttemptLimiter{checkState: LoginState{Locked: true, RetryAfter: 14*time.Minute + time.Second}}
	service := newTestService(t, store, limiter, &fakeTokenIssuer{})

	_, err := service.Login(context.Background(), LoginInput{Username: "alice", Password: "secret1"})
	assertServiceErrorKind(t, err, ErrorUnauthorized)
	if store.lookupCalls != 0 {
		t.Fatalf("GetUserByUsername() calls = %d", store.lookupCalls)
	}
}

func TestProfileAndUpdateNeverExposePasswordHash(t *testing.T) {
	store := newMemoryUserStore()
	service := newTestService(t, store, &fakeAttemptLimiter{}, &fakeTokenIssuer{})
	registered, err := service.Register(context.Background(), RegisterInput{Username: "alice", Password: "secret1"})
	if err != nil {
		t.Fatal(err)
	}

	profile, err := service.Profile(context.Background(), registered.ID)
	if err != nil || profile.Username != "alice" {
		t.Fatalf("profile = %#v, error = %v", profile, err)
	}
	newUsername := "alice_2"
	avatar := "https://example.test/avatar.png"
	updated, err := service.UpdateProfile(context.Background(), registered.ID, UpdateProfileInput{
		Username:  &newUsername,
		AvatarSet: true,
		Avatar:    &avatar,
	})
	if err != nil || updated.Username != newUsername || updated.Avatar == nil || *updated.Avatar != avatar {
		t.Fatalf("updated = %#v, error = %v", updated, err)
	}
}

func assertServiceErrorKind(t *testing.T, err error, kind ErrorKind) *ServiceError {
	t.Helper()
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("error = %v, want ServiceError", err)
	}
	if serviceErr.Kind != kind {
		t.Fatalf("error kind = %q, want %q", serviceErr.Kind, kind)
	}
	return serviceErr
}
