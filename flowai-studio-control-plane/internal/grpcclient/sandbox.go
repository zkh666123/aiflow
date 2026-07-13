package grpcclient

import (
	"context"

	aiflowv1 "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/gen/aiflow/v1"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/httpapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type SandboxHealthChecker struct {
	client aiflowv1.SandboxServiceClient
	token  string
}

func NewSandboxHealthChecker(connection grpc.ClientConnInterface, token string) *SandboxHealthChecker {
	return &SandboxHealthChecker{
		client: aiflowv1.NewSandboxServiceClient(connection),
		token:  token,
	}
}

func (checker *SandboxHealthChecker) Check(ctx context.Context) httpapi.CheckResult {
	request := &aiflowv1.SandboxServiceHealthCheckRequest{
		Context: requestContext(ctx),
	}
	authenticated := metadata.AppendToOutgoingContext(ctx, serviceTokenMetadataKey, checker.token)
	response, err := checker.client.HealthCheck(authenticated, request)
	if err != nil || response.GetReport() == nil {
		return httpapi.CheckResult{Status: httpapi.CheckStatusUnhealthy, Message: "unavailable"}
	}
	return httpapi.CheckResult{Status: healthState(response.GetReport().GetState())}
}
