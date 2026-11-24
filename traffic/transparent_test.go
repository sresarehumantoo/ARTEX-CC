package traffic

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestTransparentTCPConfigWithDefaults(t *testing.T) {
	t.Parallel()

	t.Run("zero_config", func(t *testing.T) {
		cfg := TransparentTCPConfig{}.withDefaults()
		if cfg.Addr != DefaultTransparentTCPAddr {
			t.Fatalf("addr = %q", cfg.Addr)
		}
		if cfg.MaxConcurrent != DefaultSOCKS5MaxConcurrent {
			t.Fatalf("max concurrent = %d", cfg.MaxConcurrent)
		}
		if cfg.MaxBytesPerDir != DefaultSOCKS5MaxBytesPerDir {
			t.Fatalf("max bytes = %d", cfg.MaxBytesPerDir)
		}
		if cfg.PreviewBytes != DefaultSOCKS5PreviewBytes {
			t.Fatalf("preview = %d", cfg.PreviewBytes)
		}
		if cfg.IdleTimeout != DefaultSOCKS5IdleTimeout {
			t.Fatalf("idle = %v", cfg.IdleTimeout)
		}
		if cfg.HandshakeTimeout != DefaultSOCKS5HandshakeTimeout {
			t.Fatalf("handshake = %v", cfg.HandshakeTimeout)
		}
		if cfg.DialTimeout != DefaultSOCKS5DialTimeout {
			t.Fatalf("dial = %v", cfg.DialTimeout)
		}
		if cfg.PaceTimeout != DefaultSOCKS5PaceTimeout {
			t.Fatalf("pace = %v", cfg.PaceTimeout)
		}
		if cfg.DialContext == nil {
			t.Fatal("dial context nil")
		}
		if cfg.origDst == nil {
			t.Fatal("origDst nil")
		}
	})

	t.Run("partial_config", func(t *testing.T) {
		cfg := TransparentTCPConfig{
			Addr:          "10.0.0.1:1081",
			MaxConcurrent: 32,
			IdleTimeout:   3 * time.Minute,
		}.withDefaults()
		if cfg.Addr != "10.0.0.1:1081" {
			t.Fatalf("addr overwritten: %q", cfg.Addr)
		}
		if cfg.MaxConcurrent != 32 {
			t.Fatalf("max concurrent overwritten: %d", cfg.MaxConcurrent)
		}
		if cfg.IdleTimeout != 3*time.Minute {
			t.Fatalf("idle overwritten: %v", cfg.IdleTimeout)
		}
		if cfg.DialTimeout != DefaultSOCKS5DialTimeout {
			t.Fatalf("dial not defaulted: %v", cfg.DialTimeout)
		}
	})
}

func TestParseOriginalDstIPv4(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		raw := make([]byte, sockaddrInSize)
		binary.NativeEndian.PutUint16(raw[0:2], afInet)
		binary.BigEndian.PutUint16(raw[2:4], 8080)
		raw[4], raw[5], raw[6], raw[7] = 10, 0, 0, 1
		ip, port, err := parseOriginalDstIPv4(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !ip.Equal(net.IPv4(10, 0, 0, 1)) {
			t.Fatalf("ip = %v", ip)
		}
		if port != 8080 {
			t.Fatalf("port = %d", port)
		}
	})

	t.Run("short_buffer", func(t *testing.T) {
		_, _, err := parseOriginalDstIPv4(make([]byte, 8))
		if err == nil {
			t.Fatal("expected error for short buffer")
		}
	})

	t.Run("wrong_family", func(t *testing.T) {
		raw := make([]byte, sockaddrInSize)
		binary.NativeEndian.PutUint16(raw[0:2], 99)
		_, _, err := parseOriginalDstIPv4(raw)
		if err == nil {
			t.Fatal("expected error for wrong family")
		}
	})

	t.Run("ipv6_family", func(t *testing.T) {
		raw := make([]byte, sockaddrInSize)
		binary.NativeEndian.PutUint16(raw[0:2], 10) // AF_INET6
		_, _, err := parseOriginalDstIPv4(raw)
		if err != ErrTransparentTCPIPv6Unsupported {
			t.Fatalf("err = %v, want IPv6 unsupported", err)
		}
	})

	t.Run("zero_port", func(t *testing.T) {
		raw := make([]byte, sockaddrInSize)
		binary.NativeEndian.PutUint16(raw[0:2], afInet)
		binary.BigEndian.PutUint16(raw[2:4], 0)
		raw[4], raw[5], raw[6], raw[7] = 10, 0, 0, 1
		_, _, err := parseOriginalDstIPv4(raw)
		if err == nil {
			t.Fatal("expected error for zero port")
		}
	})

	t.Run("unspecified_ip", func(t *testing.T) {
		raw := make([]byte, sockaddrInSize)
		binary.NativeEndian.PutUint16(raw[0:2], afInet)
		binary.BigEndian.PutUint16(raw[2:4], 80)
		raw[4], raw[5], raw[6], raw[7] = 0, 0, 0, 0
		_, _, err := parseOriginalDstIPv4(raw)
		if err == nil {
			t.Fatal("expected error for unspecified IP")
		}
	})
}

func TestIsSelfRedirect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		origIP    net.IP
		origPort  int
		localAddr net.Addr
		want      bool
	}{
		{"matching", net.IPv4(127, 0, 0, 1), 1081, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1081}, true},
		{"port_mismatch", net.IPv4(127, 0, 0, 1), 1081, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}, false},
		{"ip_mismatch", net.IPv4(10, 0, 0, 1), 1081, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1081}, false},
		{"nil_origip", nil, 1081, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1081}, false},
		{"zero_port", net.IPv4(127, 0, 0, 1), 0, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}, false},
		{"nil_local", net.IPv4(127, 0, 0, 1), 1081, nil, false},
		{"unspecified_local_ip", net.IPv4(127, 0, 0, 1), 1081, &net.TCPAddr{IP: net.IPv4zero, Port: 1081}, true},
		{"unspecified_local_nil_ip", net.IPv4(127, 0, 0, 1), 1081, &net.TCPAddr{IP: nil, Port: 1081}, true},
		{"udp_addr_type", net.IPv4(127, 0, 0, 1), 1081, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1081}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isSelfRedirect(tc.origIP, tc.origPort, tc.localAddr)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
