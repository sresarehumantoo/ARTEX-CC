package traffic

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestSOCKS5ConfigWithDefaults(t *testing.T) {
	t.Parallel()
	t.Run("zero_config", func(t *testing.T) {
		cfg := SOCKS5Config{}.withDefaults()
		if cfg.Addr != DefaultSOCKS5Addr {
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
		if cfg.UDPMaxDatagrams != DefaultSOCKS5UDPMaxDatagrams {
			t.Fatalf("udp max datagrams = %d", cfg.UDPMaxDatagrams)
		}
		if cfg.UDPMaxBytes != DefaultSOCKS5UDPMaxBytes {
			t.Fatalf("udp max bytes = %d", cfg.UDPMaxBytes)
		}
		if cfg.UDPIdleTimeout != DefaultSOCKS5UDPIdleTimeout {
			t.Fatalf("udp idle = %v", cfg.UDPIdleTimeout)
		}
		if cfg.UDPPreviewBytes != cfg.PreviewBytes {
			t.Fatalf("udp preview = %d, want %d", cfg.UDPPreviewBytes, cfg.PreviewBytes)
		}
		if cfg.ResolveUDP4 == nil {
			t.Fatal("resolve nil")
		}
	})

	t.Run("partial_config", func(t *testing.T) {
		cfg := SOCKS5Config{
			Addr:          "10.0.0.1:9090",
			MaxConcurrent: 128,
			IdleTimeout:   5 * time.Minute,
		}.withDefaults()
		if cfg.Addr != "10.0.0.1:9090" {
			t.Fatalf("addr overwritten: %q", cfg.Addr)
		}
		if cfg.MaxConcurrent != 128 {
			t.Fatalf("max concurrent overwritten: %d", cfg.MaxConcurrent)
		}
		if cfg.IdleTimeout != 5*time.Minute {
			t.Fatalf("idle overwritten: %v", cfg.IdleTimeout)
		}
		if cfg.DialTimeout != DefaultSOCKS5DialTimeout {
			t.Fatalf("dial not defaulted: %v", cfg.DialTimeout)
		}
	})
}

func TestReadSOCKS5Request(t *testing.T) {
	t.Parallel()

	writeAndRead := func(t *testing.T, data []byte, enableUDP bool) (cmd, atyp byte, host string, port int, rep byte) {
		t.Helper()
		c, s := net.Pipe()
		go func() {
			_, _ = c.Write(data)
			c.Close()
		}()
		cmd, atyp, host, port, rep = readSOCKS5Request(s, enableUDP)
		s.Close()
		return
	}

	t.Run("ipv4_connect", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AtypIPv4, 10, 0, 0, 1, 0x00, 0x50}
		cmd, atyp, host, port, rep := writeAndRead(t, pkt, false)
		if cmd != socks5CmdConnect || atyp != socks5AtypIPv4 || host != "10.0.0.1" || port != 80 || rep != socks5RepSuccess {
			t.Fatalf("got cmd=%d atyp=%d host=%q port=%d rep=%d", cmd, atyp, host, port, rep)
		}
	})

	t.Run("domain_connect", func(t *testing.T) {
		domain := []byte("example.com")
		pkt := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AtypDomain, byte(len(domain))}
		pkt = append(pkt, domain...)
		pkt = append(pkt, 0x01, 0xBB) // port 443
		cmd, _, host, port, rep := writeAndRead(t, pkt, false)
		if cmd != socks5CmdConnect || host != "example.com" || port != 443 || rep != socks5RepSuccess {
			t.Fatalf("got cmd=%d host=%q port=%d rep=%d", cmd, host, port, rep)
		}
	})

	t.Run("ipv6_connect", func(t *testing.T) {
		pkt := make([]byte, 4+16+2)
		pkt[0] = socks5Version
		pkt[1] = socks5CmdConnect
		pkt[3] = socks5AtypIPv6
		copy(pkt[4:20], net.IPv6loopback)
		binary.BigEndian.PutUint16(pkt[20:22], 8080)
		cmd, _, host, port, rep := writeAndRead(t, pkt, false)
		if cmd != socks5CmdConnect || host != "::1" || port != 8080 || rep != socks5RepSuccess {
			t.Fatalf("got cmd=%d host=%q port=%d rep=%d", cmd, host, port, rep)
		}
	})

	t.Run("bad_version", func(t *testing.T) {
		pkt := []byte{0x04, socks5CmdConnect, 0x00, socks5AtypIPv4, 10, 0, 0, 1, 0x00, 0x50}
		_, _, _, _, rep := writeAndRead(t, pkt, false)
		if rep == socks5RepSuccess {
			t.Fatal("expected failure for bad version")
		}
	})

	t.Run("unsupported_atyp", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdConnect, 0x00, 0x09, 0, 0, 0, 1, 0x00, 0x50}
		_, _, _, _, rep := writeAndRead(t, pkt, false)
		if rep != socks5RepAtypNotSupported {
			t.Fatalf("rep = 0x%02x, want 0x%02x", rep, socks5RepAtypNotSupported)
		}
	})

	t.Run("empty_domain", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AtypDomain, 0}
		_, _, _, _, rep := writeAndRead(t, pkt, false)
		if rep == socks5RepSuccess {
			t.Fatal("expected failure for empty domain")
		}
	})

	t.Run("bind_unsupported", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdBind, 0x00, socks5AtypIPv4, 10, 0, 0, 1, 0x00, 0x50}
		_, _, _, _, rep := writeAndRead(t, pkt, false)
		if rep != socks5RepCmdNotSupported {
			t.Fatalf("rep = 0x%02x, want 0x%02x", rep, socks5RepCmdNotSupported)
		}
	})

	t.Run("udp_associate_disabled", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdUDPAssociate, 0x00, socks5AtypIPv4, 0, 0, 0, 0, 0x00, 0x00}
		_, _, _, _, rep := writeAndRead(t, pkt, false)
		if rep != socks5RepCmdNotSupported {
			t.Fatalf("rep = 0x%02x, want 0x%02x", rep, socks5RepCmdNotSupported)
		}
	})

	t.Run("udp_associate_enabled", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdUDPAssociate, 0x00, socks5AtypIPv4, 0, 0, 0, 0, 0x00, 0x00}
		cmd, _, _, _, rep := writeAndRead(t, pkt, true)
		if cmd != socks5CmdUDPAssociate || rep != socks5RepSuccess {
			t.Fatalf("cmd=%d rep=0x%02x", cmd, rep)
		}
	})

	t.Run("udp_associate_ipv6_rejected", func(t *testing.T) {
		pkt := make([]byte, 4+16+2)
		pkt[0] = socks5Version
		pkt[1] = socks5CmdUDPAssociate
		pkt[3] = socks5AtypIPv6
		copy(pkt[4:20], net.IPv6loopback)
		binary.BigEndian.PutUint16(pkt[20:22], 8080)
		_, _, _, _, rep := writeAndRead(t, pkt, true)
		if rep != socks5RepAtypNotSupported {
			t.Fatalf("rep = 0x%02x, want 0x%02x", rep, socks5RepAtypNotSupported)
		}
	})

	t.Run("short_read", func(t *testing.T) {
		pkt := []byte{socks5Version, socks5CmdConnect}
		_, _, _, _, rep := writeAndRead(t, pkt, false)
		if rep == socks5RepSuccess {
			t.Fatal("expected failure for short read")
		}
	})
}

