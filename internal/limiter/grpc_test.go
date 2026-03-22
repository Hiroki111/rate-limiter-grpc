package limiter

import (
	"context"
	"testing"
	"time"
)

func TestMeshIntegration(t *testing.T) {
	// 1. Setup a cancellable context for the whole test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Start two nodes.
	// Node A (8080) listens for Node B on 9081
	// Node B (8081) listens for Node A on 9080
	limit := 5
	nodePortA := "8080"
	gRPCPortA := "9080"
	nodePortB := "8081"
	gRPCPortB := "9081"
	nodeA := NewGRPCRateLimiter(ctx, nodePortA, gRPCPortA, limit, []string{"localhost:" + gRPCPortB})
	nodeB := NewGRPCRateLimiter(ctx, nodePortB, gRPCPortB, limit, []string{"localhost:" + gRPCPortA})

	// Give them a moment to perform the gRPC handshake
	time.Sleep(500 * time.Millisecond)

	// 3. Simulate 3 hits on Node A
	for i := 0; i < 3; i++ {
		allowed, _ := nodeA.Allow(ctx, "user-1")
		if !allowed {
			t.Fatal("Node A should have allowed the request")
		}
	}

	// 4. Wait for Gossip (the 100ms ticker in streamUpdates)
	time.Sleep(200 * time.Millisecond)

	// 5. Check Node B. It should see those 3 hits from Node A.
	// If we hit Node B 3 more times, the 3rd one (total 6) should be BLOCKED.

	// Hit 4 (Total 4) -> Allowed
	allowed, _ := nodeB.Allow(ctx, "user-1")
	if !allowed {
		t.Error("Hit 4 should be allowed")
	}

	// Hit 5 (Total 5) -> Allowed
	allowed, _ = nodeB.Allow(ctx, "user-1")
	if !allowed {
		t.Error("Hit 5 should be allowed")
	}

	// Hit 6 (Total 6) -> SHOULD BE BLOCKED
	allowed, _ = nodeB.Allow(ctx, "user-1")
	if allowed {
		t.Error("Hit 6 should have been blocked by the distributed limit!")
	}
}
