# rate-limiter-grpc

A Shared-Nothing distributed rate limiter using gRPC P2P synchronization.

## Usage

```
config := limiter.Config{
    ServerPort: "8080",
    GRPCPort:   "9080",
    Limit:      100,
    Peers:      []string{"node2:9080", "node3:9080"},
}

engine := limiter.NewGRPCRateLimiter(ctx, config)
allowed, err := engine.Allow(ctx, "user-123")
```

## Operational Guide

1. Running Locally (Go Binary)
To test the "Eventual Consistency" mesh on your local machine without Docker, open two terminals:

Node A (Terminal 1):
```
go run cmd/rate-limiter/main.go --server-port=8080 --grpc-port=9080 --peers=localhost:9081 --limit=10
```

Node B (Terminal 2):
```
go run cmd/rate-limiter/main.go --server-port=8081 --grpc-port=9081 --peers=localhost:9080 --limit=10
```

2. Running with Docker Compose
The easiest way to deploy a 3-node cluster is using the provided docker-compose.yml. This handles networking automatically, allowing nodes to find each other by their service names.

Start the cluster:

```
docker-compose up --build
```

- Node 1 is available at http://localhost:8081
- Node 2 is available at http://localhost:8082
- Node 3 is available at http://localhost:8083

3. Rebuilding the Docker Image
If you make changes to the Go code and want to update your local Docker image:

- Build the image and tag it locally

```
docker build -t grpc-limiter:latest .
```

- Verify the image exists

```
docker images | grep grpc-limiter
```

4. Pushing to Docker Hub

To share your image or deploy it to a cloud environment, follow these steps to push to your Docker Hub repository.

Step A: Login to your account

```
docker login
```

Step B: Tag the image

```
# Pattern: docker tag <local-image> <username>/<repo-name>:<tag>
docker tag grpc-limiter:latest hiroki111/rate-limiter-grpc:latest
```

Step C: Push the image

```
docker push hiroki111/rate-limiter-grpc:latest
```

## How it works

This limiter does not require Redis. Each node maintains its own local sliding window and "gossips" its state to peers via gRPC streams.

- Eventual Consistency: Synchronization happens every 100ms.
- Resilient: If a peer goes down, the node will continue limiting based on local data and automatically reconnect when the peer returns.
- Graceful: Supports Go Context for clean shutdowns.

## Running a local 2-node cluster

Terminal 1:
```
go run cmd/rate-limiter/main.go --server-port=8080 --grpc-port=9080 --peers=localhost:9081 --limit=10
```

Terminal 2:
```
go run cmd/rate-limiter/main.go --server-port=8081 --grpc-port=9081 --peers=localhost:9080 --limit=10
```