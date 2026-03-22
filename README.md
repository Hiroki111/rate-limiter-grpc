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