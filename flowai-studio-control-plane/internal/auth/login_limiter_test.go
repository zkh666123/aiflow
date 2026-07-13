package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type evalCall struct {
	script string
	keys   []string
	args   []interface{}
}

type fakeEvalClient struct {
	results []interface{}
	errors  []error
	calls   []evalCall
}

func (client *fakeEvalClient) Eval(
	_ context.Context,
	script string,
	keys []string,
	args ...interface{},
) *redis.Cmd {
	client.calls = append(client.calls, evalCall{script: script, keys: append([]string(nil), keys...), args: args})
	var result interface{}
	if len(client.results) > 0 {
		result = client.results[0]
		client.results = client.results[1:]
	}
	var err error
	if len(client.errors) > 0 {
		err = client.errors[0]
		client.errors = client.errors[1:]
	}
	return redis.NewCmdResult(result, err)
}

func TestLoginLimiterMapsCheckFailureLockAndResetResults(t *testing.T) {
	client := &fakeEvalClient{results: []interface{}{
		[]interface{}{int64(0), int64(5), int64(0)},
		[]interface{}{int64(0), int64(4), int64(0)},
		[]interface{}{int64(0), int64(3), int64(0)},
		[]interface{}{int64(0), int64(2), int64(0)},
		[]interface{}{int64(0), int64(1), int64(0)},
		[]interface{}{int64(1), int64(0), int64((15 * time.Minute) / time.Millisecond)},
		[]interface{}{int64(0), int64(5), int64(0)},
		int64(1),
	}}
	limiter := NewLoginLimiter(client)
	ctx := context.Background()

	state, err := limiter.Check(ctx, "alice")
	if err != nil || state.Locked || state.RemainingAttempts != 5 {
		t.Fatalf("initial state = %#v, error = %v", state, err)
	}
	for remaining := 4; remaining >= 1; remaining-- {
		state, err = limiter.RecordFailure(ctx, "alice")
		if err != nil || state.Locked || state.RemainingAttempts != remaining {
			t.Fatalf("failure state = %#v, error = %v", state, err)
		}
	}
	state, err = limiter.RecordFailure(ctx, "alice")
	if err != nil || !state.Locked || state.RemainingAttempts != 0 || state.RetryAfter != 15*time.Minute {
		t.Fatalf("locked state = %#v, error = %v", state, err)
	}
	state, err = limiter.Check(ctx, "alice")
	if err != nil || state.Locked || state.RemainingAttempts != 5 {
		t.Fatalf("expired lock state = %#v, error = %v", state, err)
	}
	if err := limiter.Reset(ctx, "alice"); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	if len(client.calls) != 8 {
		t.Fatalf("calls = %d", len(client.calls))
	}
	for _, call := range client.calls {
		if len(call.keys) != 1 || strings.Contains(call.keys[0], "alice") {
			t.Fatalf("unsafe Redis key = %#v", call.keys)
		}
		if !strings.HasPrefix(call.keys[0], "flowai:auth:login:") {
			t.Fatalf("unexpected Redis key = %q", call.keys[0])
		}
	}
	if !strings.Contains(loginFailureScript, "HINCRBY") || !strings.Contains(loginFailureScript, "PEXPIRE") {
		t.Fatalf("failure script is not atomic state management: %s", loginFailureScript)
	}
}

func TestLoginLimiterPropagatesRedisFailures(t *testing.T) {
	client := &fakeEvalClient{errors: []error{errors.New("redis unavailable")}}
	limiter := NewLoginLimiter(client)

	if _, err := limiter.Check(context.Background(), "alice"); err == nil {
		t.Fatal("Check() accepted a Redis failure")
	}
}
