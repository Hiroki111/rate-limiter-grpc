package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"rate-limiter/internal/limiter"
)

func main() {
	serverPort := flag.String("server-port", "8080", "HTTP server port")
	gRPCPort := flag.String("grpc-port", "9080", "gRPC port")
	peers := flag.String("peers", "", "Comma-separated list of gRPC peer addresses (e.g. host1:9080,host2:9081)")
	limit := flag.Int("limit", 100, "Global requests per minute")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
	}

	config := limiter.Config{
		ServerPort: *serverPort,
		GRPCPort:   *gRPCPort,
		Limit:      *limit,
		Peers:      peerList,
	}

	engine := limiter.NewGRPCRateLimiter(ctx, config)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/resource", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user")
		if userID == "" {
			userID = "default-user"
		}

		allowed, err := engine.Allow(r.Context(), userID)
		if err != nil {
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

	server := &http.Server{
		Addr:    ":" + *serverPort,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting HTTP Server at :%s, limit :%d", *serverPort, *limit)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited.")
}
