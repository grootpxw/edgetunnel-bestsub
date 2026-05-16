package proxyip

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type allJSONResponse struct {
	Data []struct {
		IP   string `json:"ip"`
		Meta struct {
			Country string `json:"country"`
		} `json:"meta"`
	} `json:"data"`
}

type checkAPIResponse struct {
	Candidate    string `json:"candidate"`
	Success      bool   `json:"success"`
	ResponseTime int    `json:"responseTime"`
}

type ProxyIPResult struct {
	IP           string
	ResponseTime int
}

// FetchAndCheck fetches proxy IPs, filters by country, checks latency, and returns the top limit IPs.
func FetchAndCheck(country string, limit int) ([]string, error) {
	if country == "" {
		return nil, nil
	}

	country = strings.ToUpper(country)
	log.Printf("Starting auto-fetch proxy IPs for country: %s", country)

	// 1. Fetch all IPs
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://zip.cm.edu.kg/all.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all.json: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read all.json: %w", err)
	}

	var allData allJSONResponse
	if err := json.Unmarshal(body, &allData); err != nil {
		return nil, fmt.Errorf("failed to parse all.json: %w", err)
	}

	// 2. Filter by country
	var candidates []string
	for _, item := range allData.Data {
		if strings.ToUpper(item.Meta.Country) == country {
			candidates = append(candidates, item.IP)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no proxy IPs found for country %s", country)
	}
	log.Printf("Found %d candidate proxy IPs for country %s", len(candidates), country)

	// 3. Check latency concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan ProxyIPResult, len(candidates))
	sem := make(chan struct{}, 20) // Limit concurrency to 20

	for _, ip := range candidates {
		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			checkURL := fmt.Sprintf("https://api.090227.xyz/check?proxyip=%s", targetIP)
			cResp, cErr := client.Get(checkURL)
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

	if len(validResults) == 0 {
		return nil, fmt.Errorf("no valid proxy IPs after latency check")
	}

	// 4. Sort by ResponseTime ascending
	sort.Slice(validResults, func(i, j int) bool {
		return validResults[i].ResponseTime < validResults[j].ResponseTime
	})

	// 5. Pick top limit
	var finalIPs []string
	for i := 0; i < len(validResults) && i < limit; i++ {
		finalIPs = append(finalIPs, validResults[i].IP)
	}

	log.Printf("Successfully fetched and checked %d proxy IPs", len(finalIPs))
	return finalIPs, nil
}
