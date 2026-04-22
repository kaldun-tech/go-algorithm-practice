package practice

import (
	"container/heap"
	"errors"
	"math"
)

// Payment Channel Network Router (Lightning)
// Implements backwards-Dijkstra pathfinding for Lightning Network payments.
//
// Key insight: We run from destination -> source because fees depend on the
// amount being forwarded, which we only know working backwards from the
// final payment amount.
//
// Reflection prompts:
// - Time complexity: O(|E| log |V|) — same as standard Dijkstra
// - Why heap key is amountToSend not totalFee: fees compound, so we minimize total outlay
// - What breaks if you run forwards: can't compute fees without knowing downstream amounts
// - Reliability extension: multiply amountToSend by (1/probability) penalty

var (
	ErrNoPath        = errors.New("no path found")
	ErrSameNode      = errors.New("source and destination are the same")
	ErrInvalidAmount = errors.New("invalid payment amount")
)

// NodeID represents a 33-byte compressed public key
type NodeID [33]byte

// ChannelPolicy defines the fee structure and constraints for one direction of a channel
type ChannelPolicy struct {
	BaseFee    int64  // fixed fee in msat
	FeeRatePPM int64  // proportional fee in parts per million
	CLTVDelta  uint32 // timelock delta in blocks
	Disabled   bool   // whether this direction is disabled
	MinHTLC    int64  // minimum payment size in msat
	MaxHTLC    int64  // maximum payment size in msat
}

// ChannelEdge represents a directed edge in the channel graph
type ChannelEdge struct {
	ChannelID uint64
	From, To  NodeID
	Capacity  int64 // announced max in satoshis
	Policy    ChannelPolicy
}

// Graph holds the channel network as an adjacency list
type Graph struct {
	// Common Go idiom: use map[Type]struct{} for the set of nodes
	Nodes map[NodeID]struct{}
	// Edges maps NodeID -> outbound edges from that node
	Edges map[NodeID][]ChannelEdge
}

// NewGraph creates an empty channel graph
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[NodeID]struct{}),
		Edges: make(map[NodeID][]ChannelEdge),
	}
}

// AddEdge adds a directed channel edge to the graph
func (g *Graph) AddEdge(edge ChannelEdge) {
	g.Nodes[edge.From] = struct{}{}
	g.Nodes[edge.To] = struct{}{}
	g.Edges[edge.From] = append(g.Edges[edge.From], edge)
}

// Hop represents one step in a payment route
type Hop struct {
	NodeID       NodeID
	ChannelID    uint64
	AmtToForward int64  // msat — what this node forwards onward
	Fee          int64  // msat — fee this node charges
	CLTVDelta    uint32 // timelock delta this hop requires
}

// pathNode is used in the priority queue for Dijkstra's algorithm
type pathNode struct {
	node         NodeID
	amountToSend int64  // total amount source must send to reach destination
	totalCLTV    uint32 // total CLTV delta accumulated (for tie-breaking)
	index        int    // index in heap (for heap.Fix)
}

// pathHeap implements heap.Interface for Dijkstra's priority queue
type pathHeap []*pathNode

func (h pathHeap) Len() int { return len(h) }

// Less compares two path nodes for the priority queue
// Primary sort by amountToSend, secondary by totalCLTV
// Part of Go's heap.Interface
// Answers: Should element at index i come before element at index j?
func (h pathHeap) Less(i, j int) bool {
	if h[i].amountToSend != h[j].amountToSend {
		return h[i].amountToSend < h[j].amountToSend
	}
	return h[i].totalCLTV < h[j].totalCLTV
}

func (h pathHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *pathHeap) Push(x any) {
	n := len(*h)
	node := x.(*pathNode)
	node.index = n
	*h = append(*h, node)
}

func (h *pathHeap) Pop() any {
	old := *h
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.index = -1
	*h = old[0 : n-1]
	return node
}

// Ensure pathHeap implements heap.Interface
var _ heap.Interface = (*pathHeap)(nil)

// hopFee calculates the fee for forwarding amountMsat through a channel
// Formula: base_fee + (amount * fee_rate_ppm / 1_000_000)
func hopFee(policy ChannelPolicy, amountMsat int64) int64 {
	// Implement fee calculation: Start with base fee
	fee := policy.BaseFee
	// Add proportional fee: amount * feeRatePPM / 1_000_000
	fee += (amountMsat * int64(policy.FeeRatePPM)) / 1_000_000
	return fee
}

// edgePassesFilters checks if an edge can carry the given amount
func edgePassesFilters(edge ChannelEdge, amountMsat int64) bool {
	// Check all constraints: Edge not disabled
	if edge.Policy.Disabled {
		return false
	}
	// Amount >= MinHTLC
	if amountMsat < edge.Policy.MinHTLC {
		return false
	}
	// Amount <= MaxHTLC
	if amountMsat > edge.Policy.MaxHTLC {
		return false
	}
	// Amount <= Capacity * 1000 (convert sats to msat)
	if amountMsat > int64(edge.Capacity)*1000 {
		return false
	}
	return true
}

