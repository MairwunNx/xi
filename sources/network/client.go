package network

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"ximanager/sources/tracing"

	"golang.org/x/net/proxy"
)

func NewProxyClient(proxy proxy.Dialer, config *ProxyConfig, log *tracing.Logger) *http.Client {
	dc := func(ctx context.Context, network, address string) (net.Conn, error) {
		return proxy.Dial(network, address)
	}

	return &http.Client{
		Timeout: time.Duration(config.TimeoutSeconds),
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dc,
			MaxIdleConns:          20,
			IdleConnTimeout:       10 * time.Minute,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
			MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
			OnProxyConnectResponse: func(ctx context.Context, proxyURL *url.URL, connectReq *http.Request, connectRes *http.Response) error {
				log.I("Connected to proxy", tracing.ProxyUrl, proxyURL.String(), tracing.ProxyRes, connectRes.Status)
				return nil
			},
		},
	}
}
