package traffic

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
)

func TestParseSOCKS5UDPHeader(t *testing.T) {
	t.Parallel()

	t.Run("valid_ipv4", func(t *testing.T) {
		pkt := make([]byte, 10+5)
		pkt[0], pkt[1], pkt[2] = 0x00, 0x00, 0x00
		pkt[3] = socks5AtypIPv4
		pkt[4], pkt[5], pkt[6], pkt[7] = 10, 0, 0, 1
		binary.BigEndian.PutUint16(pkt[8:10], 8080)
		copy(pkt[10:], []byte("hello"))
		atyp, host, port, payload, err := parseSOCKS5UDPHeader(pkt)
		if err != nil {
			t.Fatal(err)
		}
		if atyp != socks5AtypIPv4 || host != "10.0.0.1" || port != 8080 || string(payload) != "hello" {
			t.Fatalf("got atyp=%d host=%q port=%d payload=%q", atyp, host, port, payload)
		}
	})

	t.Run("valid_domain", func(t *testing.T) {
		domain := "example.com"
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypDomain, byte(len(domain))}
		pkt = append(pkt, []byte(domain)...)
		var portBuf [2]byte
		binary.BigEndian.PutUint16(portBuf[:], 443)
		pkt = append(pkt, portBuf[:]...)
		pkt = append(pkt, []byte("data")...)
		atyp, host, port, payload, err := parseSOCKS5UDPHeader(pkt)
		if err != nil {
			t.Fatal(err)
		}
		if atyp != socks5AtypDomain || host != "example.com" || port != 443 || string(payload) != "data" {
			t.Fatalf("got atyp=%d host=%q port=%d payload=%q", atyp, host, port, payload)
		}
	})

	t.Run("short_packet", func(t *testing.T) {
		_, _, _, _, err := parseSOCKS5UDPHeader([]byte{0, 0})
		if err != errSOCKS5UDPShortPacket {
			t.Fatalf("err = %v, want short packet", err)
		}
	})

	t.Run("bad_rsv", func(t *testing.T) {
		pkt := []byte{0x01, 0x00, 0x00, socks5AtypIPv4, 10, 0, 0, 1, 0, 80}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPBadReserved {
			t.Fatalf("err = %v, want bad reserved", err)
		}
	})

	t.Run("fragmented", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x01, socks5AtypIPv4, 10, 0, 0, 1, 0, 80}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPFragmented {
			t.Fatalf("err = %v, want fragmented", err)
		}
	})

	t.Run("unsupported_atyp_ipv6", func(t *testing.T) {
		pkt := make([]byte, 4+16+2)
		pkt[3] = socks5AtypIPv6
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPUnsupportedAtyp {
			t.Fatalf("err = %v, want unsupported atyp", err)
		}
	})

	t.Run("unspecified_ip", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypIPv4, 0, 0, 0, 0, 0, 80}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v, want bad destination", err)
		}
	})

	t.Run("empty_domain", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypDomain, 0}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPShortPacket {
			t.Fatalf("err = %v, want short packet", err)
		}
	})

	t.Run("short_ipv4_packet", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypIPv4, 10, 0}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPShortPacket {
			t.Fatalf("err = %v, want short packet", err)
		}
	})

	t.Run("short_domain_body", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypDomain, 10, 'a', 'b'}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPShortPacket {
			t.Fatalf("err = %v, want short packet", err)
		}
	})

	t.Run("whitespace_only_domain", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypDomain, 3, ' ', ' ', ' ', 0, 80}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v, want bad destination", err)
		}
	})

	t.Run("zero_port", func(t *testing.T) {
		pkt := []byte{0x00, 0x00, 0x00, socks5AtypIPv4, 10, 0, 0, 1, 0x00, 0x00}
		_, _, _, _, err := parseSOCKS5UDPHeader(pkt)
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v, want bad destination", err)
		}
	})
}

