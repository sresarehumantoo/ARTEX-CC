package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func cmdTools(args []string) {
	fs := flag.NewFlagSet("tools", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "print the catalog as JSON")
	fs.Parse(args)

	catalog := mcpserver.Catalog()
	if *asJSON {
		writeToolsJSON(os.Stdout, catalog)
	} else {
		writeToolsText(os.Stdout, catalog)
	}
}

func writeToolsJSON(w io.Writer, catalog []mcpserver.CatalogEntry) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(catalog)
}

func writeToolsText(w io.Writer, catalog []mcpserver.CatalogEntry) {
	width := 0
	for _, e := range catalog {
		width = max(width, len(e.Name))
	}
	for _, cat := range tools.Categories {
		var entries []mcpserver.CatalogEntry
		for _, e := range catalog {
			if e.Category == cat {
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s (%d)\n", cat, len(entries))
		for _, e := range entries {
			note := ""
			if e.RequiresProxy {
				note = "[proxy]"
			}
			fmt.Fprintf(w, "  %-*s  %-7s  %s\n", width, e.Name, note, summarize(e.Description, 72))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "%d tools ([proxy] = registered only with serve --proxy)\n", len(catalog))
}

func summarize(desc string, n int) string {
	if i := strings.Index(desc, ". "); i >= 0 {
		desc = desc[:i+1]
	}
	if r := []rune(desc); len(r) > n {
		desc = string(r[:n-3]) + "..."
	}
	return desc
}
