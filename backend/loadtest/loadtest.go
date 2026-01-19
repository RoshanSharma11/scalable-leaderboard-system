package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// LoadTestConfig contains configuration for the load test
type LoadTestConfig struct {
	BaseURL           string
	Duration          time.Duration
	ReadConcurrency   int
	WriteConcurrency  int
	SearchConcurrency int
	RampUpTime        time.Duration
	SpikeTest         bool
	SpikeDuration     time.Duration
	SpikeMultiplier   int
}

// LatencyMetrics tracks detailed latency statistics
type LatencyMetrics struct {
	samples []time.Duration
	mu      sync.Mutex
}

func NewLatencyMetrics() *LatencyMetrics {
	return &LatencyMetrics{
		samples: make([]time.Duration, 0, 100000),
	}
}

func (lm *LatencyMetrics) Record(d time.Duration) {
	lm.mu.Lock()
	lm.samples = append(lm.samples, d)
	lm.mu.Unlock()
}

func (lm *LatencyMetrics) Calculate() map[string]interface{} {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if len(lm.samples) == 0 {
		return map[string]interface{}{}
	}

	sorted := make([]time.Duration, len(lm.samples))
	copy(sorted, lm.samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	count := len(sorted)
	min := sorted[0]
	max := sorted[count-1]
	p50 := sorted[int(float64(count)*0.50)]
	p90 := sorted[int(float64(count)*0.90)]
	p95 := sorted[int(float64(count)*0.95)]
	p99 := sorted[int(float64(count)*0.99)]
	p999 := sorted[int(float64(count)*0.999)]

	// Calculate mean
	var sum time.Duration
	for _, s := range sorted {
		sum += s
	}
	mean := sum / time.Duration(count)

	// Calculate standard deviation
	var variance float64
	for _, s := range sorted {
		diff := float64(s - mean)
		variance += diff * diff
	}
	stddev := time.Duration(math.Sqrt(variance / float64(count)))

	return map[string]interface{}{
		"count":  count,
		"min":    min,
		"mean":   mean,
		"stddev": stddev,
		"p50":    p50,
		"p90":    p90,
		"p95":    p95,
		"p99":    p99,
		"p999":   p999,
		"max":    max,
	}
}

// TestResults stores results of the load test
type TestResults struct {
	ReadOps      uint64
	WriteOps     uint64
	SearchOps    uint64
	ReadErrors   uint64
	WriteErrors  uint64
	SearchErrors uint64

	ReadLatency   *LatencyMetrics
	WriteLatency  *LatencyMetrics
	SearchLatency *LatencyMetrics

	Duration time.Duration
}

func main() {
	// Parse flags
	baseURL := flag.String("url", "http://localhost:8080", "Base URL of the service")
	duration := flag.Duration("duration", 30*time.Second, "Test duration")
	reads := flag.Int("reads", 100, "Number of concurrent read goroutines")
	writes := flag.Int("writes", 10, "Number of concurrent write goroutines (simulated)")
	searches := flag.Int("searches", 20, "Number of concurrent search goroutines")
	rampUp := flag.Duration("rampup", 5*time.Second, "Ramp-up time")
	spike := flag.Bool("spike", false, "Enable spike test")
	spikeDuration := flag.Duration("spike-duration", 10*time.Second, "Duration of spike")
	spikeMultiplier := flag.Int("spike-multiplier", 5, "Spike multiplier")

	flag.Parse()

	config := LoadTestConfig{
		BaseURL:           *baseURL,
		Duration:          *duration,
		ReadConcurrency:   *reads,
		WriteConcurrency:  *writes,
		SearchConcurrency: *searches,
		RampUpTime:        *rampUp,
		SpikeTest:         *spike,
		SpikeDuration:     *spikeDuration,
		SpikeMultiplier:   *spikeMultiplier,
	}

	log.Println("╔══════════════════════════════════════════════════════════════╗")
	log.Println("║       LEADERBOARD LOAD TESTING TOOL                          ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")
	log.Println()
	log.Printf("Configuration:")
	log.Printf("  Target URL:            %s", config.BaseURL)
	log.Printf("  Test Duration:         %v", config.Duration)
	log.Printf("  Read Concurrency:      %d", config.ReadConcurrency)
	log.Printf("  Write Concurrency:     %d (simulated)", config.WriteConcurrency)
	log.Printf("  Search Concurrency:    %d", config.SearchConcurrency)
	log.Printf("  Ramp-up Time:          %v", config.RampUpTime)
	log.Printf("  Spike Test:            %v", config.SpikeTest)
	if config.SpikeTest {
		log.Printf("  Spike Duration:        %v", config.SpikeDuration)
		log.Printf("  Spike Multiplier:      %dx", config.SpikeMultiplier)
	}
	log.Println()

	// Check if service is available
	log.Println("Checking service availability...")
	resp, err := http.Get(config.BaseURL + "/health")
	if err != nil {
		log.Fatalf("Service not available: %v", err)
	}
	resp.Body.Close()
	log.Println("✓ Service is healthy")
	log.Println()

	// Run load test
	results := runLoadTest(config)

	// Print results
	printResults(results, config)
}

func runLoadTest(config LoadTestConfig) *TestResults {
	results := &TestResults{
		ReadLatency:   NewLatencyMetrics(),
		WriteLatency:  NewLatencyMetrics(),
		SearchLatency: NewLatencyMetrics(),
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	spike := make(chan bool, 1)

	startTime := time.Now()

	// Start read workers
	log.Printf("Starting %d read workers...", config.ReadConcurrency)
	for i := 0; i < config.ReadConcurrency; i++ {
		wg.Add(1)
		go readWorker(&wg, config.BaseURL, results, stop, spike, i, config.RampUpTime, config.ReadConcurrency)
	}

	// Start search workers
	log.Printf("Starting %d search workers...", config.SearchConcurrency)
	for i := 0; i < config.SearchConcurrency; i++ {
		wg.Add(1)
		go searchWorker(&wg, config.BaseURL, results, stop, spike, i, config.RampUpTime, config.SearchConcurrency)
	}

	log.Println("Load test started!")
	log.Println()

	// Progress reporter
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				reads := atomic.LoadUint64(&results.ReadOps)
				searches := atomic.LoadUint64(&results.SearchOps)
				readErrs := atomic.LoadUint64(&results.ReadErrors)
				searchErrs := atomic.LoadUint64(&results.SearchErrors)

				rps := float64(reads) / elapsed.Seconds()
				sps := float64(searches) / elapsed.Seconds()

				log.Printf("[%v] Reads: %d (%.0f/s, %d errors) | Searches: %d (%.0f/s, %d errors)",
					elapsed.Round(time.Second), reads, rps, readErrs, searches, sps, searchErrs)
			}
		}
	}()

	// Run spike test if enabled
	if config.SpikeTest {
		normalDuration := config.Duration - config.SpikeDuration
		time.Sleep(normalDuration)

		log.Println()
		log.Printf("🔥 INITIATING SPIKE TEST (%dx traffic for %v)...", config.SpikeMultiplier, config.SpikeDuration)
		spike <- true

		// Start additional spike workers
		spikeWorkers := (config.ReadConcurrency + config.SearchConcurrency) * (config.SpikeMultiplier - 1)
		log.Printf("Spawning %d additional workers...", spikeWorkers)

		for i := 0; i < spikeWorkers/2; i++ {
			wg.Add(1)
			go readWorker(&wg, config.BaseURL, results, stop, spike, i+10000, 0, 1)
		}
		for i := 0; i < spikeWorkers/2; i++ {
			wg.Add(1)
			go searchWorker(&wg, config.BaseURL, results, stop, spike, i+10000, 0, 1)
		}

		time.Sleep(config.SpikeDuration)
	} else {
		time.Sleep(config.Duration)
	}

	// Stop all workers
	log.Println()
	log.Println("Stopping workers...")
	close(stop)
	wg.Wait()

	results.Duration = time.Since(startTime)
	log.Println("Load test completed!")
	log.Println()

	return results
}

