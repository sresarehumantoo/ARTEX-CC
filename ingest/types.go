package ingest

// Result is the parsed output of a scanner tool containing discovered assets and findings.
type Result struct {
	Assets   []Asset
	Findings []Finding
}

// Asset is a single network asset extracted from scanner output.
type Asset struct {
	Type          string
	Domain        string
	IP            string
	Port          int
	Proto         string
	ServiceName   string
	URL           string
	Method        string
	Technologies  []string
	StatusCode    int
	PageTitle     string
	ContentLength int64
}

// Finding is a vulnerability finding extracted from scanner output.
type Finding struct {
	VulnClass string
	Name      string
	Severity  string
	Summary   string
	Evidence  string
	Host      string
	URL       string
}
