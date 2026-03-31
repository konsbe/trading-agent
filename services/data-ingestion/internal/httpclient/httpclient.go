package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"sync"
	"time"
)

var transportOnce sync.Once
var sharedTransport *http.Transport

// transportForTLS returns an HTTP transport that uses the system root store plus
// optional PEM roots from EXTRA_ROOT_CA_FILE (path inside the container). Use this
// for APIs that fail with x509: unknown authority behind TLS-inspecting proxies.
func transportForTLS() *http.Transport {
	transportOnce.Do(func() {
		t := http.DefaultTransport.(*http.Transport).Clone()
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if p := os.Getenv("EXTRA_ROOT_CA_FILE"); p != "" {
			if b, err := os.ReadFile(p); err == nil {
				pool.AppendCertsFromPEM(b)
			}
		}
		t.TLSClientConfig = &tls.Config{
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		}
		sharedTransport = t
	})
	return sharedTransport
}

// New returns an HTTP client with the shared TLS-aware transport.
func New(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: transportForTLS(),
	}
}
