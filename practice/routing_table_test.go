package practice

import "testing"

// =============================================================================
// Basic functionality tests
// =============================================================================

func TestRoutingTable_AddAndFindSingleRoute(t *testing.T) {
	rt := NewRoutingTable()

	err := rt.AddRoute("10.0.0.0/8", "gw1")
	if err != nil {
		t.Fatalf("AddRoute failed: %v", err)
	}

	got := rt.FindRoute("10.5.5.5")
	if got != "gw1" {
		t.Errorf("expected gw1, got %s", got)
	}
}

func TestRoutingTable_NoMatchReturnsEmpty(t *testing.T) {
	rt := NewRoutingTable()

	rt.AddRoute("10.0.0.0/8", "gw1")

	got := rt.FindRoute("192.168.1.1")
	if got != "" {
		t.Errorf("expected empty string for no match, got %s", got)
	}
}

// =============================================================================
// Longest prefix match tests
// =============================================================================

func TestRoutingTable_LongestPrefixMatch(t *testing.T) {
	rt := NewRoutingTable()

	// Add routes in order from less to more specific
	rt.AddRoute("10.0.0.0/8", "gw1")
	rt.AddRoute("10.1.0.0/16", "gw2")
	rt.AddRoute("10.1.1.0/24", "gw3")

	tests := []struct {
		ip       string
		expected string
	}{
		{"10.1.1.5", "gw3"},   // Matches /24
		{"10.1.5.7", "gw2"},   // Matches /16
		{"10.2.5.7", "gw1"},   // Matches /8 only
		{"192.168.1.1", ""},   // No match
	}

	for _, tc := range tests {
		got := rt.FindRoute(tc.ip)
		if got != tc.expected {
			t.Errorf("FindRoute(%s): expected %s, got %s", tc.ip, tc.expected, got)
		}
	}
}

func TestRoutingTable_OverlappingRoutesAddedOutOfOrder(t *testing.T) {
	// Challenge: ensure longest prefix wins even when added out of order
	rt := NewRoutingTable()

	// Add more specific first
	rt.AddRoute("10.1.0.0/16", "gw2")
	rt.AddRoute("10.0.0.0/8", "gw1")

	got := rt.FindRoute("10.1.5.7")
	if got != "gw2" {
		t.Errorf("expected gw2 (more specific), got %s", got)
	}

	got = rt.FindRoute("10.2.5.7")
	if got != "gw1" {
		t.Errorf("expected gw1 (less specific fallback), got %s", got)
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestRoutingTable_ExactHostRoute(t *testing.T) {
	rt := NewRoutingTable()

	// /32 is a single host
	rt.AddRoute("10.1.1.1/32", "gw-host")
	rt.AddRoute("10.1.1.0/24", "gw-network")

	got := rt.FindRoute("10.1.1.1")
	if got != "gw-host" {
		t.Errorf("expected gw-host for exact match, got %s", got)
	}

	got = rt.FindRoute("10.1.1.2")
	if got != "gw-network" {
		t.Errorf("expected gw-network for network match, got %s", got)
	}
}

func TestRoutingTable_DefaultRoute(t *testing.T) {
	rt := NewRoutingTable()

	// 0.0.0.0/0 is the default route (matches everything)
	rt.AddRoute("0.0.0.0/0", "default-gw")
	rt.AddRoute("10.0.0.0/8", "private-gw")

	got := rt.FindRoute("10.5.5.5")
	if got != "private-gw" {
		t.Errorf("expected private-gw for 10.x.x.x, got %s", got)
	}

	got = rt.FindRoute("8.8.8.8")
	if got != "default-gw" {
		t.Errorf("expected default-gw for public IP, got %s", got)
	}
}

func TestRoutingTable_InvalidCIDR(t *testing.T) {
	rt := NewRoutingTable()

	err := rt.AddRoute("not-a-cidr", "gw1")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestRoutingTable_InvalidIP(t *testing.T) {
	rt := NewRoutingTable()
	rt.AddRoute("10.0.0.0/8", "gw1")

	got := rt.FindRoute("not-an-ip")
	if got != "" {
		t.Errorf("expected empty for invalid IP, got %s", got)
	}
}
