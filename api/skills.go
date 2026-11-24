package api

import (
	"errors"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const skillsDir = "skills"

var (
	skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	skillsMu         sync.Mutex
)

type Skill struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type skillInput struct {
	Description string `json:"description"`
	Content     string `json:"content"`
}

func (a *API) registerSkillsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/skills", a.handleListSkills)
	mux.HandleFunc("PUT /api/skills/{name}", a.handlePutSkill)
	mux.HandleFunc("DELETE /api/skills/{name}", a.handleDeleteSkill)
}

func skillPath(name string) string {
	return skillsDir + "/" + name + ".json"
}

func (a *API) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skillsMu.Lock()
	defer skillsMu.Unlock()
	out := []Skill{}
	names, err := a.files.ListDir(skillsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, n := range names {
		name, ok := strings.CutSuffix(n, ".json")
		if !ok {
			continue
		}
		var s Skill
		if err := a.files.ReadJSON(skillPath(name), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handlePutSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !skillNamePattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "name must be lowercase letters, digits, and hyphens")
		return
	}
	var in skillInput
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	skillsMu.Lock()
	defer skillsMu.Unlock()
	s := Skill{Name: name, Description: in.Description, Content: in.Content, UpdatedAt: time.Now().UTC()}
	if err := a.files.WriteJSON(skillPath(name), s); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *API) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !skillNamePattern.MatchString(name) {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	skillsMu.Lock()
	defer skillsMu.Unlock()
	if err := a.files.ReadJSON(skillPath(name), &Skill{}); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "skill not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if err := a.files.Delete(skillPath(name)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
