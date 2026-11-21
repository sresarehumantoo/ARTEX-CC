// Package ingest parses scanner output (nmap, nuclei, httpx, ffuf, masscan) into asset results.
package ingest

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type nmapRun struct {
	XMLName xml.Name   `xml:"nmaprun"`
	Hosts   []nmapHost `xml:"host"`
}

type nmapHost struct {
	Addresses []nmapAddress  `xml:"address"`
	Hostnames []nmapHostname `xml:"hostnames>hostname"`
	Ports     []nmapPort     `xml:"ports>port"`
}

type nmapAddress struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type nmapHostname struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type nmapPort struct {
	Protocol string      `xml:"protocol,attr"`
	PortID   int         `xml:"portid,attr"`
	State    nmapState   `xml:"state"`
	Service  nmapService `xml:"service"`
}

type nmapState struct {
	State string `xml:"state,attr"`
}

type nmapService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

// ParseNmap parses nmap XML output into asset results.
func ParseNmap(data string) (*Result, error) {
	data = strings.TrimSpace(data)
	if strings.HasPrefix(data, "<") {
		return parseNmapXML(data)
	}
	return parseNmapGreppable(data)
}

func parseNmapXML(data string) (*Result, error) {
	var run nmapRun
	if err := xml.Unmarshal([]byte(data), &run); err != nil {
		return nil, fmt.Errorf("parse nmap XML: %w", err)
	}

	r := &Result{}
	for _, host := range run.Hosts {
		var ip string
		for _, addr := range host.Addresses {
			if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
				ip = addr.Addr
				break
			}
		}
		if ip == "" {
			continue
		}

		r.Assets = append(r.Assets, Asset{Type: "ip", IP: ip})

		for _, hn := range host.Hostnames {
			if hn.Name != "" {
				r.Assets = append(r.Assets, Asset{Type: "subdomain", Domain: hn.Name, IP: ip})
			}
		}

		for _, port := range host.Ports {
			if port.State.State != "open" {
				continue
			}
			svcName := port.Service.Name
			if svcName == "" {
				svcName = fmt.Sprintf("unknown-%d", port.PortID)
			}
			if svcName == "http" || svcName == "https" || svcName == "http-proxy" {
				scheme := "http"
				if svcName == "https" || port.PortID == 443 || port.PortID == 8443 {
					scheme = "https"
				}
				h := ip
				if strings.Contains(h, ":") {
					h = "[" + h + "]"
				}
				url := fmt.Sprintf("%s://%s:%d", scheme, h, port.PortID)
				r.Assets = append(r.Assets, Asset{
					Type: "service",
					URL:  url,
					IP:   ip,
					Port: port.PortID,
				})
			} else {
				r.Assets = append(r.Assets, Asset{
					Type:        "service",
					IP:          ip,
					Port:        port.PortID,
					Proto:       port.Protocol,
					ServiceName: svcName,
				})
			}
		}
	}

	return r, nil
}

func parseNmapGreppable(data string) (*Result, error) {
	r := &Result{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Host:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[1]

		r.Assets = append(r.Assets, Asset{Type: "ip", IP: ip})

		portIdx := strings.Index(line, "Ports:")
		if portIdx < 0 {
			continue
		}
		portSection := line[portIdx+len("Ports:"):]
		if tabIdx := strings.Index(portSection, "\t"); tabIdx >= 0 {
			portSection = portSection[:tabIdx]
		}
		portSection = strings.TrimSpace(portSection)

		for _, portEntry := range strings.Split(portSection, ",") {
			portEntry = strings.TrimSpace(portEntry)
			if portEntry == "" {
				continue
			}
			parts := strings.Split(portEntry, "/")
			if len(parts) < 3 {
				continue
			}
			portNum, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				continue
			}
			state := strings.TrimSpace(parts[1])
			if state != "open" {
				continue
			}
			proto := strings.TrimSpace(parts[2])
			svcName := ""
			if len(parts) > 4 {
				svcName = strings.TrimSpace(parts[4])
			}
			if svcName == "" {
				svcName = fmt.Sprintf("unknown-%d", portNum)
			}

			if svcName == "http" || svcName == "https" || svcName == "http-proxy" {
				scheme := "http"
				if svcName == "https" || portNum == 443 || portNum == 8443 {
					scheme = "https"
				}
				h := ip
				if strings.Contains(h, ":") {
					h = "[" + h + "]"
				}
				url := fmt.Sprintf("%s://%s:%d", scheme, h, portNum)
				r.Assets = append(r.Assets, Asset{
					Type: "service",
					URL:  url,
					IP:   ip,
					Port: portNum,
				})
			} else {
				r.Assets = append(r.Assets, Asset{
					Type:        "service",
					IP:          ip,
					Port:        portNum,
					Proto:       proto,
					ServiceName: svcName,
				})
			}
		}
	}

	if len(r.Assets) == 0 {
		return nil, fmt.Errorf("no hosts found in nmap greppable output")
	}
	return r, nil
}