func TestBuildSOCKS5UDPPacket(t *testing.T) {
	t.Parallel()

	t.Run("valid_ipv4", func(t *testing.T) {
		payload := []byte("hello")
		pkt, err := buildSOCKS5UDPPacket(net.IPv4(10, 0, 0, 1), 8080, payload)
		if err != nil {
			t.Fatal(err)
		}
		if len(pkt) != 10+len(payload) {
			t.Fatalf("len = %d", len(pkt))
		}
		if pkt[0] != 0 || pkt[1] != 0 || pkt[2] != 0 || pkt[3] != socks5AtypIPv4 {
			t.Fatalf("header: %v", pkt[:4])
		}
		port := binary.BigEndian.Uint16(pkt[8:10])
		if port != 8080 {
			t.Fatalf("port = %d", port)
		}
		if string(pkt[10:]) != "hello" {
			t.Fatalf("payload = %q", pkt[10:])
		}
	})

	t.Run("nil_ip", func(t *testing.T) {
		_, err := buildSOCKS5UDPPacket(nil, 80, []byte("x"))
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("ipv6", func(t *testing.T) {
		_, err := buildSOCKS5UDPPacket(net.IPv6loopback, 80, []byte("x"))
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("port_zero", func(t *testing.T) {
		_, err := buildSOCKS5UDPPacket(net.IPv4(10, 0, 0, 1), 0, []byte("x"))
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("port_too_large", func(t *testing.T) {
		_, err := buildSOCKS5UDPPacket(net.IPv4(10, 0, 0, 1), 70000, []byte("x"))
		if err != errSOCKS5UDPBadDestination {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("empty_payload", func(t *testing.T) {
		pkt, err := buildSOCKS5UDPPacket(net.IPv4(1, 2, 3, 4), 53, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(pkt) != 10 {
			t.Fatalf("len = %d, want 10", len(pkt))
		}
	})
}

func TestAcceptClientUDPSource(t *testing.T) {
	t.Parallel()

	srv := &socks5Server{}

	t.Run("nil_from", func(t *testing.T) {
		var mu sync.Mutex
		var pinned *net.UDPAddr
		ok := srv.acceptClientUDPSource(nil, nil, 0, &mu, &pinned)
		if ok {
			t.Fatal("expected false for nil from")
		}
	})

	t.Run("non_ipv4", func(t *testing.T) {
		var mu sync.Mutex
		var pinned *net.UDPAddr
		from := &net.UDPAddr{IP: net.IPv6loopback, Port: 1234}
		ok := srv.acceptClientUDPSource(from, nil, 0, &mu, &pinned)
		if ok {
			t.Fatal("expected false for IPv6")
		}
	})

	t.Run("wrong_expected_ip", func(t *testing.T) {
		var mu sync.Mutex
		var pinned *net.UDPAddr
		from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1234}
		expectedIP := net.IPv4(10, 0, 0, 2)
		ok := srv.acceptClientUDPSource(from, expectedIP, 0, &mu, &pinned)
		if ok {
			t.Fatal("expected false for wrong IP")
		}
	})

	t.Run("wrong_expected_port", func(t *testing.T) {
		var mu sync.Mutex
		var pinned *net.UDPAddr
		from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1234}
		ok := srv.acceptClientUDPSource(from, nil, 5555, &mu, &pinned)
		if ok {
			t.Fatal("expected false for wrong port")
		}
	})

	t.Run("first_pin", func(t *testing.T) {
		var mu sync.Mutex
		var pinned *net.UDPAddr
		from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1234}
		ok := srv.acceptClientUDPSource(from, nil, 0, &mu, &pinned)
		if !ok {
			t.Fatal("expected true for first pin")
		}
		if pinned == nil || !pinned.IP.Equal(net.IPv4(10, 0, 0, 1).To4()) || pinned.Port != 1234 {
			t.Fatalf("pinned = %v", pinned)
		}
	})

	t.Run("matching_pinned", func(t *testing.T) {
		var mu sync.Mutex
		pinned := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1).To4(), Port: 1234}
		from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1234}
		ok := srv.acceptClientUDPSource(from, nil, 0, &mu, &pinned)
		if !ok {
			t.Fatal("expected true for matching pinned")
		}
	})

	t.Run("mismatching_pinned_port", func(t *testing.T) {
		var mu sync.Mutex
		pinned := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1).To4(), Port: 1234}
		from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 5678}
		ok := srv.acceptClientUDPSource(from, nil, 0, &mu, &pinned)
		if ok {
			t.Fatal("expected false for mismatching port")
		}
	})

	t.Run("mismatching_pinned_ip", func(t *testing.T) {
		var mu sync.Mutex
		pinned := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1).To4(), Port: 1234}
		from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: 1234}
		ok := srv.acceptClientUDPSource(from, nil, 0, &mu, &pinned)
		if ok {
			t.Fatal("expected false for mismatching IP")
		}
	})

	t.Run("nil_from_ip", func(t *testing.T) {
		var mu sync.Mutex
		var pinned *net.UDPAddr
		from := &net.UDPAddr{IP: nil, Port: 1234}
		ok := srv.acceptClientUDPSource(from, nil, 0, &mu, &pinned)
		if ok {
			t.Fatal("expected false for nil IP in from")
		}
	})
}