func TestWriteSOCKS5Reply(t *testing.T) {
	t.Parallel()

	readReply := func(t *testing.T, rep byte, bindAddr net.Addr) []byte {
		t.Helper()
		c, s := net.Pipe()
		done := make(chan []byte, 1)
		go func() {
			data, _ := io.ReadAll(c)
			done <- data
		}()
		_ = writeSOCKS5Reply(s, rep, bindAddr)
		s.Close()
		return <-done
	}

	t.Run("tcp_addr_ipv4", func(t *testing.T) {
		data := readReply(t, socks5RepSuccess, &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 8080})
		if len(data) != 10 {
			t.Fatalf("len = %d, want 10", len(data))
		}
		if data[0] != socks5Version || data[1] != socks5RepSuccess || data[3] != socks5AtypIPv4 {
			t.Fatalf("header: %v", data[:4])
		}
		port := binary.BigEndian.Uint16(data[8:10])
		if port != 8080 {
			t.Fatalf("port = %d", port)
		}
	})

	t.Run("udp_addr", func(t *testing.T) {
		data := readReply(t, socks5RepSuccess, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5555})
		if len(data) != 10 {
			t.Fatalf("len = %d", len(data))
		}
		port := binary.BigEndian.Uint16(data[8:10])
		if port != 5555 {
			t.Fatalf("port = %d", port)
		}
	})

	t.Run("nil_addr", func(t *testing.T) {
		data := readReply(t, socks5RepGeneralFailure, nil)
		if len(data) != 10 {
			t.Fatalf("len = %d", len(data))
		}
		if data[1] != socks5RepGeneralFailure {
			t.Fatalf("rep = 0x%02x", data[1])
		}
	})

	t.Run("ipv6_addr", func(t *testing.T) {
		data := readReply(t, socks5RepSuccess, &net.TCPAddr{IP: net.IPv6loopback, Port: 443})
		if len(data) != 22 {
			t.Fatalf("len = %d, want 22", len(data))
		}
		if data[3] != socks5AtypIPv6 {
			t.Fatalf("atyp = 0x%02x, want 0x%02x", data[3], socks5AtypIPv6)
		}
	})
}

