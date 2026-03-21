package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"rate-limiter/internal/limiter"
)

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	peers := flag.String("peers", "", "Comma-separated list of gRPC peers")
	limit := flag.Int("limit", 100, "Global requests per minute")
	flag.Parse()

	fmt.Printf("Starting at :%s, limit :%d\n", *port, *limit)
	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
	}
	engine := limiter.NewGRPCRateLimiter(*port, *limit, peerList)

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
