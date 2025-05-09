package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// Relay represents a Tor Relay with its details
type Relay struct {
	Fingerprint string   `json:"fingerprint"`
	OrAddresses []string `json:"or_addresses"`
	Country     string   `json:"country"`
	Reachable   []string // Populated after checking
}

// RelayResponse represents the JSON structure from onionoo
type RelayResponse struct {
	Relays []Relay `json:"relays"`
}

// Logger для записи в файл
var logger *log.Logger

// initLogger инициализирует логгер для записи в файл "_scanner.log"
func initLogger() {
	logFile, err := os.OpenFile("_scanner.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logger = log.New(logFile, "", log.LstdFlags)
}

// logPrint заменяет fmt.Fprintf(os.Stderr, ...)
func logPrint(format string, v ...interface{}) {
	logger.Printf(format, v...)
}

// logPrintln заменяет fmt.Fprintln(os.Stderr, ...)
func logPrintln(v ...interface{}) {
	logger.Println(v...)
}

// loadRelays downloads relay data from specified URLs with proxy and CORS support
func loadRelays(urls []string, timeout time.Duration, proxy string) ([]Relay, error) {
	client := &http.Client{Timeout: timeout}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			logPrint("Can't download Tor Relay data from %s: %v\n", u, err)
			continue
		}
		defer resp.Body.Close()

		var data RelayResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			logPrint("Failed to parse JSON from %s: %v\n", u, err)
			continue
		}
		logPrint("Successfully loaded relays from %s\n", u)
		return data.Relays, nil
	}
	return nil, fmt.Errorf("failed to download relay data from all URLs")
}

// checkRelay tests if a relay address is reachable and writes to file immediately
func checkRelay(address string, timeout time.Duration, results chan<- struct {
	Address string
	Relay   *Relay
}, wg *sync.WaitGroup, relay *Relay, file *os.File, mu *sync.Mutex) {
	defer wg.Done()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err == nil {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetLinger(0)
		}
		conn.Close()
		// Write to file immediately
		line := fmt.Sprintf("%s %s\n", address, relay.Fingerprint)
		mu.Lock()
		file.WriteString(line)
		mu.Unlock()
		// Send result to channel
		results <- struct {
			Address string
			Relay   *Relay
		}{address, relay}
	} else {
		logPrint("Failed to connect to %s: %v\n", address, err)
	}
}

// parseAddress splits address into host and port, handling IPv6
func parseAddress(addr string) (string, string) {
	parts := strings.Split(addr, ":")
	if len(parts) < 2 {
		return "", ""
	}
	host := parts[0]
	port := parts[len(parts)-1]
	if strings.HasPrefix(host, "[") && strings.Contains(addr, "]:") {
		host = host[1:]
		portIdx := strings.LastIndex(addr, ":")
		host = addr[1:portIdx]
		port = addr[portIdx+1:]
	}
	return host, port
}

// filterAndSortRelays applies country and port filters
func filterAndSortRelays(relays []Relay, preferredCountry string, ports []string) []Relay {
	var onlyCountries, excludeCountries, sortedCountries map[string]int
	if preferredCountry != "" {
		onlyCountries = make(map[string]int)
		excludeCountries = make(map[string]int)
		sortedCountries = make(map[string]int)
		countries := strings.Split(preferredCountry, ",")
		for i, c := range countries {
			c = strings.TrimSpace(c)
			if strings.HasPrefix(c, "!") {
				onlyCountries[c[1:]] = i
			} else if strings.HasPrefix(c, "-") {
				excludeCountries[c[1:]] = i
			} else {
				sortedCountries[c] = i
			}
		}
	}

	var filtered []Relay
	for _, r := range relays {
		if len(onlyCountries) > 0 {
			if _, exists := onlyCountries[r.Country]; !exists {
				continue
			}
		}
		if _, excluded := excludeCountries[r.Country]; excluded {
			continue
		}

		if len(ports) > 0 {
			var newAddrs []string
			for _, addr := range r.OrAddresses {
				_, portStr := parseAddress(addr)
				for _, p := range ports {
					if p == portStr {
						newAddrs = append(newAddrs, addr)
						break
					}
				}
			}
			if len(newAddrs) == 0 {
				continue
			}
			r.OrAddresses = newAddrs
		}
		filtered = append(filtered, r)
	}

	if len(sortedCountries) > 0 {
		sort.Slice(filtered, func(i, j int) bool {
			ci, okI := sortedCountries[filtered[i].Country]
			cj, okJ := sortedCountries[filtered[j].Country]
			if !okI {
				ci = 1000
			}
			if !okJ {
				cj = 1000
			}
			return ci < cj
		})
	}

	return filtered
}

