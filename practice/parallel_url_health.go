package practice

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// Build a CLI tool in Go that takes a list of URLs and checks their health concurrently:
// - Fan-out: distribute URLs across N workers
// - Worker pool: fixed goroutine count (configurable, say 5)
// - Fan-in: collect results back into a single results channel
// - Each worker does an HTTP GET and reports status code + latency
// - Graceful shutdown via context cancellation if any worker hits a timeout
// - Print a summary at the end — total checked, how many healthy, avg latency

// Input Scale
// - 10–500 URLs, provided as a text file (one URL per line)
// - Assume valid URL format, no need to validate syntax
// - Pool size N is a CLI flag, default 5

// Complexity
// - Time: O(URLs / N) wall clock, bounded by network I/O not CPU — don't overthink this, it's not an algorithmic problem
// - Space: O(N) for the worker pool plus O(URLs) for results — keep result collection lean, don't buffer everything in memory if you can avoid it

// Concurrency
// - Fixed worker pool — no spawning unbounded goroutines
// - Context with timeout per request (say 5s), and a top-level context for graceful shutdown on interrupt (SIGINT)

// Error Handling
// - Network errors and timeouts should be captured and reported, not crash the program
// - A URL that fails is a result, not an exception — treat it as status "ERROR" with the error message
// - No retries for this pass (you can add as an extension if time permits)

// Outputs
// - Per-URL: url, status code, latency in ms (or ERROR + reason)
// - Summary: total, healthy (2xx), errors, average latency of successful checks

type Result struct {
	URL        string
	StatusCode int
	LatencyMs  int64
	Err        error
}

type Summary struct {
	Total      int
	Healthy    int
	Errors     int
	AvgLatency float64
}

var (
	inputFile string
	poolSize  int
)

var rootCmd = &cobra.Command{
	Use:     "health",
	Short:   "Parallel Health Check CLI",
	Long:    "health is a parallel URL health checker that distributes work across a worker pool",
	Version: "0.1.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		urls, err := readURLsFromFile(inputFile)
		if err != nil {
			return err
		}

		results, err := RunChecker(ctx, urls, poolSize)
		if err != nil {
			return err
		}

		summary := ComputeSummary(results)
		printResults(results, summary)
		return nil
	},
}

func init() {
	rootCmd.Flags().StringVarP(&inputFile, "file", "f", "testdata/urls.txt", "path to URL file")
	rootCmd.Flags().IntVarP(&poolSize, "pool", "p", 5, "number of worker goroutines")
}

func readURLsFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var urls []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

func printResults(results []Result, summary Summary) {
	fmt.Println("=== Results ===")
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("%s: ERROR - %v\n", r.URL, r.Err)
		} else {
			fmt.Printf("%s: %d (%dms)\n", r.URL, r.StatusCode, r.LatencyMs)
		}
	}
	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total: %d, Healthy: %d, Errors: %d, Avg Latency: %.2fms\n",
		summary.Total, summary.Healthy, summary.Errors, summary.AvgLatency)
}

func checkUrl(ctx context.Context, url string) Result {
	// Set up per-request timeout, respecting parent cancellation
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	start := time.Now()

	// Create a request using context for cancellation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Result{URL: url, Err: err}
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{URL: url, Err: err, LatencyMs: time.Since(start).Milliseconds()}
	}
	defer resp.Body.Close()

	// Success response
	return Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		LatencyMs:  time.Since(start).Milliseconds(),
	}
}

func RunChecker(ctx context.Context, urls []string, poolSize int) ([]Result, error) {
	// ctx already has signal cancellation from caller
	// errgroup wraps it so any worker error also cancels siblings
	eg, ctx := errgroup.WithContext(ctx)

	// The input channel feeding URLs to workers. This can be small or even unbuffered since main is feeding it and workers are consuming.
	// A buffer of `poolSize` is common just to keep workers from blocking on each other.
	jobs := make(chan string, poolSize)
	// The output channel collecting from workers. Buffering this at `len(urls)`
	// is the clean move so workers never block on writing results, and you drain it after all workers finish.
	results := make(chan Result, len(urls))

	// Spawn a fixed size worker pool. These are consumers of jobs, and producers of results
	for range poolSize {
		eg.Go(func() error {
			// Workers pull from a shared channel in jobs
			for url := range jobs {
				select {
				case <-ctx.Done():
					return ctx.Err() // Bail on SIGINT or sibling error
				default:
					results <- checkUrl(ctx, url)
				}
			}
			return nil
		})
	}

	// Feed (producer of) jobs in a separate goroutine so we don't block
	go func() {
		defer close(jobs)
		for _, url := range urls {
			select {
			case jobs <- url:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to finish
	err := eg.Wait()
	close(results) // Now safe - all producers done

	// Drain results into slice
	var out []Result
	for r := range results {
		out = append(out, r)
	}

	return out, err
}

func ComputeSummary(results []Result) Summary {
	var sum Summary
	healthyLatency := int64(0)
	for _, r := range results {
		sum.Total++
		if r.Err != nil {
			sum.Errors++
		} else if 200 <= r.StatusCode && r.StatusCode < 300 {
			sum.Healthy++
			healthyLatency += r.LatencyMs
		}
		// 4xx/5xx codes are counted in Total but neither Healthy nor Errors
	}

	if sum.Healthy > 0 {
		sum.AvgLatency = float64(healthyLatency) / float64(sum.Healthy)
	}

	return sum
}

// Execute runs the health checker CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
