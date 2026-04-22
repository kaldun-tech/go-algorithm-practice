package practice

import (
	"testing"
)

// Helper to create a NodeID from a byte (for test readability)
func nodeID(b byte) NodeID {
	var id NodeID
	id[0] = b
	return id
}

// =============================================================================
// Step 1: Fee Calculation Tests
// =============================================================================

func TestHopFee(t *testing.T) {
	tests := []struct {
		name       string
		policy     ChannelPolicy
		amountMsat int64
		wantFee    int64
	}{
		{
			name:       "zero fee policy",
			policy:     ChannelPolicy{BaseFee: 0, FeeRatePPM: 0},
			amountMsat: 1_000_000,
			wantFee:    0,
		},
		{
			name:       "base fee only",
			policy:     ChannelPolicy{BaseFee: 1000, FeeRatePPM: 0},
			amountMsat: 1_000_000,
			wantFee:    1000,
		},
		{
			name:       "ppm fee only",
			policy:     ChannelPolicy{BaseFee: 0, FeeRatePPM: 1000}, // 0.1%
			amountMsat: 1_000_000,
			wantFee:    1000, // 1M * 1000 / 1M = 1000
		},
		{
			name:       "combined fees",
			policy:     ChannelPolicy{BaseFee: 500, FeeRatePPM: 500},
			amountMsat: 2_000_000,
			wantFee:    1500, // 500 + (2M * 500 / 1M) = 500 + 1000
		},
		{
			name:       "large amount",
			policy:     ChannelPolicy{BaseFee: 1, FeeRatePPM: 100},
			amountMsat: 100_000_000, // 100k sats
			wantFee:    10001,       // 1 + (100M * 100 / 1M) = 1 + 10000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hopFee(tt.policy, tt.amountMsat)
			if got != tt.wantFee {
				t.Errorf("hopFee() = %d, want %d", got, tt.wantFee)
			}
		})
	}
}

// =============================================================================
// Step 2-4: Pathfinding Tests
// =============================================================================

func TestFindPath_LinearThreeHop(t *testing.T) {
	// Graph: A -> B -> C -> D
	// Payment: A to D for 1,000,000 msat
	// Verify fee compounding works correctly (fees calculated backwards)

	g := NewGraph()
	nodeA := nodeID('A')
	nodeB := nodeID('B')
	nodeC := nodeID('C')
	nodeD := nodeID('D')

	// Add edges with different fee policies
	g.AddEdge(ChannelEdge{
		ChannelID: 1,
		From:      nodeA,
		To:        nodeB,
		Capacity:  1_000_000, // 1M sats
		Policy: ChannelPolicy{
			BaseFee:    100,
			FeeRatePPM: 100,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 2,
		From:      nodeB,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:    200,
			FeeRatePPM: 200,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 3,
		From:      nodeC,
		To:        nodeD,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:    300,
			FeeRatePPM: 300,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})

	path, err := FindPath(g, nodeA, nodeD, 1_000_000)
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}

	// Verify path has 3 hops
	if len(path) != 3 {
		t.Fatalf("expected 3 hops, got %d", len(path))
	}

	// Verify fee compounding iterating backwards
	// D receives: 1,000,000
	// C forwards: 1,000,000, charges fee on that amount
	// B forwards: 1,000,000 + C's fee, charges fee on THAT amount
	// A sends: amount B needs + B's fee
	d_receives := path[2].AmtToForward
	c_fee := path[2].Fee
	c_forwards := path[1].AmtToForward
	b_fee := path[1].Fee
	b_forwards := path[0].AmtToForward
	a_fee := path[0].Fee

	if d_receives != 1_000_000 {
		t.Errorf("Expected 1_000_000 for D receives, got %d", d_receives)
	}
	if c_forwards != d_receives+c_fee {
		t.Errorf("Expected %d for C forwards, got %d", d_receives+c_fee, c_forwards)
	}
	if b_forwards != c_forwards+b_fee {
		t.Errorf("Expected %d for B forwards, got %d", c_forwards+b_fee, b_forwards)
	}
	// Total A sends = B forwards + first hop fee
	totalSent := b_forwards + a_fee
	expectedTotal := 1_000_000 + c_fee + b_fee + a_fee
	if totalSent != expectedTotal {
		t.Errorf("Expected A to send %d, got %d", expectedTotal, totalSent)
	}

	t.Log("Path found:")
	for i, hop := range path {
		t.Logf("  Hop %d: AmtToForward=%d, Fee=%d, CLTV=%d",
			i, hop.AmtToForward, hop.Fee, hop.CLTVDelta)
	}
}

