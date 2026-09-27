// Package host composes GrabBag.gg storage and HTTP handling.
package host

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/KroniK907/grabbag/internal/store"
)

const listenPort = "8654"

// Main opens the default host store and listens on every local interface,
// including loopback. The board page advertises the request hostname when
// that host is public, and the first usable LAN IPv4 otherwise. That
// advertised URL does not change the listen address.
func Main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}

// options are the host command-line flags.
type options struct {
	port       string
	devPreview bool
	devBench   bool
}

// parseFlags reads -port, -dev-preview, and -dev-bench. The default port is listenPort.
// The value must be a TCP port from 1 through 65535. -dev-preview mounts the
// UI scenario gallery at PreviewPath. -dev-bench runs a throwaway room with
// every screen on one page at BenchPath.
func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("grabbag", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	port := fs.String("port", listenPort, "TCP port to listen on")
	devPreview := fs.Bool("dev-preview", false, "serve the UI scenario gallery at /dev/ui/")
	devBench := fs.Bool("dev-bench", false, "run a throwaway room with board, settings, and phones on one page at /dev/bench/")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	n, err := strconv.Atoi(*port)
	if err != nil || n < 1 || n > 65535 {
		return options{}, fmt.Errorf("host: -port must be 1 through 65535, got %q", *port)
	}
	return options{port: strconv.Itoa(n), devPreview: *devPreview, devBench: *devBench}, nil
}

func run(opts options) error {
	if opts.devBench {
		return runBench(opts)
	}
	port := opts.port
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
		joinURL = advertisedJoinURL(ip, port)
	}

	listener, err := listenOn("", port)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()

	log.Printf("GrabBag.gg listening on http://127.0.0.1:%s", port)
	log.Printf("Setup guide http://127.0.0.1:%s/docs", port)
	if joinURL != "" {
		log.Printf("LAN join URL %s", joinURL)
	}
	handler, err := NewHandler(db, joinURL)
	if err != nil {
		return fmt.Errorf("host: build handler: %w", err)
	}
	if opts.devPreview {
		handler = withPreview(handler)
		log.Printf("UI preview http://127.0.0.1:%s%s", port, PreviewPath)
	}
	if err := http.Serve(listener, handler); err != nil {
		return fmt.Errorf("host: serve: %w", err)
	}
	return nil
}

func advertisedJoinURL(ip net.IP, port string) string {
	return "http://" + net.JoinHostPort(ip.String(), port) + "/"
}

// listenOn binds TCP on host:port. Production uses host "" (all interfaces)
// so phones on the LAN can join. Tests bind 127.0.0.1 so Windows Firewall
// does not prompt for each throwaway test binary.
func listenOn(host, port string) (net.Listener, error) {
	addr := net.JoinHostPort(host, port)
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
