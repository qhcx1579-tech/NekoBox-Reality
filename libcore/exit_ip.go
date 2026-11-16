package libcore

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing/common/metadata"
)

// QueryExitIP is pinned to the selected proxy outbound, without a direct fallback.
func QueryExitIP(instance *BoxInstance) (result string, err error) {
	defer func() {
		if recover() != nil {
			result, err = "", errors.New("exit_ip_proxy_unavailable")
		}
	}()
	if instance == nil || instance.Box == nil {
		return "", errors.New("exit_ip_proxy_unavailable")
	}
	if _, found := instance.Box.Outbound().Outbound("proxy"); !found {
		return "", errors.New("exit_ip_proxy_unavailable")
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.NewDetour(instance.Box.Outbound(), "proxy", true).
				DialContext(ctx, network, metadata.ParseSocksaddr(address))
		},
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
	}
	defer transport.CloseIdleConnections()
	return fetchExitIP(&http.Client{Transport: transport})
}
