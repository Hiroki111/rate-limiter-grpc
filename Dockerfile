FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rate-limiter-grpc ./cmd/rate-limiter/main.go

FROM alpine:latest  
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/rate-limiter-grpc .

# Default ports (can be overridden)
EXPOSE 8080 9080

ENTRYPOINT ["./rate-limiter-grpc"]