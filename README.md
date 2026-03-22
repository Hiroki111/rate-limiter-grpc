# rate-limiter-grpc

A Shared-Nothing distributed rate limiter using gRPC P2P synchronization.

## Example Usage

### Implementation in `main.go`

```
package main

import (
	"context"
	"log"
  "net/http"
	"os"
	"strings"
	"strconv"
	"rate-limiter-grpc/internal/limiter"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	config := limiter.Config{
		ServerPort: getEnv("SERVER_PORT", "8080"),
		GRPCPort:   getEnv("GRPC_PORT", "9080"),
		Limit:      getEnvAsInt("LIMIT_PER_MINUTE", 100),
		Peers:      getEnvAsSlice("PEER_ADDRESSES", ""),
	}

	log.Printf("Initializing Node %s (Limit: %d) with peers: %v", 
		config.ServerPort, config.Limit, config.Peers)

	ctx := context.Background()
	engine := limiter.NewGRPCRateLimiter(ctx, config)
	
  http.HandleFunc("/api/resource", func(w http.ResponseWriter, r *http.Request) {
    allowed, _ := engine.Allow(r.Context(), "user-1")
    // ... handle logic ...
  })
	// ... rest of your server setup ...
}

// --- Helper Functions ---

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsSlice(key string, fallback string) []string {
	valueStr := getEnv(key, fallback)
	if valueStr == "" {
		return []string{}
	}
	return strings.Split(valueStr, ",")
}
```

### How to provide these values

#### Local Development (.env file)

Create a file named .env in your project root:

```
SERVER_PORT=8080
GRPC_PORT=9080
LIMIT_PER_MINUTE=50
PEER_ADDRESSES=localhost:9081,localhost:9082
```

#### Kubernetes (Helm Chart)

In your values.yaml, you would define the variables, which Helm injects into the container:

```
env:
  - name: LIMIT_PER_MINUTE
    value: "5000"
  - name: PEER_ADDRESSES
    value: "limiter-2:9080,limiter-3:9080"
```

#### Docker Compose

Update your docker-compose.yml to use the environment: key:

```
services:
  node-1:
    image: hiroki111/rate-limiter-grpc:latest
    environment:
      - SERVER_PORT=8080
      - GRPC_PORT=9080
      - LIMIT_PER_MINUTE=10
      - PEER_ADDRESSES=node-2:9080,node-3:9080
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