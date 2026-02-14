package main

import (
	"fmt"
	"strings"
	"time"

	tokenbucket "github.com/kaldun-tech/go-algorithm-practice/rate-limiting/token-bucket"
)

func main() {
	// Example 1: Basic token bucket
	fmt.Println("Example 1: Basic Token Bucket (10 req/sec)")
	tokenBucketBasic()

	fmt.Println("\n" + strings.Repeat("=", 50))

	// Example 2: With burst capacity
	fmt.Println("\nExample 2: Token Bucket with Burst (5 req/sec, burst 10)")
	tokenBucketBurst()

	fmt.Println("\n" + strings.Repeat("=", 50))

	// Example 3: Detailed info
	fmt.Println("\nExample 3: AllowWithInfo - Detailed Rate Limit Information")
	detailedRateLimitInfo()

	fmt.Println("\n" + strings.Repeat("=", 50))

	// Example 4: AllowN for batch operations
	fmt.Println("\nExample 4: AllowN - Batch Operations")
	exampleAllowN()
}

func tokenBucketBasic() {
	// Create token bucket: 10 requests per second
	limiter := tokenbucket.NewTokenBucket(10, time.Second, 0) // burst defaults to rate

	// Simulate requests from a user
	key := "user:alice"

	// First 10 requests should succeed (bucket starts full)
	for i := 1; i <= 12; i++ {
		allowed := limiter.Allow(key)
		if allowed {
			fmt.Printf("Request %2d: ✓ ALLOWED\n", i)
		} else {
			fmt.Printf("Request %2d: ✗ RATE LIMITED\n", i)
		}
	}

	// Wait a bit and try again
	fmt.Println("\nWaiting 500ms (should refill ~5 tokens)...")
	time.Sleep(500 * time.Millisecond)

	for i := 13; i <= 16; i++ {
		allowed := limiter.Allow(key)
		if allowed {
			fmt.Printf("Request %2d: ✓ ALLOWED\n", i)
		} else {
			fmt.Printf("Request %2d: ✗ RATE LIMITED\n", i)
		}
	}
}

func tokenBucketBurst() {
	// Allow bursts up to 10, but only 5 per second sustained
	limiter := tokenbucket.NewTokenBucket(5, time.Second, 10)

	key := "user:bob"

	// Can handle initial burst of 10
	fmt.Println("Initial burst of 10 requests:")
	for i := 1; i <= 12; i++ {
		allowed := limiter.Allow(key)
		if allowed {
			fmt.Printf("Request %2d: ✓ ALLOWED\n", i)
		} else {
			fmt.Printf("Request %2d: ✗ RATE LIMITED\n", i)
		}
	}
}

func detailedRateLimitInfo() {
	limiter := tokenbucket.NewTokenBucket(10, time.Second, 15)

	key := "user:charlie"

	// Make some requests
	for i := 1; i <= 3; i++ {
		result := limiter.AllowWithInfo(key, 1)
		fmt.Printf("Request %d:\n", i)
		fmt.Printf("  Allowed: %v\n", result.Allowed)
		fmt.Printf("  Remaining: %d tokens\n", result.Remaining)
		fmt.Printf("  Reset in: %v\n", time.Until(result.ResetAt).Round(time.Millisecond))
		if !result.Allowed {
			fmt.Printf("  Retry after: %v\n", result.RetryAfter)
		}
		fmt.Println()
	}

	// Drain the bucket
	fmt.Println("Draining bucket...")
	for limiter.Allow(key) {
		// Keep draining
	}

	// Now check what info we get
	result := limiter.AllowWithInfo(key, 1)
	fmt.Println("After draining:")
	fmt.Printf("  Allowed: %v\n", result.Allowed)
	fmt.Printf("  Remaining: %d tokens\n", result.Remaining)
	fmt.Printf("  Retry after: %v\n", result.RetryAfter.Round(time.Millisecond))
	fmt.Printf("  Reset at: %v (in %v)\n",
		result.ResetAt.Format("15:04:05"),
		time.Until(result.ResetAt).Round(time.Millisecond))
}

func exampleAllowN() {
	limiter := tokenbucket.NewTokenBucket(100, time.Minute, 100)

	key := "user:david"

	// Normal request costs 1 token
	allowed := limiter.AllowN(key, 1)
	fmt.Printf("Single request (1 token): %v\n", allowed)

	// Batch upload costs 10 tokens
	allowed = limiter.AllowN(key, 10)
	fmt.Printf("Batch upload (10 tokens): %v\n", allowed)

	// Expensive operation costs 50 tokens
	allowed = limiter.AllowN(key, 50)
	fmt.Printf("Expensive operation (50 tokens): %v\n", allowed)

	// Check remaining
	result := limiter.AllowWithInfo(key, 0) // 0 tokens = just check, don't consume
	fmt.Printf("Remaining tokens: %d\n", result.Remaining)

	// Try another expensive operation (should fail - only ~39 tokens left)
	allowed = limiter.AllowN(key, 50)
	fmt.Printf("Another expensive operation (50 tokens): %v\n", allowed)
}
