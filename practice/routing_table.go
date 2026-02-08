package practice

import (
	"net"
)

// Think through:
// 1. How do you represent each node? (children map, isRoute bool, gateway string)
// 2. CIDR parsing: how do you convert "192.168.1.0/24" to usable format?
// 3. Longest prefix match: how do you track "best match so far"?

// TrieNode represents a single bit in the IP address path
type TrieNode struct {
	// Children for 0 and 1 bits
	// Is this node a route endpoint?
	// What gateway does this route point to?
}

type RoutingTable struct {
	root *TrieNode
}

func NewRoutingTable() *RoutingTable {
	// Initialize with empty root
	return nil
}

// AddRoute adds a CIDR route to the routing table
// Example: AddRoute("10.0.0.0/8", "gw1")
func (rt *RoutingTable) AddRoute(cidr string, gateway string) error {
	// Step 1: Parse CIDR using net.ParseCIDR
	//   - Returns *net.IPNet which contains IP and Mask
	//   - The mask tells you how many bits are significant
	//
	// Step 2: Convert IP to bits and walk the trie
	//   - For each bit in the prefix (up to mask length):
	//   - Create child node if it doesn't exist
	//   - Move to that child
	//
	// Step 3: Mark final node as route destination with gateway

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	// ipNet.IP is the network address
	// ipNet.Mask tells you the prefix length
	// ones, _ := ipNet.Mask.Size() gives you the number of significant bits

	_ = ipNet // Remove when you use it
	return nil
}

// FindRoute finds the gateway for the given IP using longest prefix match
// Example: FindRoute("10.1.5.7") -> "gw2"
func (rt *RoutingTable) FindRoute(ip string) string {
	// Step 1: Parse IP using net.ParseIP
	//
	// Step 2: Walk trie bit by bit
	//   - At each node, if it's a route endpoint, remember it as "best match"
	//   - Continue walking as long as children exist
	//
	// Step 3: Return the last (most specific) gateway seen

	return ""
}

// Helper: Get bit at position pos from IP address bytes
// pos 0 is the most significant bit of the first byte
func getBit(ip net.IP, pos int) int {
	// ip is a []byte, typically 4 bytes for IPv4
	// To get bit at position pos:
	//   byteIndex := pos / 8
	//   bitIndex := 7 - (pos % 8)  // bits numbered from MSB
	//   return (ip[byteIndex] >> bitIndex) & 1
	return 0
}
