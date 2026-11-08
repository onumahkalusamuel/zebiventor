// Package discovery advertises a hub on the LAN via mDNS/DNS-SD and lets
// tills find it without anyone typing an IP address. If multicast is blocked
// (some routers and Windows firewall profiles), tills can still connect by
// entering the hub address manually.
package discovery

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const service = "_zebiventor._tcp"

// Hub is a hub found on the network.
type Hub struct {
	HubID     string `json:"hub_id"`
	StoreName string `json:"store_name"`
	URL       string `json:"url"`
	Host      string `json:"host"`
}

// Advertise announces the hub until ctx is cancelled.
func Advertise(ctx context.Context, hubID, storeName string, port int) error {
	host, _ := osHostname()
	instance := fmt.Sprintf("Zebiventor Hub (%s)", host)
	srv, err := zeroconf.Register(instance, service, "local.", port,
		[]string{"hub_id=" + hubID, "store=" + storeName, "v=1"}, nil)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown()
	}()
	return nil
}

// Browse looks for hubs for up to timeout.
func Browse(ctx context.Context, timeout time.Duration) ([]Hub, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry, 16)
	var mu sync.Mutex
	found := map[string]Hub{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range entries {
			h := Hub{Host: strings.TrimSuffix(e.HostName, ".")}
			for _, t := range e.Text {
				k, v, _ := strings.Cut(t, "=")
				switch k {
				case "hub_id":
					h.HubID = v
				case "store":
					h.StoreName = v
				}
			}
			ip := pickIP(e.AddrIPv4)
			if ip == nil || h.HubID == "" {
				continue
			}
			h.URL = fmt.Sprintf("http://%s:%d", ip, e.Port)
			mu.Lock()
			found[h.HubID+h.URL] = h
			mu.Unlock()
		}
	}()
	if err := resolver.Browse(ctx, service, "local.", entries); err != nil {
		return nil, err
	}
	<-ctx.Done()
	<-done

	mu.Lock()
	defer mu.Unlock()
	out := make([]Hub, 0, len(found))
	for _, h := range found {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StoreName+out[i].URL < out[j].StoreName+out[j].URL })
	return out, nil
}

func pickIP(ips []net.IP) net.IP {
	for _, ip := range ips {
		if !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return ip
		}
	}
	if len(ips) > 0 {
		return ips[0]
	}
	return nil
}

// LANAddresses lists this machine's IPv4 LAN addresses (for display).
func LANAddresses() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip4 := ipn.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
					out = append(out, ip4.String())
				}
			}
		}
	}
	return out
}
