package limiter

import (
	"context"
	"fmt"
	"rate-limiter/proto"
	"sync"
	"sync/atomic"
	"time"
)

type GRPCRateLimiter struct {
	proto.UnimplementedLimiterSyncServer
	nodeID      string
	globalLimit int
	localLimit  int

	// Local state
	localCounts [60]int64
	timestamps  [60]int64

	// Peer state
	mu      sync.RWMutex
	mirrors map[string]map[int64]int64 // [nodeID][timestamp]count
}

func NewGRPCContext(globalLimit int, peers []string) *GRPCRateLimiter {
	numOfNodes := len(peers) + 1
	return &GRPCRateLimiter{
		nodeID:      fmt.Sprintf("node-%d", time.Now().UnixNano()),
		globalLimit: globalLimit,
		localLimit:  globalLimit / numOfNodes,
		mirrors:     make(map[string]map[int64]int64),
	}
}

// Allow will implement the RateLimiter interface
func (g *GRPCRateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - 60

	var localSum int64
	for i := 0; i < 60; i++ {
		ts := atomic.LoadInt64(&g.timestamps[i])
		if ts > windowStart {
			localSum += atomic.LoadInt64(&g.localCounts[i])
		}
	}

	var peerSum int64
	g.mu.RLock()
	for _, nodeBuckets := range g.mirrors {
		for ts, count := range nodeBuckets {
			if ts > windowStart {
				peerSum += count
			}
		}
	}
	g.mu.RUnlock()

	if localSum+peerSum > int64(g.globalLimit) {
		return false, nil
	}

	idx := now % 60
	oldTs := atomic.SwapInt64(&g.timestamps[idx], now)
	if oldTs != now {
		atomic.StoreInt64(&g.localCounts[idx], 1)
	} else {
		atomic.AddInt64(&g.localCounts[idx], 1)
	}

	return true, nil
}

// func (l *GRPCRateLimiter) startGossipLoop(ctx context.Context, peerAddr string) {
// 	// 1. Establish the gRPC connection
// 	conn, _ := grpc.Dial(peerAddr, grpc.WithInsecure())
// 	client := proto.NewLimiterSyncClient(conn)
// 	stream, _ := client.SyncBuckets(ctx)

// 	ticker := time.NewTicker(100 * time.Millisecond)

// 	go func() {
// 		for {
// 			select {
// 			case <-ticker.C:
// 				// 2. Collect local changes and send them
// 				l.broadcastLatestBuckets(stream)
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}()
// }

// func (l *GRPCRateLimiter) broadcastLatestBuckets(stream proto.LimiterSync_SyncBucketsClient) {
// 	now := time.Now().Unix()

// 	// We only send the last 2-3 buckets to ensure peers are up to date
// 	// even if a single packet was dropped earlier.
// 	for offset := int64(0); offset < 3; offset++ {
// 		ts := now - offset
// 		count := l.localBuffer.GetCountForTimestamp(ts)

// 		stream.Send(&proto.BucketUpdate{
// 			NodeId:     l.nodeID,
// 			Timestamp:  ts,
// 			TotalCount: count,
// 		})
// 	}
// }