// generateOutput writes the relay configuration to outfile
func generateOutput(workingRelays []Relay, torrcFmt bool, prefsjs string, outfile *os.File) error {
	prefix := ""
	if torrcFmt {
		prefix = "Bridge "
	}

	writer := bufio.NewWriter(outfile)

	for _, r := range workingRelays {
		for _, addr := range r.Reachable {
			line := fmt.Sprintf("%s%s %s\n", prefix, addr, r.Fingerprint)
			fmt.Fprint(writer, line)
			logPrint("Added to output: %s", line)
		}
	}
	if torrcFmt {
		fmt.Fprintln(writer, "UseBridges 1")
	}
	writer.Flush()

	if prefsjs != "" {
		if _, err := os.Stat(prefsjs); os.IsNotExist(err) {
			return fmt.Errorf("prefs.js file does not exist: %s", prefsjs)
		}
		content, err := os.ReadFile(prefsjs)
		if err != nil {
			return fmt.Errorf("can't read prefs.js: %v", err)
		}
		lines := strings.Split(string(content), "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.Contains(line, "torbrowser.settings.bridges.") {
				newLines = append(newLines, line)
			}
		}
		for i, r := range workingRelays {
			for _, addr := range r.Reachable {
				newLines = append(newLines, fmt.Sprintf(`user_pref("torbrowser.settings.bridges.bridge_strings.%d", "%s %s");`, i, addr, r.Fingerprint))
			}
		}
		newLines = append(newLines, `user_pref("torbrowser.settings.bridges.enabled", true);`)
		newLines = append(newLines, `user_pref("torbrowser.settings.bridges.source", 2);`)
		return os.WriteFile(prefsjs, []byte(strings.Join(newLines, "\n")), 0644)
	}
	return nil
}

// startBrowser attempts to launch Tor Browser
func startBrowser() error {
	browserCmds := []string{
		"Browser/start-tor-browser --detach",
		"Browser/firefox.exe",
	}
	for _, cmd := range browserCmds {
		parts := strings.Split(cmd, " ")
		if _, err := os.Stat(parts[0]); err == nil {
			err := exec.Command(parts[0], parts[1:]...).Start()
			if err != nil {
				logPrint("Failed to start browser with %s: %v\n", cmd, err)
				continue
			}
			logPrint("Successfully started browser with %s\n", cmd)
			return nil
		}
	}
	return fmt.Errorf("no valid browser executable found")
}

