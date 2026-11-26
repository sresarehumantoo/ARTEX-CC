package store

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDomainKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"EXAMPLE.COM", "example.com"},
		{"  Example.COM  ", "example.com"},
		{"example.com.", "example.com"},
		{"", ""},
		{"  ", ""},
	}
	for _, tt := range tests {
		got := DomainKey(tt.input)
		if got != tt.want {
			t.Errorf("DomainKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRootDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		wantRoot string
		wantApex bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", "example.com", false},
		{"deep.sub.example.com", "example.com", false},
		{"", "", true},
		{"10.10.10.1", "10.10.10.1", true},
		{"com", "com", true},
		{"co.uk", "co.uk", true},
		{"example.com.", "example.com", true},
		{"SUB.Example.Com", "example.com", false},
	}
	for _, tt := range tests {
		root, isApex := RootDomain(tt.input)
		if root != tt.wantRoot || isApex != tt.wantApex {
			t.Errorf("RootDomain(%q) = (%q, %v), want (%q, %v)",
				tt.input, root, isApex, tt.wantRoot, tt.wantApex)
		}
	}
}

func TestCalcCSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  *string
	}{
		{"192.168.1.42", strp("192.168.1.0/24")},
		{"10.0.0.1", strp("10.0.0.0/24")},
		{"not-an-ip", nil},
		{"", nil},
		{"::1", nil},
		{"2001:db8::1", nil},
	}
	for _, tt := range tests {
		got := calcCSegment(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("calcCSegment(%q) = %q, want nil", tt.input, *got)
			}
		} else {
			if got == nil {
				t.Errorf("calcCSegment(%q) = nil, want %q", tt.input, *tt.want)
			} else if *got != *tt.want {
				t.Errorf("calcCSegment(%q) = %q, want %q", tt.input, *got, *tt.want)
			}
		}
	}
}

func strp(s string) *string { return &s }

func TestAssetLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		asset Asset
		want  string
	}{
		{"root_domain", Asset{Type: AssetRootDomain, Domain: "example.com"}, "example.com"},
		{"subdomain", Asset{Type: AssetSubdomain, Domain: "sub.example.com"}, "sub.example.com"},
		{"ip", Asset{Type: AssetIP, IP: "10.10.10.1"}, "10.10.10.1"},
		{"service with url", Asset{Type: AssetService, URL: "https://example.com"}, "https://example.com"},
		{"service with name", Asset{Type: AssetService, ServiceName: "ssh"}, "ssh"},
		{"endpoint", Asset{Type: AssetEndpoint, Method: "GET", URL: "/api/v1"}, "GET /api/v1"},
		{"app with bundle", Asset{Type: AssetApp, BundleID: "com.example.app"}, "com.example.app"},
		{"app with name", Asset{Type: AssetApp, AppName: "My App"}, "My App"},
		{"unknown type", Asset{ID: 42, Type: "unknown"}, "asset:42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assetLabel(tt.asset)
			if got != tt.want {
				t.Errorf("assetLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAssetToTools(t *testing.T) {
	t.Parallel()
	port := 443
	statusCode := 200
	a := Asset{
		ID:           1,
		Type:         AssetService,
		Domain:       "example.com",
		RootDomain:   "example.com",
		IP:           "10.10.10.1",
		URL:          "https://example.com",
		Port:         &port,
		StatusCode:   &statusCode,
		Technologies: []string{"nginx"},
		PageTitle:    "Example",
		ServiceName:  "web",
		BundleID:     "com.example",
		AppName:      "Example App",
		Method:       "GET",
		Extra:        json.RawMessage(`{"custom":"value"}`),
	}

	ta := assetToTools(a)
	if ta.ID != 1 {
		t.Errorf("ID = %d, want 1", ta.ID)
	}
	if ta.Type != AssetService {
		t.Errorf("Type = %q, want %q", ta.Type, AssetService)
	}
	if ta.Label != "https://example.com" {
		t.Errorf("Label = %q, want %q", ta.Label, "https://example.com")
	}
	if ta.Data["domain"] != "example.com" {
		t.Errorf("Data[domain] = %v", ta.Data["domain"])
	}
	if ta.Data["ip"] != "10.10.10.1" {
		t.Errorf("Data[ip] = %v", ta.Data["ip"])
	}
	if ta.Data["port"] != 443 {
		t.Errorf("Data[port] = %v", ta.Data["port"])
	}
	if ta.Data["custom"] != "value" {
		t.Errorf("Data[custom] = %v", ta.Data["custom"])
	}
}

func TestAssetToToolsMinimal(t *testing.T) {
	t.Parallel()
	a := Asset{ID: 5, Type: AssetIP, IP: "1.2.3.4"}
	ta := assetToTools(a)
	if ta.Data["ip"] != "1.2.3.4" {
		t.Errorf("Data[ip] = %v, want 1.2.3.4", ta.Data["ip"])
	}
	if _, ok := ta.Data["domain"]; ok {
		t.Error("empty domain should not be in Data")
	}
	if _, ok := ta.Data["url"]; ok {
		t.Error("empty url should not be in Data")
	}
}

func TestStringArrayParsePostgresArray(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty braces", "{}", nil},
		{"empty string", "", nil},
		{"single unquoted", "{hello}", []string{"hello"}},
		{"single quoted", `{"hello"}`, []string{"hello"}},
		{"multiple quoted", `{"foo","bar","baz"}`, []string{"foo", "bar", "baz"}},
		{"mixed", `{foo,"bar"}`, []string{"foo", "bar"}},
		{"escaped quote", `{"say \"hi\""}`, []string{`say "hi"`}},
		{"escaped backslash", `{"path\\to"}`, []string{`path\to`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a stringArray
			if err := a.parsePostgresArray(tt.input); err != nil {
				t.Fatalf("parsePostgresArray(%q): %v", tt.input, err)
			}
			if !sliceEqual([]string(a), tt.want) {
				t.Errorf("parsePostgresArray(%q) = %v, want %v", tt.input, a, tt.want)
			}
		})
	}
}

func TestStringArrayParseInvalid(t *testing.T) {
	t.Parallel()
	var a stringArray
	err := a.parsePostgresArray("not-an-array")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestStringArrayValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input stringArray
		want  string
	}{
		{"nil", nil, "{}"},
		{"empty", stringArray{}, "{}"},
		{"single", stringArray{"hello"}, `{"hello"}`},
		{"multiple", stringArray{"a", "b"}, `{"a","b"}`},
		{"with quote", stringArray{`say "hi"`}, `{"say \"hi\""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := tt.input.Value()
			if err != nil {
				t.Fatalf("Value(): %v", err)
			}
			got := fmt.Sprint(v)
			if got != tt.want {
				t.Errorf("Value() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringArrayScan(t *testing.T) {
	t.Parallel()
	var a stringArray
	if err := a.Scan(nil); err != nil {
		t.Fatalf("Scan(nil): %v", err)
	}
	if a != nil {
		t.Errorf("Scan(nil) = %v, want nil", a)
	}

	if err := a.Scan([]byte(`{"x","y"}`)); err != nil {
		t.Fatalf("Scan([]byte): %v", err)
	}
	if len(a) != 2 || a[0] != "x" || a[1] != "y" {
		t.Errorf("Scan([]byte) = %v, want [x, y]", a)
	}

	if err := a.Scan(`{"a"}`); err != nil {
		t.Fatalf("Scan(string): %v", err)
	}
	if len(a) != 1 || a[0] != "a" {
		t.Errorf("Scan(string) = %v, want [a]", a)
	}

	err := a.Scan(42)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestInt64ArrayParsePostgresArray(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []int64
	}{
		{"empty braces", "{}", nil},
		{"empty string", "", nil},
		{"single", "{42}", []int64{42}},
		{"multiple", "{1,2,3}", []int64{1, 2, 3}},
		{"with spaces", "{ 1 , 2 , 3 }", []int64{1, 2, 3}},
		{"with NULL", "{1,NULL,3}", []int64{1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a int64Array
			if err := a.parsePostgresArray(tt.input); err != nil {
				t.Fatalf("parsePostgresArray(%q): %v", tt.input, err)
			}
			if !int64SliceEqual([]int64(a), tt.want) {
				t.Errorf("parsePostgresArray(%q) = %v, want %v", tt.input, a, tt.want)
			}
		})
	}
}

func TestInt64ArrayParseInvalid(t *testing.T) {
	t.Parallel()
	var a int64Array
	err := a.parsePostgresArray("not-an-array")
	if err == nil {
		t.Error("expected error for invalid format")
	}

	err = a.parsePostgresArray("{abc}")
	if err == nil {
		t.Error("expected error for non-numeric value")
	}
}

func TestInt64ArrayScan(t *testing.T) {
	t.Parallel()
	var a int64Array
	if err := a.Scan(nil); err != nil {
		t.Fatalf("Scan(nil): %v", err)
	}
	if a != nil {
		t.Errorf("Scan(nil) = %v, want nil", a)
	}

	if err := a.Scan([]byte("{10,20}")); err != nil {
		t.Fatalf("Scan([]byte): %v", err)
	}
	if len(a) != 2 || a[0] != 10 || a[1] != 20 {
		t.Errorf("Scan([]byte) = %v, want [10, 20]", a)
	}

	if err := a.Scan("{5}"); err != nil {
		t.Fatalf("Scan(string): %v", err)
	}
	if len(a) != 1 || a[0] != 5 {
		t.Errorf("Scan(string) = %v, want [5]", a)
	}

	err := a.Scan(42)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func int64SliceEqual(a, b []int64) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNormalizeAssetURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"https://user:pass@Example.com/", "https://example.com/"},
		{"HTTPS://EXAMPLE.COM:443/Path?Q=A", "https://example.com/Path?Q=A"},
		{"http://Example.com:80/a", "http://example.com/a"},
		{"http://example.com:443/a", "http://example.com:443/a"},
		{"https://example.com:8443/a", "https://example.com:8443/a"},
		{"http://admin@[2001:DB8::1]:80/x", "http://[2001:db8::1]/x"},
		{"https://[::1]:8443/", "https://[::1]:8443/"},
		{"https://example.com/p#Frag", "https://example.com/p#Frag"},
		{"  https://example.com/a  ", "https://example.com/a"},
		{"http://u:p@host/%zz", "http://host/%zz"},
		{"http://u:p@exa mple.com/a", "http://exa mple.com/a"},
		{"https://BÜCHER.de/x", "https://bücher.de/x"},
		{"https://u:p@BÜCHER.de:443/x", "https://bücher.de/x"},
		{"https://user:pa/ss@Host/", "https://host/"},
		{"https://user:p#ss@host/", "https://host/"},
		{"http://u:p@host:abc/", "http://host:abc/"},
		{"//u:p@Host/x", "//host/x"},
		{"http://Host:8080/path@x", "http://host:8080/path@x"},
		{"https://Host/users/@me", "https://host/users/@me"},
		{"https://host/a b", "https://host/a b"},
		{"example.com/path", "example.com/path"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeAssetURL(tt.in); got != tt.want {
			t.Errorf("NormalizeAssetURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
