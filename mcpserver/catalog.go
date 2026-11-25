package mcpserver

import (
	"encoding/json"

	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

// CatalogEntry describes one built-in MCP tool as clients see it.
type CatalogEntry struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Category      string          `json:"category"`
	InputSchema   json.RawMessage `json:"inputSchema"`
	ReadOnly      bool            `json:"readOnly"`
	RequiresProxy bool            `json:"requiresProxy"`
}

// Catalog returns every built-in tool regardless of serve flags, in
// registration order. Tools are constructed against nil dependencies and
// are never run, so no database, proxy, or evidence store is needed.
func Catalog() []CatalogEntry {
	reg := tools.NewRegistry()
	tools.RegisterAll(reg, nil, 0, 0)
	tools.RegisterEvidenceTools(reg, nil, nil, 0)
	s := &Server{}
	all := append(reg.Tools(), s.listSessionsTool(), s.switchEngagementTool())

	proxyReg := tools.NewRegistry()
	tools.RegisterTrafficTools(proxyReg, nil, nil, 0)

	var out []CatalogEntry
	add := func(t tools.Tool, requiresProxy bool) {
		out = append(out, CatalogEntry{
			Name:          t.Name,
			Description:   t.Description,
			Category:      t.Category,
			InputSchema:   convertTool(t).RawInputSchema,
			ReadOnly:      t.ReadOnly,
			RequiresProxy: requiresProxy,
		})
	}
	for _, t := range all {
		add(t, false)
	}
	for _, t := range proxyReg.Tools() {
		add(t, true)
	}
	return out
}