func main() {
	// Инициализация логгера
	initLogger()

	numRelays := flag.Int("n", 30, "Number of concurrent relays tested")
	goal := flag.Int("g", 5, "Test until at least this number of working relays are found")
	preferredCountry := flag.String("c", "", "Preferred/excluded/exclusive country list, comma-separated (e.g., US,!CA,-RU)")
	timeout := flag.Float64("timeout", 10.0, "Socket connection timeout in seconds")
	outfileName := flag.String("o", "", "Output file for reachable relays")
	torrcFmt := flag.Bool("torrc", false, "Output in torrc format")
	proxy := flag.String("proxy", "", "Proxy for onionoo download (e.g., http://proxy:port)")
	urls := flag.String("url", "", "Comma-separated list of alternative URLs")
	ports := flag.String("p", "", "Comma-separated list of ports to filter (e.g., 443,9001)")
	prefsjs := flag.String("browser", "", "Path to prefs.js for Tor Browser")
	startBrowserFlag := flag.Bool("start-browser", false, "Launch browser after scanning")
	flag.Parse()

	outfile := os.Stdout
	if *outfileName != "" {
		f, err := os.Create(*outfileName)
		if err != nil {
			logPrint("Failed to create output file: %v\n", err)
			log.Fatal(err)
		}
		defer f.Close()
		outfile = f
	}

	var portList []string
	if *ports != "" {
		portList = strings.Split(*ports, ",")
	}

	urlList := []string{
		"https://onionoo.torproject.org/details?type=relay&running=true&fields=fingerprint,or_addresses,country",
		"https://icors.vercel.app/?https://onionoo.torproject.org/details?type=relay&running=true&fields=fingerprint,or_addresses,country",
		"https://github.com/ValdikSS/tor-onionoo-mirror/raw/master/details-running-relays-fingerprint-address-only.json",
		"https://bitbucket.org/ValdikSS/tor-onionoo-mirror/raw/master/details-running-relays-fingerprint-address-only.json",
	}
	if *urls != "" {
		urlList = append(strings.Split(*urls, ","), urlList...)
	}

	logPrint("Tor Relay Scanner. Will scan up to %d working relays\n", *goal)
	logPrintln("Downloading Tor Relay information from Tor Metrics…")
	relays, err := loadRelays(urlList, time.Duration(*timeout)*time.Second, *proxy)
	if err != nil {
		logPrint("Error downloading relays: %v\n", err)
		log.Fatal(err)
	}
	logPrintln("Done!")

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(relays), func(i, j int) { relays[i], relays[j] = relays[j], relays[i] })
	relays = filterAndSortRelays(relays, *preferredCountry, portList)
	if len(relays) == 0 {
		logPrintln("No relays match the specified criteria")
		log.Fatal("No relays match the specified criteria")
	}

	// Open the file for appending in real-time
	bridgesFile, err := os.OpenFile("_bridges.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logPrint("Failed to open _bridges.txt: %v\n", err)
		log.Fatal(err)
	}
	defer bridgesFile.Close()

	var mu sync.Mutex // Mutex to synchronize file writes

	var workingRelays []Relay
	for i := 0; i < len(relays) && len(workingRelays) < *goal; i += *numRelays {
		end := i + *numRelays
		if end > len(relays) {
			end = len(relays)
		}
		chunk := relays[i:end]

		logPrint("\nAttempt %d/%d, Testing %d random relays:\n", (i / *numRelays) + 1, (len(relays) + *numRelays - 1) / *numRelays, len(chunk))
		for _, r := range chunk {
			logPrintln(r.Fingerprint)
		}

		var wg sync.WaitGroup
		results := make(chan struct {
			Address string
			Relay   *Relay
		}, len(chunk)*10)
		for _, relay := range chunk {
			for _, addr := range relay.OrAddresses {
				wg.Add(1)
				go checkRelay(addr, time.Duration(*timeout)*time.Second, results, &wg, &relay, bridgesFile, &mu)
			}
		}
		wg.Wait()
		close(results)

		for res := range results {
			res.Relay.Reachable = append(res.Relay.Reachable, res.Address)
		}

		for _, r := range chunk {
			if len(r.Reachable) > 0 {
				workingRelays = append(workingRelays, r)
			}
		}

		logPrintln("Reachable relays this attempt:")
		for _, r := range chunk {
			if len(r.Reachable) > 0 {
				for _, addr := range r.Reachable {
					logPrint("%s %s\n", addr, r.Fingerprint)
				}
			}
		}
	}

	if len(workingRelays) > 0 {
		if err := generateOutput(workingRelays, *torrcFmt, *prefsjs, outfile); err != nil {
			logPrint("Failed to generate output: %v\n", err)
			log.Fatal(err)
		}
	} else {
		logPrintln("No reachable relays found.")
	}

	if *startBrowserFlag {
		if err := startBrowser(); err != nil {
			logPrint("Browser start failed: %v\n", err)
		}
	}
}