// prevHop tracks the path during Dijkstra traversal
type prevHop struct {
	node      NodeID
	edge      ChannelEdge
	amount    int64  // amount at this point in the path
	totalCLTV uint32 // accumulated CLTV at this point
}

// FindPath finds the lowest-cost path from src to dst for the given amount.
// Uses backwards Dijkstra starting from dst and working towards src.
// Returns the path as a slice of Hops from src towards dst.
func FindPath(g *Graph, src, dst NodeID, amountMsat int64) ([]Hop, error) {
	// Step 1: Build reverse graph (incoming edges for each node)
	// For each edge A->B, we need to find it when processing node B
	reverse := buildReverseGraph(g)

	// Step 2: Initialize
	// dist is the distance map representing the minimum amount you'd need
	// to send from that node to deliver amountMsat to the destination
	dist := make(map[NodeID]int64)
	// Destination receives exact amount
	dist[dst] = amountMsat
	// All other nodes start with MaxInt64
	for nodeID := range g.Nodes {
		if nodeID != dst {
			dist[nodeID] = math.MaxInt64
		}
	}

	// prev map for path reconstruction
	prev := make(map[NodeID]prevHop)

	// Initialize the path heap
	h := &pathHeap{}
	heap.Init(h)
	dstPathNode := &pathNode{
		node:         dst,
		amountToSend: amountMsat,
		totalCLTV:    0,
	}
	heap.Push(h, dstPathNode)

	// Map to track nodes in heap for updates
	inHeap := make(map[NodeID]*pathNode)
	inHeap[dst] = dstPathNode

	// Step 3: Dijkstra loop
	// Pop minimum amountToSend node from heap
	for h.Len() > 0 {
		cur := heap.Pop(h).(*pathNode)
		delete(inHeap, cur.node)

		if cur.node == src {
			// Step 4: If it's src, we found the path - reconstruct and return
			// Dijkstra guarantees that when you pop a node you found the optimal path
			return reconstructPath(prev, src, dst, amountMsat), nil
		}

		for _, edge := range reverse[cur.node] {
			// If better path found, push or update heap
			incomingAmount := cur.amountToSend + hopFee(edge.Policy, cur.amountToSend)
			newCLTV := cur.totalCLTV + edge.Policy.CLTVDelta

			if edgePassesFilters(edge, incomingAmount) && incomingAmount < dist[edge.From] {
				dist[edge.From] = incomingAmount
				// Record in prev map
				prev[edge.From] = prevHop{
					node:      cur.node,
					edge:      edge,
					amount:    incomingAmount,
					totalCLTV: newCLTV,
				}

				// Push/update heap
				if existing, ok := inHeap[edge.From]; ok {
					// Already in heap - update in place and fix
					existing.amountToSend = incomingAmount
					existing.totalCLTV = newCLTV
					heap.Fix(h, existing.index)
				} else {
					// Not in heap - push new
					pn := &pathNode{
						node:         edge.From,
						amountToSend: incomingAmount,
						totalCLTV:    newCLTV,
					}
					heap.Push(h, pn)
					inHeap[edge.From] = pn
				}
				// Alternative: allow duplicates and skip stale ones when popping
				// cur := heap.Pop(h).(*pathNode)
				// Skip if we already found a better path to this node
				// if cur.amountToSend > dist[cur.node] { continue }
			}
		}
	}

	// Loop finished without finding src - no path exists
	return nil, ErrNoPath
}

// buildReverseGraph creates a mapping from each node to its incoming edges
// Returns a map rather than a *Graph to avoid unnecessary allocations
func buildReverseGraph(g *Graph) map[NodeID][]ChannelEdge {
	// For each edge From->To in g, add it to reverse[To]
	reverse := make(map[NodeID][]ChannelEdge)
	for _, edges := range g.Edges {
		for _, edge := range edges {
			reverse[edge.To] = append(reverse[edge.To], edge)
		}
	}
	return reverse
}

// reconstructPath builds the hop list from the prev map
// the prev map records how we reached each node during the Dijkstra traversal
// It's the breadcrumb trail for reconstructing the final path.
// prev[NodeA].node = the next node towards the destination (because we traversed backwards from dst to src)
// So when reconstructing, we start at the src and follow the prev pointers forward to dst
func reconstructPath(prev map[NodeID]prevHop, src, dst NodeID, amountMsat int64) []Hop {
	// Walk from src to dst using prev map
	var hops []Hop

	for cur := src; cur != dst; {
		ph := prev[cur]
		edge := ph.edge
		// What this node receives (stored in prev)
		amountIn := ph.amount
		// What this node forwards (amount in next node)
		var amountOut int64
		if ph.node == dst {
			amountOut = amountMsat // Original payment amount
		} else {
			amountOut = prev[ph.node].amount
		}

		hops = append(hops, Hop{
			NodeID:    ph.node,
			ChannelID: edge.ChannelID,
			// Amount the next node receives
			AmtToForward: amountOut,
			// Fee is the difference between what this node receives and forwards
			Fee: amountIn - amountOut,
			// CLTVDelta: from the edge policy
			CLTVDelta: edge.Policy.CLTVDelta,
		})

		cur = ph.node
	}

	return hops
}