func TestReadSOCKS5UserPass(t *testing.T) {
	t.Parallel()

	callReadUserPass := func(t *testing.T, data []byte) (paced, ok bool) {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		done := make(chan struct{})
		go func() {
			defer close(done)
			c, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				return
			}
			defer c.Close()
			if len(data) > 0 {
				_, _ = c.Write(data)
			}
			c.(*net.TCPConn).CloseWrite()
			discard := make([]byte, 64)
			for {
				if _, err := c.Read(discard); err != nil {
					return
				}
			}
		}()
		s, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		paced, ok = readSOCKS5UserPass(s)
		s.Close()
		<-done
		return
	}

	t.Run("pace_marker", func(t *testing.T) {
		user := []byte(paceUser)
		pass := []byte("anything")
		data := []byte{socks5UserPassVer, byte(len(user))}
		data = append(data, user...)
		data = append(data, byte(len(pass)))
		data = append(data, pass...)
		paced, ok := callReadUserPass(t, data)
		if !paced || !ok {
			t.Fatalf("paced=%v ok=%v", paced, ok)
		}
	})

	t.Run("non_pace_user", func(t *testing.T) {
		user := []byte("someone")
		data := []byte{socks5UserPassVer, byte(len(user))}
		data = append(data, user...)
		data = append(data, 0) // zero-length password
		paced, ok := callReadUserPass(t, data)
		if paced {
			t.Fatal("should not be paced")
		}
		if !ok {
			t.Fatal("ok should be true")
		}
	})

	t.Run("bad_version", func(t *testing.T) {
		data := []byte{0x02, 4, 't', 'e', 's', 't', 0}
		_, ok := callReadUserPass(t, data)
		if ok {
			t.Fatal("ok should be false for bad version")
		}
	})

	t.Run("empty_read", func(t *testing.T) {
		_, ok := callReadUserPass(t, nil)
		if ok {
			t.Fatal("ok should be false for empty input")
		}
	})
}

func TestLimitedCapture(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		c := &limitedCapture{max: 100}
		c.record(nil)
		c.record([]byte{})
		if c.total != 0 || len(c.buf) != 0 || c.truncated {
			t.Fatalf("total=%d buf=%d truncated=%v", c.total, len(c.buf), c.truncated)
		}
	})

	t.Run("within_cap", func(t *testing.T) {
		c := &limitedCapture{max: 100}
		c.record([]byte("hello"))
		c.record([]byte("world"))
		if c.total != 10 || string(c.buf) != "helloworld" || c.truncated {
			t.Fatalf("total=%d buf=%q truncated=%v", c.total, c.buf, c.truncated)
		}
	})

	t.Run("over_cap_partial", func(t *testing.T) {
		c := &limitedCapture{max: 8}
		c.record([]byte("hello")) // 5 captured
		c.record([]byte("world")) // only 3 fit
		if c.total != 10 {
			t.Fatalf("total=%d", c.total)
		}
		if string(c.buf) != "hellowor" {
			t.Fatalf("buf=%q", c.buf)
		}
		if !c.truncated {
			t.Fatal("should be truncated")
		}
	})

	t.Run("over_cap_nothing_fits", func(t *testing.T) {
		c := &limitedCapture{max: 5}
		c.record([]byte("12345"))
		c.record([]byte("more"))
		if c.total != 9 {
			t.Fatalf("total=%d", c.total)
		}
		if string(c.buf) != "12345" {
			t.Fatalf("buf=%q", c.buf)
		}
		if !c.truncated {
			t.Fatal("should be truncated")
		}
	})

	t.Run("zero_max", func(t *testing.T) {
		c := &limitedCapture{max: 0}
		c.record([]byte("data"))
		if c.total != 4 {
			t.Fatalf("total=%d", c.total)
		}
		if len(c.buf) != 0 {
			t.Fatalf("buf should be empty, got %q", c.buf)
		}
		if !c.truncated {
			t.Fatal("should be truncated with max=0")
		}
	})
}

func TestCloseWriteNoPanic(t *testing.T) {
	t.Parallel()
	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()
	closeWrite(c)
}
