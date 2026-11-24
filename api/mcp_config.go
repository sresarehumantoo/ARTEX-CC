package api

import (
	"errors"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	mcpServersFile = "mcp_servers.json"
	builtinMCPName = "artex"
)

var (
	mcpNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	mcpMu          sync.Mutex
)

type MCPServerConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

type mcpServerView struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
	EnvKeys []string `json:"env_keys"`
	BuiltIn bool     `json:"built_in"`
}

type mcpServerInput struct {
	Name string `json:"name"`
	MCPServerConfig
}

func (a *API) registerMCPConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mcp/servers", a.handleListMCPServers)
	mux.HandleFunc("POST /api/mcp/servers", a.handleAddMCPServer)
	mux.HandleFunc("DELETE /api/mcp/servers/{name}", a.handleDeleteMCPServer)
}

func (a *API) readMCPServers() (map[string]MCPServerConfig, error) {
	servers := map[string]MCPServerConfig{}
	err := a.files.ReadJSON(mcpServersFile, &servers)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return servers, nil
}

func (a *API) handleListMCPServers(w http.ResponseWriter, r *http.Request) {
	mcpMu.Lock()
	servers, err := a.readMCPServers()
	mcpMu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	names := make([]string, 0, len(servers))
	for n := range servers {
		names = append(names, n)
	}
	sort.Strings(names)
	out := []mcpServerView{{
		Name:    builtinMCPName + " (built-in)",
		Type:    "http",
		URL:     "/mcp",
		EnvKeys: []string{},
		BuiltIn: true,
	}}
	for _, n := range names {
		c := servers[n]
		keys := make([]string, 0, len(c.Env))
		for k := range c.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out = append(out, mcpServerView{
			Name: n, Type: c.Type, Command: c.Command, Args: c.Args, URL: c.URL, EnvKeys: keys,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleAddMCPServer(w http.ResponseWriter, r *http.Request) {
	var in mcpServerInput
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !mcpNamePattern.MatchString(in.Name) {
		writeError(w, http.StatusBadRequest, "name must start with a letter or digit and contain only letters, digits, '.', '_' or '-'")
		return
	}
	if in.Name == builtinMCPName {
		writeError(w, http.StatusConflict, "name is reserved for the built-in server")
		return
	}
	in.Command = strings.TrimSpace(in.Command)
	in.URL = strings.TrimSpace(in.URL)
	switch in.Type {
	case "stdio":
		if in.Command == "" {
			writeError(w, http.StatusBadRequest, "command is required for stdio servers")
			return
		}
		in.URL = ""
	case "http":
		if in.URL == "" {
			writeError(w, http.StatusBadRequest, "url is required for http servers")
			return
		}
		in.Command, in.Args = "", nil
	default:
		writeError(w, http.StatusBadRequest, "type must be stdio or http")
		return
	}

	mcpMu.Lock()
	defer mcpMu.Unlock()
	servers, err := a.readMCPServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, exists := servers[in.Name]; exists {
		writeError(w, http.StatusConflict, "a server with that name already exists")
		return
	}
	servers[in.Name] = in.MCPServerConfig
	if err := a.files.WriteJSON(mcpServersFile, servers); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": in.Name})
}

func (a *API) handleDeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == builtinMCPName || name == builtinMCPName+" (built-in)" {
		writeError(w, http.StatusBadRequest, "the built-in server cannot be deleted")
		return
	}
	mcpMu.Lock()
	defer mcpMu.Unlock()
	servers, err := a.readMCPServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := servers[name]; !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	delete(servers, name)
	if err := a.files.WriteJSON(mcpServersFile, servers); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
