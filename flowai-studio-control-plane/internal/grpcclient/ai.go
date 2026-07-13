package grpcclient

import (
	"context"
	"fmt"
	"time"

	aiflowv1 "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/gen/aiflow/v1"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/httpapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const serviceTokenMetadataKey = "x-flowai-service-token"

type AIHealthChecker struct {
	client aiflowv1.ModelServiceClient
	token  string
}

func NewAIHealthChecker(connection grpc.ClientConnInterface, token string) *AIHealthChecker {
	return &AIHealthChecker{
		client: aiflowv1.NewModelServiceClient(connection),
		token:  token,
	}
}

func (checker *AIHealthChecker) Check(ctx context.Context) httpapi.CheckResult {
	request := &aiflowv1.ModelServiceHealthCheckRequest{
		Context: requestContext(ctx),
	}
	authenticated := metadata.AppendToOutgoingContext(ctx, serviceTokenMetadataKey, checker.token)
	response, err := checker.client.HealthCheck(authenticated, request)
	if err != nil || response.GetReport() == nil {
		return httpapi.CheckResult{Status: httpapi.CheckStatusUnhealthy, Message: "unavailable"}
	}
	return httpapi.CheckResult{Status: healthState(response.GetReport().GetState())}
}

func requestContext(ctx context.Context) *aiflowv1.RequestContext {
	request := &aiflowv1.RequestContext{
		RequestId: fmt.Sprintf("health-%d", time.Now().UnixNano()),
		Caller:    "control-plane",
	}
	if deadline, ok := ctx.Deadline(); ok {
		request.Deadline = timestamppb.New(deadline)
	}
	return request
}

func healthState(state aiflowv1.HealthState) httpapi.CheckStatus {
	switch state {
	case aiflowv1.HealthState_HEALTH_STATE_HEALTHY:
		return httpapi.CheckStatusHealthy
	case aiflowv1.HealthState_HEALTH_STATE_DEGRADED:
		return httpapi.CheckStatusDegraded
	case aiflowv1.HealthState_HEALTH_STATE_NOT_READY:
		return httpapi.CheckStatusNotReady
	default:
		return httpapi.CheckStatusUnhealthy
	}
}
