package proxyip

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

type checkAPIResponse struct {
	Candidate    string `json:"candidate"`
	Success      bool   `json:"success"`
	ResponseTime int    `json:"responseTime"`
}

type ProxyIPResult struct {
	IP           string
	ResponseTime int
}

type Options struct {
	Country           string
	Limit             int
	SourceURL         string
	RequireGeoIPMatch bool
	GeoIPDBPath       string
	WorkerVerify      WorkerVerifyOptions
	WorkerBaseURL     string
	WorkerPassword    string
	UserAgent         string
}

type WorkerVerifyOptions struct {
	Enabled   bool
	URL       string
	MaxChecks int
}

type workerProxyIPTestResponse struct {
	Success      bool   `json:"success"`
	ProxyIP      string `json:"proxyip"`
	URL          string `json:"url"`
	IP           string `json:"ip"`
	Country      string `json:"country"`
	Colo         string `json:"colo"`
	Status       int    `json:"status"`
	ResponseTime int    `json:"responseTime"`
	Error        string `json:"error"`
}

// FetchAndCheck fetches proxy IPs, filters by country, checks latency, and returns the top limit IPs.
func FetchAndCheck(ctx context.Context, opts Options) ([]string, error) {
	if opts.Country == "" {
		return nil, nil
	}
	if opts.Limit <= 0 {
		opts.Limit = 8
	}
	if opts.SourceURL == "" {
		opts.SourceURL = "https://zip.cm.edu.kg/all.txt"
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "bestsub-go/0.1"
	}

	country := strings.ToUpper(opts.Country)
	log.Printf("Starting auto-fetch proxy IPs for country: %s", country)

	// 1. Fetch all IPs
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.SourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all.txt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch all.txt: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read all.txt: %w", err)
	}

	// 2. Filter by country
	candidates := parseAllTXT(string(body), country)

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no proxy IPs found for country %s", country)
	}
	log.Printf("Found %d candidate proxy IPs for country %s", len(candidates), country)

	// 3. Check latency concurrently
	validResults := checkLatency(ctx, client, candidates)

	if len(validResults) == 0 {
		return nil, fmt.Errorf("no valid proxy IPs after latency check")
	}

	// 4. Sort by ResponseTime ascending
	sort.Slice(validResults, func(i, j int) bool {
		return validResults[i].ResponseTime < validResults[j].ResponseTime
	})

	if opts.RequireGeoIPMatch {
		filtered, err := filterGeoIPCountry(validResults, opts.GeoIPDBPath, country)
		if err != nil {
			return nil, err
		}
		validResults = filtered
		if len(validResults) == 0 {
			return nil, fmt.Errorf("no valid proxy IPs after GeoIP country check")
		}
	}

	if opts.WorkerVerify.Enabled {
		filtered, err := filterWorkerExitCountry(ctx, opts, validResults, country)
		if err != nil {
			return nil, err
		}
		validResults = filtered
		if len(validResults) == 0 {
			return nil, fmt.Errorf("no valid proxy IPs after Worker exit country check")
		}
	}
	sort.SliceStable(validResults, func(i, j int) bool {
		return validResults[i].ResponseTime < validResults[j].ResponseTime
	})

	// 5. Pick top limit
	var finalIPs []string
	for i := 0; i < len(validResults) && i < opts.Limit; i++ {
		finalIPs = append(finalIPs, validResults[i].IP)
	}

	log.Printf("Successfully fetched and checked %d proxy IPs", len(finalIPs))
	return finalIPs, nil
}

func checkLatency(ctx context.Context, client *http.Client, candidates []string) []ProxyIPResult {
	var wg sync.WaitGroup
	resultsChan := make(chan ProxyIPResult, len(candidates))
	sem := make(chan struct{}, 20) // Limit concurrency to 20

	for _, ip := range candidates {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			checkURL := fmt.Sprintf("https://api.090227.xyz/check?proxyip=%s", url.QueryEscape(targetIP))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
			if err != nil {
				return
			}
			cResp, cErr := client.Do(req)
			if cErr != nil {
				return
			}
			defer cResp.Body.Close()

			if cResp.StatusCode != http.StatusOK {
				return
			}

			cBody, cErr := io.ReadAll(cResp.Body)
			if cErr != nil {
				return
			}

			var checkData checkAPIResponse
			if err := json.Unmarshal(cBody, &checkData); err == nil && checkData.Success {
				resultsChan <- ProxyIPResult{
					IP:           targetIP,
					ResponseTime: checkData.ResponseTime,
				}
			}
		}(ip)
	}

	wg.Wait()
	close(resultsChan)

	var validResults []ProxyIPResult
	for res := range resultsChan {
		validResults = append(validResults, res)
	}
	return validResults
}

