from __future__ import annotations

import argparse
import json
from datetime import UTC, datetime, timedelta

import grpc
from google.protobuf import timestamp_pb2

from aiflow.v1 import common_pb2, models_pb2, models_pb2_grpc, sandbox_pb2, sandbox_pb2_grpc

METADATA_KEY = "x-flowai-service-token"


def request_context() -> common_pb2.RequestContext:
    deadline = timestamp_pb2.Timestamp()
    deadline.FromDatetime(datetime.now(UTC) + timedelta(seconds=5))
    return common_pb2.RequestContext(
        request_id=f"native-check-{datetime.now(UTC).timestamp()}",
        caller="native-check",
        deadline=deadline,
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("service", choices=("ai", "sandbox"))
    parser.add_argument("--address", required=True)
    parser.add_argument("--token", required=True)
    arguments = parser.parse_args()

    metadata = ((METADATA_KEY, arguments.token),)
    with grpc.insecure_channel(arguments.address) as channel:
        if arguments.service == "ai":
            response = models_pb2_grpc.ModelServiceStub(channel).HealthCheck(
                models_pb2.ModelServiceHealthCheckRequest(context=request_context()),
                metadata=metadata,
                timeout=10,
            )
        else:
            response = sandbox_pb2_grpc.SandboxServiceStub(channel).HealthCheck(
                sandbox_pb2.SandboxServiceHealthCheckRequest(context=request_context()),
                metadata=metadata,
                timeout=10,
            )

    print(
        json.dumps(
            {
                "service": arguments.service,
                "state": common_pb2.HealthState.Name(response.report.state),
                "components": [item.name for item in response.report.components],
            },
            separators=(",", ":"),
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
