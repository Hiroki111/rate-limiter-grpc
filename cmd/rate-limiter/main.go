package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"rate-limiter/internal/limiter"

	"github.com/redis/go-redis/v9"
)

func main() {
	mode := flag.String("mode", "redis", "Rate limiter mode: 'redis' or 'grpc'")
	port := flag.String("port", "8080", "HTTP server port")
	redisAddr := flag.String("redis-addr", "localhost:6379", "Redis address")
	peers := flag.String("peers", "", "Comma-separated list of gRPC peers (for grpc mode)")
	limit := flag.Int("limit", 100, "Global requests per minute")
	flag.Parse()

	var engine limiter.RateLimiter

	switch *mode {
	case "redis":
		fmt.Printf("Starting in REDIS mode at :%s\n", *port)
		redisClient := redis.NewClient(&redis.Options{Addr: *redisAddr})
		engine = limiter.NewRedisLimiter(redisClient, *limit, 60)

	case "grpc":
		fmt.Printf("Starting in gRPC P2P mode at :%s\n", *port)
		peerList := strings.Split(*peers, ",")
		engine = limiter.NewGRPCContext(*limit, peerList)

	default:
		log.Fatalf("Invalid mode: %s", *mode)
	}

	http.HandleFunc("/api/resource", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user")
		if userID == "" {
			userID = "default-user"
		}

		allowed, err := engine.Allow(r.Context(), userID)
		if err != nil {
			// Fail-open logic could be added here
			http.Error(w, "Internal Limiter Error", 500)
			return
		}

		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "Rate limit exceeded. Try again later.")
			return
		}

		fmt.Fprint(w, "Access Granted")
	})

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
