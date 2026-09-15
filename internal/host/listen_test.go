package host

import (
	"net"
	"testing"
)

func TestFirstNonLoopbackIPv4(t *testing.T) {
	t.Parallel()
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
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
}

func TestFirstNonLoopbackIPv4RefusesMissingAddress(t *testing.T) {
	t.Parallel()
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
	}

	if _, err := firstNonLoopbackIPv4(addrs); err == nil {
		t.Fatal("expected an error without a non-loopback IPv4 address")
	}
}
