package httpapi

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type CheckStatus string

const (
	CheckStatusHealthy   CheckStatus = "healthy"
	CheckStatusDegraded  CheckStatus = "degraded"
	CheckStatusUnhealthy CheckStatus = "unhealthy"
	CheckStatusNotReady  CheckStatus = "not_ready"
)

type CheckResult struct {
	Status  CheckStatus `json:"status"`
	Version string      `json:"version,omitempty"`
	Message string      `json:"message,omitempty"`
}

type Checker interface {
	Check(context.Context) CheckResult
}

type CheckerFunc func(context.Context) CheckResult

func (check CheckerFunc) Check(ctx context.Context) CheckResult {
	return check(ctx)
}

type healthData struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
}

type namedCheckResult struct {
	name   string
	result CheckResult
}

func NewHealthHandler(checkers map[string]Checker, timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now().UTC()
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		results := make(map[string]CheckResult, len(checkers))
		resultChannel := make(chan namedCheckResult, len(checkers))
		var start sync.WaitGroup
		start.Add(len(checkers))
		for name, checker := range checkers {
			go func(name string, checker Checker) {
				defer start.Done()
				resultChannel <- namedCheckResult{name: name, result: checker.Check(ctx)}
			}(name, checker)
		}

		completed := make(chan struct{})
		go func() {
			start.Wait()
			close(completed)
		}()

		for len(results) < len(checkers) {
			select {
			case result := <-resultChannel:
				results[result.name] = normalizeCheckResult(result.result)
			case <-ctx.Done():
				for name := range checkers {
					if _, ok := results[name]; !ok {
						results[name] = CheckResult{
							Status:  CheckStatusUnhealthy,
							Message: "unavailable",
						}
					}
				}
			case <-completed:
				for len(resultChannel) > 0 {
					result := <-resultChannel
					results[result.name] = normalizeCheckResult(result.result)
				}
			}
		}

		status := "healthy"
		for _, result := range results {
			if result.Status != CheckStatusHealthy {
				status = "degraded"
				break
			}
		}

		data := healthData{Status: status, Timestamp: now, Checks: results}
		c.JSON(http.StatusOK, SuccessEnvelope(data, now))
	}
}

func normalizeCheckResult(result CheckResult) CheckResult {
	switch result.Status {
	case CheckStatusHealthy, CheckStatusDegraded, CheckStatusUnhealthy, CheckStatusNotReady:
		return result
	default:
		return CheckResult{Status: CheckStatusUnhealthy, Message: "unavailable"}
	}
}

func DatabaseChecker(pool *pgxpool.Pool) CheckerFunc {
	return func(ctx context.Context) CheckResult {
		if err := pool.Ping(ctx); err != nil {
			return CheckResult{Status: CheckStatusUnhealthy, Message: "unavailable"}
		}
		return CheckResult{Status: CheckStatusHealthy}
	}
}

func PGVectorChecker(pool *pgxpool.Pool) CheckerFunc {
	return func(ctx context.Context) CheckResult {
		var version string
		err := pool.QueryRow(
			ctx,
			"SELECT extversion::text FROM pg_extension WHERE extname = 'vector'",
		).Scan(&version)
		if err != nil {
			return CheckResult{Status: CheckStatusUnhealthy, Message: "unavailable"}
		}
		return CheckResult{Status: CheckStatusHealthy, Version: version}
	}
}

func RedisChecker(client *redis.Client) CheckerFunc {
	return func(ctx context.Context) CheckResult {
		if err := client.Ping(ctx).Err(); err != nil {
			return CheckResult{Status: CheckStatusUnhealthy, Message: "unavailable"}
		}
		return CheckResult{Status: CheckStatusHealthy}
	}
}
