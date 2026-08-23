package content

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxArticleResponseBytes = 5 << 20
	maxRobotsResponseBytes  = 1 << 20
)

type fetchResult struct {
	Body        []byte
	FinalURL    string
	StatusCode  int
	ContentType string
	Duration    time.Duration
}

type safeFetcher struct {
	resolver *net.Resolver
}

func newSafeFetcher() *safeFetcher {
	return &safeFetcher{resolver: net.DefaultResolver}
}

func (fetcher *safeFetcher) fetchHTML(ctx context.Context, rawURL string, allowedHosts []string, timeout time.Duration, userAgent string) (fetchResult, error) {
	return fetcher.fetch(ctx, rawURL, allowedHosts, timeout, userAgent, maxArticleResponseBytes, map[string]bool{
		"text/html":             true,
		"application/xhtml+xml": true,
	})
}

func (fetcher *safeFetcher) fetchRobots(ctx context.Context, rawURL string, allowedHosts []string, timeout time.Duration, userAgent string) (fetchResult, error) {
	return fetcher.fetch(ctx, rawURL, allowedHosts, timeout, userAgent, maxRobotsResponseBytes, map[string]bool{
		"text/plain":               true,
		"text/html":                true,
		"application/octet-stream": true,
	})
}

func (fetcher *safeFetcher) fetch(
	ctx context.Context,
	rawURL string,
	allowedHosts []string,
	timeout time.Duration,
	userAgent string,
	maxBytes int64,
	allowedContentTypes map[string]bool,
) (fetchResult, error) {
	parsed, err := validateOutboundURL(rawURL, allowedHosts)
	if err != nil {
		return fetchResult{}, err
	}
	if timeout < 3*time.Second || timeout > 60*time.Second {
		return fetchResult{}, fmt.Errorf("invalid fetch timeout")
	}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		DialContext: func(dialContext context.Context, network, address string) (net.Conn, error) {
			return fetcher.dialValidated(dialContext, network, address, allowedHosts, timeout)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			_, err := validateOutboundURL(request.URL.String(), allowedHosts)
			return err
		},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return fetchResult{}, err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8")
	request.Header.Set("User-Agent", userAgent)
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return fetchResult{}, err
	}
	defer response.Body.Close()
	result := fetchResult{
		FinalURL:    response.Request.URL.String(),
		StatusCode:  response.StatusCode,
		ContentType: response.Header.Get("Content-Type"),
		Duration:    time.Since(started),
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, fmt.Errorf("upstream returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return result, err
	}
	if int64(len(body)) > maxBytes {
		return result, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	contentType := response.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(body)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !allowedContentTypes[strings.ToLower(mediaType)] {
		return result, fmt.Errorf("unsupported response content type %q", contentType)
	}
	result.ContentType = contentType
	result.Body = body
	return result, nil
}

func (fetcher *safeFetcher) dialValidated(ctx context.Context, network, address string, allowedHosts []string, timeout time.Duration) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream address: %w", err)
	}
	host = normalizeHost(host)
	if !hostAllowed(host, allowedHosts) {
		return nil, fmt.Errorf("upstream host is not allowlisted")
	}
	if port != "443" {
		return nil, fmt.Errorf("only HTTPS port 443 is allowed")
	}
	addresses, err := fetcher.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve upstream: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("upstream host has no addresses")
	}
	dialer := net.Dialer{Timeout: timeout, KeepAlive: 15 * time.Second}
	var failures []error
	for _, address := range addresses {
		if !publicAddress(address.IP) {
			failures = append(failures, fmt.Errorf("resolved address is not public"))
			continue
		}
		connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		if err == nil {
			return connection, nil
		}
		failures = append(failures, err)
	}
	return nil, fmt.Errorf("connect upstream: %w", errors.Join(failures...))
}

func validateOutboundURL(rawURL string, allowedHosts []string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("crawler URL must be an absolute HTTPS URL without credentials")
	}
	if parsed.Fragment != "" {
		parsed.Fragment = ""
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return nil, fmt.Errorf("crawler URL must use HTTPS port 443")
	}
	if net.ParseIP(parsed.Hostname()) != nil {
		return nil, fmt.Errorf("literal IP crawler URLs are not allowed")
	}
	if !hostAllowed(normalizeHost(parsed.Hostname()), allowedHosts) {
		return nil, fmt.Errorf("crawler URL host is not allowlisted")
	}
	return parsed, nil
}

func publicAddress(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}

func hostAllowed(host string, allowedHosts []string) bool {
	for _, allowed := range allowedHosts {
		allowed = normalizeHost(allowed)
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func normalizeHost(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return host
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if parsed, err := http.ParseTime(value); err == nil && parsed.After(now) {
		return parsed.Sub(now)
	}
	return 0
}
