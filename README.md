How to run

- Strong consistency mode with Redis

```
docker run --name rate-limit-redis -p 6379:6379 -d redis
go run cmd/rate-limiter/main.go --mode=redis --limit=5
```

- Eventual consistency mode with gRPC

```
# Make sure protc is installed in your machine. Version 3.12.4 or above is recommended.
# You can confirm if protoc is installed by running:
protoc --version
# If this doesn't show a protoc's version, install it.

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/limiter.proto
```

TODO:

- Remove code related to Redis mode
- Dockerfile
- Publish to Docker Hub
- Unit/integration tests