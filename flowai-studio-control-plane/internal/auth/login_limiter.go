package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxLoginAttempts  = 5
	loginLockDuration = 15 * time.Minute
	loginAttemptTTL   = time.Hour
)

const loginCheckScript = `
local locked = tonumber(redis.call('HGET', KEYS[1], 'locked') or '0')
if locked == 1 then
    local ttl = redis.call('PTTL', KEYS[1])
    if ttl > 0 then
        return {1, 0, ttl}
    end
    redis.call('DEL', KEYS[1])
    return {0, tonumber(ARGV[1]), 0}
end
local attempts = tonumber(redis.call('HGET', KEYS[1], 'attempts') or '0')
local remaining = tonumber(ARGV[1]) - attempts
if remaining < 0 then remaining = 0 end
return {0, remaining, 0}
`

const loginFailureScript = `
local locked = tonumber(redis.call('HGET', KEYS[1], 'locked') or '0')
if locked == 1 then
    local ttl = redis.call('PTTL', KEYS[1])
    if ttl > 0 then return {1, 0, ttl} end
    redis.call('DEL', KEYS[1])
end
local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
local maximum = tonumber(ARGV[1])
if attempts >= maximum then
    redis.call('HSET', KEYS[1], 'locked', 1)
    redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[2]))
    return {1, 0, tonumber(ARGV[2])}
end
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[3]))
return {0, maximum - attempts, 0}
`

const loginResetScript = `return redis.call('DEL', KEYS[1])`

type LoginState struct {
	Locked            bool
	RemainingAttempts int
	RetryAfter        time.Duration
}

type redisEvalClient interface {
	Eval(context.Context, string, []string, ...interface{}) *redis.Cmd
}

type LoginLimiter struct {
	client redisEvalClient
}

func NewLoginLimiter(client redisEvalClient) *LoginLimiter {
	return &LoginLimiter{client: client}
}

func (limiter *LoginLimiter) Check(ctx context.Context, username string) (LoginState, error) {
	if limiter == nil || limiter.client == nil {
		return LoginState{}, errors.New("login limiter is unavailable")
	}
	result, err := limiter.client.Eval(
		ctx,
		loginCheckScript,
		[]string{loginAttemptKey(username)},
		maxLoginAttempts,
	).Slice()
	if err != nil {
		return LoginState{}, fmt.Errorf("check login attempts: %w", err)
	}
	return parseLoginState(result)
}

func (limiter *LoginLimiter) RecordFailure(ctx context.Context, username string) (LoginState, error) {
	if limiter == nil || limiter.client == nil {
		return LoginState{}, errors.New("login limiter is unavailable")
	}
	result, err := limiter.client.Eval(
		ctx,
		loginFailureScript,
		[]string{loginAttemptKey(username)},
		maxLoginAttempts,
		loginLockDuration.Milliseconds(),
		loginAttemptTTL.Milliseconds(),
	).Slice()
	if err != nil {
		return LoginState{}, fmt.Errorf("record login failure: %w", err)
	}
	return parseLoginState(result)
}

func (limiter *LoginLimiter) Reset(ctx context.Context, username string) error {
	if limiter == nil || limiter.client == nil {
		return errors.New("login limiter is unavailable")
	}
	if err := limiter.client.Eval(ctx, loginResetScript, []string{loginAttemptKey(username)}).Err(); err != nil {
		return fmt.Errorf("reset login attempts: %w", err)
	}
	return nil
}

func loginAttemptKey(username string) string {
	digest := sha256.Sum256([]byte(username))
	return "flowai:auth:login:" + hex.EncodeToString(digest[:])
}

func parseLoginState(values []interface{}) (LoginState, error) {
	if len(values) != 3 {
		return LoginState{}, fmt.Errorf("unexpected login limiter result length %d", len(values))
	}
	locked, err := redisInteger(values[0])
	if err != nil {
		return LoginState{}, err
	}
	remaining, err := redisInteger(values[1])
	if err != nil {
		return LoginState{}, err
	}
	retryMilliseconds, err := redisInteger(values[2])
	if err != nil {
		return LoginState{}, err
	}
	return LoginState{
		Locked:            locked == 1,
		RemainingAttempts: int(remaining),
		RetryAfter:        time.Duration(retryMilliseconds) * time.Millisecond,
	}, nil
}

func redisInteger(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}
