package semgrep

// Position represents a location in a source file.
type Position struct {
	Line   int `json:"line"`
	Col    int `json:"col"`
	Offset int `json:"offset"`
}

// Extra contains extra information about a Semgrep finding.
type Extra struct {
	Message     string                 `json:"message"`
	Severity    string                 `json:"severity"`
	Lines       string                 `json:"lines"`
	Fingerprint string                 `json:"fingerprint,omitempty"`
	EngineKind  string                 `json:"engine_kind,omitempty"`
	IsIgnored   bool                   `json:"is_ignored,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Result represents a single scan finding.
type Result struct {
	CheckID string   `json:"check_id"`
	Path    string   `json:"path"`
	Start   Position `json:"start"`
	End     Position `json:"end"`
	Extra   Extra    `json:"extra"`
}

// Error represents an error returned by Semgrep during the scan.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Path    string `json:"path,omitempty"`
}

// Paths represents the files scanned.
type Paths struct {
	Scanned []string `json:"scanned"`
}

// ScanReport represents the full schema returned by "semgrep scan --json".
type ScanReport struct {
	ScanID  string   `json:"scan_id,omitempty"`
	Version string   `json:"version"`
	Results []Result `json:"results"`
	Errors  []Error  `json:"errors"`
	Paths   Paths    `json:"paths"`
}
