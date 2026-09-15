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

// Main opens the default host store and serves Hackbox on the first
// non-loopback IPv4 address.
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

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return fmt.Errorf("host: list interface addresses: %w", err)
	}
	ip, err := firstNonLoopbackIPv4(addrs)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(ip.String(), listenPort)
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return fmt.Errorf("host: listen on %s: %w", address, err)
	}
	defer func() { _ = listener.Close() }()

	log.Printf("Hackbox listening on http://%s", address)
	if err := http.Serve(listener, NewHandler(db)); err != nil {
		return fmt.Errorf("host: serve: %w", err)
	}
	return nil
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
		if ip != nil && !ip.IsLoopback() {
			return ip, nil
		}
	}
	return nil, errNoLANIPv4
}

var errNoLANIPv4 = errors.New("host: no non-loopback IPv4 address found")