func TestFindPath_ParallelPaths(t *testing.T) {
	// Graph: A -> B (expensive) -> D
	//        A -> C (cheap) -> D
	// Algorithm should pick the cheaper A -> C -> D path

	g := NewGraph()
	nodeA := nodeID('A')
	nodeB := nodeID('B')
	nodeC := nodeID('C')
	nodeD := nodeID('D')

	// Expensive path through B
	g.AddEdge(ChannelEdge{
		ChannelID: 1,
		From:      nodeA,
		To:        nodeB,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:    10000, // High fee
			FeeRatePPM: 1000,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 2,
		From:      nodeB,
		To:        nodeD,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:    10000,
			FeeRatePPM: 1000,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})

	// Cheap path through C
	g.AddEdge(ChannelEdge{
		ChannelID: 3,
		From:      nodeA,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:    100, // Low fee
			FeeRatePPM: 100,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 4,
		From:      nodeC,
		To:        nodeD,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:    100,
			FeeRatePPM: 100,
			CLTVDelta:  40,
			MinHTLC:    1000,
			MaxHTLC:    500_000_000_000,
		},
	})

	path, err := FindPath(g, nodeA, nodeD, 1_000_000)
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}

	// Should go through C, not B
	if len(path) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(path))
	}

	// Verify it picked the cheap path (through C)
	if path[0].ChannelID != 3 {
		t.Errorf("expected first hop through channel 3 (A->C), got channel %d", path[0].ChannelID)
	}
}

func TestFindPath_NoPath(t *testing.T) {
	// Graph with no connection between src and dst
	g := NewGraph()
	nodeA := nodeID('A')
	nodeB := nodeID('B')
	nodeC := nodeID('C')

	// A -> B exists, but no path to C
	g.AddEdge(ChannelEdge{
		ChannelID: 1,
		From:      nodeA,
		To:        nodeB,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})

	_, err := FindPath(g, nodeA, nodeC, 1_000_000)
	if err != ErrNoPath {
		t.Errorf("expected ErrNoPath, got %v", err)
	}
}

func TestFindPath_DisabledChannel(t *testing.T) {
	// Direct path is disabled, must use alternate
	g := NewGraph()
	nodeA := nodeID('A')
	nodeB := nodeID('B')
	nodeC := nodeID('C')

	// Direct A -> C is disabled
	g.AddEdge(ChannelEdge{
		ChannelID: 1,
		From:      nodeA,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   1, // Would be cheapest
			Disabled:  true,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})

	// Alternate path A -> B -> C
	g.AddEdge(ChannelEdge{
		ChannelID: 2,
		From:      nodeA,
		To:        nodeB,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 3,
		From:      nodeB,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})

	path, err := FindPath(g, nodeA, nodeC, 1_000_000)
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}

	// Should use the 2-hop path, not the disabled direct path
	if len(path) != 2 {
		t.Fatalf("expected 2 hops (avoiding disabled channel), got %d", len(path))
	}
}

func TestFindPath_ExceedsMaxHTLC(t *testing.T) {
	// Direct path has MaxHTLC too small, must use alternate
	g := NewGraph()
	nodeA := nodeID('A')
	nodeB := nodeID('B')
	nodeC := nodeID('C')

	// Direct A -> C has small MaxHTLC
	g.AddEdge(ChannelEdge{
		ChannelID: 1,
		From:      nodeA,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   1,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   100_000, // Only 100 sats max - too small!
		},
	})

	// Alternate path A -> B -> C with higher limits
	g.AddEdge(ChannelEdge{
		ChannelID: 2,
		From:      nodeA,
		To:        nodeB,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 3,
		From:      nodeB,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})

	path, err := FindPath(g, nodeA, nodeC, 1_000_000) // 1000 sats > 100 sat limit
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}

	if len(path) != 2 {
		t.Fatalf("expected 2 hops (avoiding MaxHTLC limit), got %d", len(path))
	}
}

