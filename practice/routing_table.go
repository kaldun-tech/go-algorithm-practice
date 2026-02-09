package practice

import (
	"encoding/binary"
	"net"
)

// Think through:
// 1. How do you represent each node? (children map, isRoute bool, gateway string)
// 2. CIDR parsing: how do you convert "192.168.1.0/24" to usable format?
// 3. Longest prefix match: how do you track "best match so far"?

// TrieNode represents a single bit in the IP address path
type TrieNode struct {
	// Children for 0 and 1 bits
	child [2]*TrieNode
	// Gateway for this route (empty string means no route ends here)
	gateway string
}

type RoutingTable struct {
	root *TrieNode
}

func NewRoutingTable() *RoutingTable {
	// Initialize with empty root
	return &RoutingTable{root: &TrieNode{}}
}

// Converts IPv4 address to uint32 for traversal
func ipv4ToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}

// Extracts a bit from an address at the specified position
func getBit(addr uint32, pos int) uint32 {
	return (addr >> uint(pos)) & 1
}

// AddRoute adds a CIDR route to the routing table
// Example: AddRoute("10.0.0.0/8", "gw1")
func (rt *RoutingTable) AddRoute(cidr string, gateway string) error {
	// Step 1: Parse CIDR using net.ParseCIDR
	//   - Returns *net.IPNet which contains IP and Mask
	//   - The mask tells you how many bits are significant
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	ones, _ := ipNet.Mask.Size()
	addr := ipv4ToUint32(ipNet.IP)

	// Step 2: Convert IP to bits and walk the trie
	//   - For each bit in the prefix (up to mask length):
	//   - Create child node if it doesn't exist
	//   - Move to that child
	n := rt.root
	for i := 31; 32-ones <= i; i-- {
		bit := getBit(addr, i)
		if n.child[bit] == nil {
			n.child[bit] = &TrieNode{}
		}
		n = n.child[bit]
	}

	// Step 3: Mark final node as route destination with gateway
	n.gateway = gateway

	return nil
}

// FindRoute finds the gateway for the given IP using longest prefix match
// Example: FindRoute("10.1.5.7") -> "gw2"
func (rt *RoutingTable) FindRoute(ip string) string {
	// Step 1: Parse IP using net.ParseIP (not ParseCIDR - this is a plain IP)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	addr := ipv4ToUint32(parsed)

	// Step 2: Walk trie bit by bit
	//   - At each node, if it's a route endpoint, remember it as "best match"
	//   - Continue walking as long as children exist
	n := rt.root
	best := ""
	for i := 31; 0 <= i && n != nil; i-- {
		if n.gateway != "" {
			best = n.gateway
		}
		bit := getBit(addr, i)
		n = n.child[bit]
	}
	// Check final node (for /32 routes)
	if n != nil && n.gateway != "" {
		best = n.gateway
	}

	// Step 3: Return the last (most specific) gateway seen
	return best
}
