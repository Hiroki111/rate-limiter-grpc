package limiter

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"rate-limiter/proto"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCRateLimiter struct {
	proto.UnimplementedLimiterSyncServer
	nodeID      string
	globalLimit int

	// Local state
	localCounts [60]int64
	timestamps  [60]int64

	// Peer state
	mu      sync.RWMutex
	mirrors map[string]map[int64]int64 // [nodeID][timestamp]count
}

const gRPCPortOffset = 1000

func NewGRPCRateLimiter(port string, globalLimit int, peers []string) *GRPCRateLimiter {
	g := &GRPCRateLimiter{
		nodeID:      fmt.Sprintf("node-%s", port),
		globalLimit: globalLimit,
		mirrors:     make(map[string]map[int64]int64),
	}

	portInt, _ := strconv.Atoi(port)
	grpcAddr := ":" + fmt.Sprintf("%d", portInt+gRPCPortOffset)

	ready := make(chan bool)
	go g.serveGRPC(grpcAddr, ready)
	<-ready

	ctx := context.Background()
	for _, peer := range peers {
		if peer == "" {
			continue
		}

		log.Printf("Targeting peer gRPC at %s", peer)
		go g.maintainPeerConnection(ctx, peer)
	}

	go g.startSweeper(ctx)

	return g
}

func (g *GRPCRateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - 60

	var localSum int64
	g.mu.RLock()
	for i := 0; i < 60; i++ {
		if g.timestamps[i] > windowStart {
			localSum += atomic.LoadInt64(&g.localCounts[i])
		}
	}

	var peerSum int64
	for _, nodeBuckets := range g.mirrors {
		for ts, count := range nodeBuckets {
			if ts > windowStart {
				peerSum += count
			}
		}
	}
	g.mu.RUnlock()

	if localSum+peerSum >= int64(g.globalLimit) {
		return false, nil
	}

	idx := now % 60
	g.mu.Lock()
	// A -> updating localCounts
	oldTs := atomic.SwapInt64(&g.timestamps[idx], now)
	if oldTs != now {
		g.localCounts[idx] = 1
	} else {
		g.localCounts[idx]++
	}
	g.mu.Unlock()

	return true, nil
}

func (g *GRPCRateLimiter) SyncBuckets(stream proto.LimiterSync_SyncBucketsServer) error {
	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&proto.SyncResponse{Acknowledged: true})
		}
		if err != nil {
			return err
		}

		g.mu.Lock()
		if _, ok := g.mirrors[update.NodeId]; !ok {
			g.mirrors[update.NodeId] = make(map[int64]int64)
		}
		g.mirrors[update.NodeId][update.Timestamp] = update.TotalCount
		g.mu.Unlock()
	}
}

func (g *GRPCRateLimiter) serveGRPC(addr string, ready chan bool) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("gRPC server failed to listen: %v", err)
	}

	server := grpc.NewServer()
	proto.RegisterLimiterSyncServer(server, g)

	log.Printf("gRPC Sync Server listening on %s", addr)

	ready <- true
	if err := server.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}

func (g *GRPCRateLimiter) maintainPeerConnection(ctx context.Context, addr string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := grpc.NewClient(
				addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				log.Printf("Client creation failed for %s: %v", addr, err)
				time.Sleep(2 * time.Second)
				continue
			}

			conn.Connect()
			if !g.waitForReady(ctx, conn) {
				conn.Close()
				time.Sleep(2 * time.Second)
				continue
			}

			log.Printf("Successfully connected and READY: %s", addr)
			client := proto.NewLimiterSyncClient(conn)

			err = g.streamUpdates(ctx, client)
			if err != nil {
				log.Printf("Stream to %s lost: %v", addr, err)
			}

			conn.Close()
			time.Sleep(1 * time.Second)
		}
	}
}

func (g *GRPCRateLimiter) waitForReady(ctx context.Context, conn *grpc.ClientConn) bool {
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return true
		}
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			return false
		}
		// Wait for the state to change or the context to expire
		if !conn.WaitForStateChange(ctx, state) {
			return false // Context cancelled
		}
	}
}

func (g *GRPCRateLimiter) streamUpdates(ctx context.Context, client proto.LimiterSyncClient) error {
	stream, err := client.SyncBuckets(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			now := time.Now().Unix()

			// We gossip the last 3 seconds of data to handle minor packet loss
			for i := int64(0); i < 3; i++ {
				ts := now - i
				idx := ts % 60

				// g.timestamps can contain 0 and old timestamps
				actualTs := atomic.LoadInt64(&g.timestamps[idx])
				if actualTs == ts {
					count := atomic.LoadInt64(&g.localCounts[idx])
					err := stream.Send(&proto.BucketUpdate{
						NodeId:     g.nodeID,
						Timestamp:  ts,
						TotalCount: count,
					})
					if err != nil {
						return err // Break and trigger reconnection
					}
				}
			}
		}
	}
}

func (g *GRPCRateLimiter) startSweeper(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().Unix()
			g.mu.Lock()

			emptyNodeIDs := make([]string, 0)
			for nodeID, buckets := range g.mirrors {
				for ts := range buckets {
					if now-ts > 60 {
						delete(buckets, ts)
					}
				}
				if len(buckets) == 0 {
					emptyNodeIDs = append(emptyNodeIDs, nodeID)
				}
			}

			for _, nodeID := range emptyNodeIDs {
				delete(g.mirrors, nodeID)
			}
			g.mu.Unlock()
		}
	}
}
