package ingest

type Result struct {
	Assets   []Asset
	Findings []Finding
}

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

type Finding struct {
	VulnClass string
	Name      string
	Severity  string
	Summary   string
	Evidence  string
	Host      string
	URL       string
}