func readWorker(wg *sync.WaitGroup, baseURL string, results *TestResults, stop chan struct{}, spike chan bool, id int, rampUp time.Duration, totalWorkers int) {
	defer wg.Done()

	// Stagger start time for ramp-up
	if rampUp > 0 {
		delay := time.Duration(float64(rampUp) * float64(id) / float64(totalWorkers))
		time.Sleep(delay)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	limits := []int{10, 50, 100}

	for {
		select {
		case <-stop:
			return
		default:
			limit := limits[id%len(limits)]
			url := fmt.Sprintf("%s/leaderboard?limit=%d", baseURL, limit)

			start := time.Now()
			resp, err := client.Get(url)
			latency := time.Since(start)

			if err != nil {
				atomic.AddUint64(&results.ReadErrors, 1)
			} else {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					atomic.AddUint64(&results.ReadOps, 1)
					results.ReadLatency.Record(latency)
				} else {
					atomic.AddUint64(&results.ReadErrors, 1)
				}
			}

			// Small delay to avoid overwhelming the system
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func searchWorker(wg *sync.WaitGroup, baseURL string, results *TestResults, stop chan struct{}, spike chan bool, id int, rampUp time.Duration, totalWorkers int) {
	defer wg.Done()

	// Stagger start time for ramp-up
	if rampUp > 0 {
		delay := time.Duration(float64(rampUp) * float64(id) / float64(totalWorkers))
		time.Sleep(delay)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	queries := []string{"user", "rahul", "kumar", "test", "amit", "priya"}

	for {
		select {
		case <-stop:
			return
		default:
			query := queries[id%len(queries)]
			url := fmt.Sprintf("%s/search?query=%s", baseURL, query)

			start := time.Now()
			resp, err := client.Get(url)
			latency := time.Since(start)

			if err != nil {
				atomic.AddUint64(&results.SearchErrors, 1)
			} else {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					atomic.AddUint64(&results.SearchOps, 1)
					results.SearchLatency.Record(latency)
				} else {
					atomic.AddUint64(&results.SearchErrors, 1)
				}
			}

			time.Sleep(5 * time.Millisecond)
		}
	}
}

func printResults(results *TestResults, config LoadTestConfig) {
	log.Println("╔══════════════════════════════════════════════════════════════╗")
	log.Println("║                     TEST RESULTS                             ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")
	log.Println()

	totalOps := results.ReadOps + results.SearchOps
	totalErrors := results.ReadErrors + results.SearchErrors
	errorRate := float64(totalErrors) / float64(totalOps+totalErrors) * 100

	log.Printf("Overall Metrics:")
	log.Printf("  Duration:              %v", results.Duration.Round(time.Millisecond))
	log.Printf("  Total Operations:      %d", totalOps)
	log.Printf("  Total Errors:          %d (%.2f%%)", totalErrors, errorRate)
	log.Printf("  Overall Throughput:    %.0f ops/sec", float64(totalOps)/results.Duration.Seconds())
	log.Println()

	log.Printf("Read Operations:")
	log.Printf("  Total:                 %d", results.ReadOps)
	log.Printf("  Errors:                %d", results.ReadErrors)
	log.Printf("  Throughput:            %.0f reads/sec", float64(results.ReadOps)/results.Duration.Seconds())

	if results.ReadOps > 0 {
		readStats := results.ReadLatency.Calculate()
		log.Printf("  Latency:")
		log.Printf("    Min:                 %v", readStats["min"])
		log.Printf("    Mean:                %v", readStats["mean"])
		log.Printf("    P50:                 %v", readStats["p50"])
		log.Printf("    P90:                 %v", readStats["p90"])
		log.Printf("    P95:                 %v", readStats["p95"])
		log.Printf("    P99:                 %v", readStats["p99"])
		log.Printf("    P99.9:               %v", readStats["p999"])
		log.Printf("    Max:                 %v", readStats["max"])
	}
	log.Println()

	log.Printf("Search Operations:")
	log.Printf("  Total:                 %d", results.SearchOps)
	log.Printf("  Errors:                %d", results.SearchErrors)
	log.Printf("  Throughput:            %.0f searches/sec", float64(results.SearchOps)/results.Duration.Seconds())

	if results.SearchOps > 0 {
		searchStats := results.SearchLatency.Calculate()
		log.Printf("  Latency:")
		log.Printf("    Min:                 %v", searchStats["min"])
		log.Printf("    Mean:                %v", searchStats["mean"])
		log.Printf("    P50:                 %v", searchStats["p50"])
		log.Printf("    P90:                 %v", searchStats["p90"])
		log.Printf("    P95:                 %v", searchStats["p95"])
		log.Printf("    P99:                 %v", searchStats["p99"])
		log.Printf("    P99.9:               %v", searchStats["p999"])
		log.Printf("    Max:                 %v", searchStats["max"])
	}
	log.Println()

	// Get final stats from service
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(config.BaseURL + "/stats")
	if err == nil {
		defer resp.Body.Close()
		var stats map[string]interface{}
		if json.NewDecoder(resp.Body).Decode(&stats) == nil {
			log.Printf("Service Statistics:")
			log.Printf("  Total Users:           %v", stats["total_users"])
			log.Printf("  Unique Usernames:      %v", stats["unique_usernames"])
			log.Printf("  Active Rating Buckets: %v", stats["active_rating_buckets"])
			log.Println()
		}
	}

	// Performance assessment
	log.Println("╔══════════════════════════════════════════════════════════════╗")
	log.Println("║                  PERFORMANCE ASSESSMENT                      ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")
	log.Println()

	opsPerSec := float64(totalOps) / results.Duration.Seconds()
	readLatency := results.ReadLatency.Calculate()
	p99, _ := readLatency["p99"].(time.Duration)

	if opsPerSec > 10000 {
		log.Println("✓ EXCELLENT: Throughput > 10K ops/sec")
	} else if opsPerSec > 5000 {
		log.Println("✓ GOOD: Throughput > 5K ops/sec")
	} else if opsPerSec > 1000 {
		log.Println("⚠ FAIR: Throughput > 1K ops/sec")
	} else {
		log.Println("✗ POOR: Throughput < 1K ops/sec")
	}

	if p99 < 10*time.Millisecond {
		log.Println("✓ EXCELLENT: P99 latency < 10ms")
	} else if p99 < 50*time.Millisecond {
		log.Println("✓ GOOD: P99 latency < 50ms")
	} else if p99 < 100*time.Millisecond {
		log.Println("⚠ FAIR: P99 latency < 100ms")
	} else {
		log.Println("✗ POOR: P99 latency > 100ms")
	}

	if errorRate < 0.1 {
		log.Println("✓ EXCELLENT: Error rate < 0.1%")
	} else if errorRate < 1.0 {
		log.Println("✓ GOOD: Error rate < 1%")
	} else if errorRate < 5.0 {
		log.Println("⚠ FAIR: Error rate < 5%")
	} else {
		log.Println("✗ POOR: Error rate > 5%")
	}

	log.Println()
	log.Println("Load test complete!")
}
