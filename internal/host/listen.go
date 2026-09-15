// Package host composes Hackbox storage and HTTP handling.
package host

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/KroniK907/hackbox/internal/store"
)

const listenPort = "8654"

// Main opens the default host store and listens on every local interface,
// including loopback. The board page shows the first usable LAN IPv4 as the
// join URL. That advertised URL does not change the listen address.
func Main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dataDir, err := store.DefaultDataDir()
	if err != nil {
		return err
	}
	db, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	joinURL := ""
	addrs, err := upInterfaceAddrs()
	if err != nil {
		return err
	}
	if ip, err := firstNonLoopbackIPv4(addrs); err != nil {
		log.Print(err)
	} else {
		joinURL = advertisedJoinURL(ip)
	}

	listener, err := openListener(listenPort)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	log.Printf("Hackbox listening on http://127.0.0.1:%s", listenPort)
	if joinURL != "" {
		log.Printf("LAN join URL %s", joinURL)
	}
	if err := http.Serve(listener, NewHandler(db, joinURL)); err != nil {
		return fmt.Errorf("host: serve: %w", err)
	}
	return nil
}

func advertisedJoinURL(ip net.IP) string {
	return "http://" + net.JoinHostPort(ip.String(), listenPort) + "/"
}

// openListener binds TCP on all unicast addresses for port, including
// 127.0.0.1 and ::1. Cloudflare and the operator browser use loopback. Phones
// on the LAN still use the advertised join URL.
func openListener(port string) (net.Listener, error) {
	addr := net.JoinHostPort("", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("host: listen on %s: %w", addr, err)
	}
	return listener, nil
}

func upInterfaceAddrs() ([]net.Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("host: list interfaces: %w", err)
	}
	var addrs []net.Addr
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ia, err := iface.Addrs()
		if err != nil {
			continue
		}
		addrs = append(addrs, ia...)
	}
	return addrs, nil
}

func firstNonLoopbackIPv4(addrs []net.Addr) (net.IP, error) {
	for _, addr := range addrs {
		var ip net.IP
		switch addr := addr.(type) {
		case *net.IPNet:
			ip = addr.IP
		case *net.IPAddr:
			ip = addr.IP
		default:
			continue
		}
		ip = ip.To4()
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			continue
		}
		return ip, nil
	}
	return nil, errNoLANIPv4
}

var errNoLANIPv4 = errors.New("host: no usable LAN IPv4 address found")
