package ingest

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type masscanRun struct {
	XMLName xml.Name      `xml:"nmaprun"`
	Hosts   []masscanHost `xml:"host"`
}

type masscanHost struct {
	Address masscanAddress `xml:"address"`
	Ports   []masscanPort  `xml:"ports>port"`
}

type masscanAddress struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type masscanPort struct {
	Protocol string       `xml:"protocol,attr"`
	PortID   int          `xml:"portid,attr"`
	State    masscanState `xml:"state"`
	Service  masscanSvc   `xml:"service"`
}

type masscanState struct {
	State string `xml:"state,attr"`
}

type masscanSvc struct {
	Name string `xml:"name,attr"`
}

// ParseMasscanXML parses masscan XML output into IP and service asset results.
func ParseMasscanXML(r io.Reader) ([]Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read masscan XML: %w", err)
	}

	var run masscanRun
	if err := xml.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("parse masscan XML: %w", err)
	}

	combined := &Result{}
	for _, host := range run.Hosts {
		ip := host.Address.Addr
		if ip == "" {
			continue
		}

		combined.Assets = append(combined.Assets, Asset{Type: "ip", IP: ip})

		for _, port := range host.Ports {
			if port.State.State != "open" {
				continue
			}
			combined.Assets = append(combined.Assets, masscanServiceAsset(ip, port.PortID, port.Protocol, port.Service.Name))
		}
	}

	if len(combined.Assets) == 0 {
		return nil, fmt.Errorf("no hosts found in masscan XML output")
	}
	return []Result{*combined}, nil
}

type masscanJSONRecord struct {
	IP    string            `json:"ip"`
	Ports []masscanJSONPort `json:"ports"`
}

type masscanJSONPort struct {
	Port    int            `json:"port"`
	Proto   string         `json:"proto"`
	Status  string         `json:"status"`
	Service masscanJSONSvc `json:"service"`
}

type masscanJSONSvc struct {
	Name string `json:"name"`
}

// ParseMasscanJSON parses masscan JSON output into IP and service asset results.
func ParseMasscanJSON(r io.Reader) ([]Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read masscan JSON: %w", err)
	}

	combined := &Result{}
	seenIPs := map[string]bool{}

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, ",")
		if line == "" || line == "[" || line == "]" {
			continue
		}

		var rec masscanJSONRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.IP == "" {
			continue
		}

		if !seenIPs[rec.IP] {
			seenIPs[rec.IP] = true
			combined.Assets = append(combined.Assets, Asset{Type: "ip", IP: rec.IP})
		}

		for _, p := range rec.Ports {
			if p.Status != "open" {
				continue
			}
			combined.Assets = append(combined.Assets, masscanServiceAsset(rec.IP, p.Port, p.Proto, p.Service.Name))
		}
	}

	if len(combined.Assets) == 0 {
		return nil, fmt.Errorf("no hosts found in masscan JSON output")
	}
	return []Result{*combined}, nil
}

func masscanServiceAsset(ip string, port int, proto, svcName string) Asset {
	if svcName == "http" || svcName == "https" || svcName == "http-proxy" {
		scheme := "http"
		if svcName == "https" || port == 443 || port == 8443 {
			scheme = "https"
		}
		h := ip
		if strings.Contains(h, ":") {
			h = "[" + h + "]"
		}
		return Asset{
			Type: "service",
			URL:  fmt.Sprintf("%s://%s:%d", scheme, h, port),
			IP:   ip,
			Port: port,
		}
	}
	if svcName == "" {
		svcName = fmt.Sprintf("unknown-%d", port)
	}
	return Asset{
		Type:        "service",
		IP:          ip,
		Port:        port,
		Proto:       proto,
		ServiceName: svcName,
	}
}