func TestFindPath_CLTVTiebreak(t *testing.T) {
	// Two paths with same fee, but different CLTV
	// Should pick the one with lower total CLTV
	g := NewGraph()
	nodeA := nodeID('A')
	nodeB := nodeID('B')
	nodeC := nodeID('C')
	nodeD := nodeID('D')

	// Path through B: same fee, HIGH CLTV
	g.AddEdge(ChannelEdge{
		ChannelID: 1,
		From:      nodeA,
		To:        nodeB,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 144, // High CLTV
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 2,
		From:      nodeB,
		To:        nodeD,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 144,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})

	// Path through C: same fee, LOW CLTV
	g.AddEdge(ChannelEdge{
		ChannelID: 3,
		From:      nodeA,
		To:        nodeC,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40, // Low CLTV
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})
	g.AddEdge(ChannelEdge{
		ChannelID: 4,
		From:      nodeC,
		To:        nodeD,
		Capacity:  1_000_000,
		Policy: ChannelPolicy{
			BaseFee:   100,
			CLTVDelta: 40,
			MinHTLC:   1000,
			MaxHTLC:   500_000_000_000,
		},
	})

	path, err := FindPath(g, nodeA, nodeD, 1_000_000)
	if err != nil {
		t.Fatalf("FindPath() error = %v", err)
	}

	// Should pick path through C (lower CLTV)
	if path[0].ChannelID != 3 {
		t.Errorf("expected path through C (channel 3) for lower CLTV, got channel %d", path[0].ChannelID)
	}

	// Verify total CLTV is 80 (40 + 40), not 288 (144 + 144)
	var totalCLTV uint32
	for _, hop := range path {
		totalCLTV += hop.CLTVDelta
	}
	if totalCLTV != 80 {
		t.Errorf("expected total CLTV 80, got %d", totalCLTV)
	}
}

// =============================================================================
// Edge Filter Tests
// =============================================================================

func TestEdgePassesFilters(t *testing.T) {
	tests := []struct {
		name       string
		edge       ChannelEdge
		amountMsat int64
		want       bool
	}{
		{
			name: "valid edge",
			edge: ChannelEdge{
				Capacity: 1_000_000,
				Policy: ChannelPolicy{
					Disabled: false,
					MinHTLC:  1000,
					MaxHTLC:  500_000_000_000,
				},
			},
			amountMsat: 100_000,
			want:       true,
		},
		{
			name: "disabled edge",
			edge: ChannelEdge{
				Capacity: 1_000_000,
				Policy: ChannelPolicy{
					Disabled: true,
					MinHTLC:  1000,
					MaxHTLC:  500_000_000_000,
				},
			},
			amountMsat: 100_000,
			want:       false,
		},
		{
			name: "below MinHTLC",
			edge: ChannelEdge{
				Capacity: 1_000_000,
				Policy: ChannelPolicy{
					Disabled: false,
					MinHTLC:  100_000,
					MaxHTLC:  500_000_000_000,
				},
			},
			amountMsat: 50_000, // Below min
			want:       false,
		},
		{
			name: "above MaxHTLC",
			edge: ChannelEdge{
				Capacity: 1_000_000,
				Policy: ChannelPolicy{
					Disabled: false,
					MinHTLC:  1000,
					MaxHTLC:  100_000, // Low max
				},
			},
			amountMsat: 200_000, // Above max
			want:       false,
		},
		{
			name: "exceeds capacity",
			edge: ChannelEdge{
				Capacity: 100, // 100 sats = 100,000 msat
				Policy: ChannelPolicy{
					Disabled: false,
					MinHTLC:  1000,
					MaxHTLC:  500_000_000_000,
				},
			},
			amountMsat: 200_000, // 200 sats worth, exceeds capacity
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := edgePassesFilters(tt.edge, tt.amountMsat)
			if got != tt.want {
				t.Errorf("edgePassesFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}