func filterGeoIPCountry(results []ProxyIPResult, dbPath string, country string) ([]ProxyIPResult, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("geoip_db_path is required when require_geoip_match is true")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("GeoIP DB not available at %s: %w", dbPath, err)
	}
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	out := make([]ProxyIPResult, 0, len(results))
	for _, result := range results {
		addr, err := proxyIPAddr(result.IP)
		if err != nil {
			continue
		}
		record, err := db.Country(net.ParseIP(addr.String()))
		if err == nil && strings.EqualFold(record.Country.IsoCode, country) {
			out = append(out, result)
		}
	}
	log.Printf("GeoIP country check kept %d/%d proxy IPs", len(out), len(results))
	return out, nil
}

func filterWorkerExitCountry(ctx context.Context, opts Options, results []ProxyIPResult, country string) ([]ProxyIPResult, error) {
	if opts.WorkerBaseURL == "" {
		return nil, fmt.Errorf("worker.base_url is required when worker_verify.enabled is true")
	}
	if opts.WorkerPassword == "" {
		return nil, fmt.Errorf("worker.password is required when worker_verify.enabled is true")
	}
	limit := opts.WorkerVerify.MaxChecks
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	client, err := newWorkerHTTPClient(opts)
	if err != nil {
		return nil, err
	}
	if err := workerLogin(ctx, client, opts); err != nil {
		return nil, err
	}

	out := make([]ProxyIPResult, 0, opts.Limit)
	for i := 0; i < limit; i++ {
		result := results[i]
		test, err := workerProxyIPTest(ctx, client, opts, result.IP)
		if err != nil {
			log.Printf("Worker proxyip test failed for %s: %v", result.IP, err)
			continue
		}
		if test.Success && strings.EqualFold(test.Country, country) {
			if test.ResponseTime > 0 {
				result.ResponseTime = test.ResponseTime
			}
			out = append(out, result)
		} else {
			log.Printf("Worker proxyip test rejected %s: success=%v country=%s error=%s", result.IP, test.Success, test.Country, test.Error)
		}
		if len(out) >= opts.Limit {
			break
		}
	}
	log.Printf("Worker exit country check kept %d/%d proxy IPs", len(out), limit)
	return out, nil
}

func newWorkerHTTPClient(opts Options) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}, nil
}

func workerLogin(ctx context.Context, client *http.Client, opts Options) error {
	form := url.Values{}
	form.Set("password", opts.WorkerPassword)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(opts.WorkerBaseURL, "/")+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", opts.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("login returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if !strings.Contains(string(body), "success") {
		return fmt.Errorf("login did not return success: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func workerProxyIPTest(ctx context.Context, client *http.Client, opts Options, proxyIP string) (workerProxyIPTestResponse, error) {
	endpoint, err := url.Parse(strings.TrimRight(opts.WorkerBaseURL, "/") + "/admin/proxyip-test")
	if err != nil {
		return workerProxyIPTestResponse{}, err
	}
	query := endpoint.Query()
	query.Set("proxyip", proxyIP)
	if opts.WorkerVerify.URL != "" {
		query.Set("url", opts.WorkerVerify.URL)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return workerProxyIPTestResponse{}, err
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return workerProxyIPTestResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return workerProxyIPTestResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return workerProxyIPTestResponse{}, fmt.Errorf("worker test returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var data workerProxyIPTestResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return workerProxyIPTestResponse{}, err
	}
	return data, nil
}

func proxyIPAddr(value string) (netip.Addr, error) {
	addrPort, err := netip.ParseAddrPort(value)
	if err == nil {
		return addrPort.Addr(), nil
	}
	return netip.ParseAddr(value)
}

func parseAllTXT(text string, country string) []string {
	country = strings.ToUpper(strings.TrimSpace(country))
	seen := map[string]struct{}{}
	var candidates []string

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		address, code, ok := parseAllTXTLine(line)
		if !ok || !strings.EqualFold(code, country) {
			continue
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		candidates = append(candidates, address)
	}
	return candidates
}

func parseAllTXTLine(line string) (string, string, bool) {
	address, country, ok := strings.Cut(line, "#")
	if !ok {
		return "", "", false
	}
	address = strings.TrimSpace(address)
	country = strings.ToUpper(strings.TrimSpace(country))
	if address == "" || country == "" {
		return "", "", false
	}
	if _, err := netip.ParseAddrPort(address); err != nil {
		return "", "", false
	}
	return address, country, true
}
