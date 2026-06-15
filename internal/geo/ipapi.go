package geo

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const ipAPIBase = "http://ip-api.com/json/"

// Result holds resolved geographic information for an IP address.
type Result struct {
	Location string
	Lat      float64
	Lon      float64
}

type geoAPIResponse struct {
	Status  string  `json:"status"`
	Country string  `json:"country"`
	City    string  `json:"city"`
	ISP     string  `json:"isp"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// IsLookupable reports whether an IP is suitable for public geo lookup.
func IsLookupable(ip string) bool {
	host := normalizeHost(ip)
	if host == "" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	parsed := net.ParseIP(host)
	if parsed == nil {
		return false
	}
	return !parsed.IsPrivate() && !parsed.IsLoopback() && !parsed.IsLinkLocalUnicast()
}

// LookupIP resolves geographic information via ip-api.com (same provider as legacy support-site backend).
func LookupIP(ip string) (*Result, error) {
	host := normalizeHost(ip)
	if !IsLookupable(host) {
		return nil, fmt.Errorf("geo: skip lookup for %q", ip)
	}

	req, err := http.NewRequest(http.MethodGet, ipAPIBase+host, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var payload geoAPIResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("geo: ip-api status %q for %s", payload.Status, host)
	}

	location := fmt.Sprintf("%s, %s (%s)", payload.City, payload.Country, payload.ISP)
	return &Result{
		Location: location,
		Lat:      payload.Lat,
		Lon:      payload.Lon,
	}, nil
}

func normalizeHost(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	if idx := strings.Index(ip, "%"); idx > 0 {
		return ip[:idx]
	}
	return ip
}
