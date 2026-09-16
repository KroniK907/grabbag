package host

import (
	"net"
	"strconv"
	"testing"
)

func TestFirstNonLoopbackIPv4(t *testing.T) {
	t.Parallel()
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("169.254.204.155"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.10.24"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("10.0.0.9"), Mask: net.CIDRMask(24, 32)},
	}

	got, err := firstNonLoopbackIPv4(addrs)
	if err != nil {
		t.Fatal(err)
	}
	if want := "192.168.10.24"; got.String() != want {
		t.Fatalf("address = %s, want %s", got, want)
	}
	if join := advertisedJoinURL(got); join != "http://192.168.10.24:8654/" {
		t.Fatalf("join URL = %q", join)
	}
}

func TestFirstNonLoopbackIPv4RefusesMissingAddress(t *testing.T) {
	t.Parallel()
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("169.254.204.155"), Mask: net.CIDRMask(16, 32)},
	}

	if _, err := firstNonLoopbackIPv4(addrs); err == nil {
		t.Fatal("expected an error without a non-loopback IPv4 address")
	}
}

func TestListenAddrAllInterfaces(t *testing.T) {
	t.Parallel()
	if got := net.JoinHostPort("", listenPort); got != ":8654" {
		t.Fatalf("production listen addr = %q, want :8654", got)
	}
}

func TestListenerAcceptsLoopback(t *testing.T) {
	listener, err := listenOn("127.0.0.1", "0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("addr type %T", listener.Addr())
	}
	if !tcpAddr.IP.IsLoopback() {
		t.Fatalf("listener bound to %s, want loopback", tcpAddr.IP)
	}

	port := strconv.Itoa(tcpAddr.Port)
	for _, host := range []string{"127.0.0.1", "localhost"} {
		conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			t.Fatalf("dial %s: %v", host, err)
		}
		_ = conn.Close()
	}
}
