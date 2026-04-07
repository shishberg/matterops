package dashboard

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/shishberg/matterops/internal/service"
)

// StateProvider returns the current state of all services.
type StateProvider interface {
	GetAllStates() map[string]service.ServiceState
}

// Dashboard serves the web UI and JSON API for service status.
type Dashboard struct {
	provider StateProvider
	tmpl     *template.Template
	mux      *http.ServeMux
}

// New creates a Dashboard, parsing the index.html template from templatesDir.
func New(provider StateProvider, templatesDir string) (*Dashboard, error) {
	tmplPath := filepath.Join(templatesDir, "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return nil, err
	}

	d := &Dashboard{
		provider: provider,
		tmpl:     tmpl,
		mux:      http.NewServeMux(),
	}

	d.mux.HandleFunc("GET /", d.handleIndex)
	d.mux.HandleFunc("GET /api/status", d.handleAPIStatus)

	return d, nil
}

// ServeHTTP dispatches requests to the registered routes.
func (d *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mux.ServeHTTP(w, r)
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	states := d.provider.GetAllStates()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.tmpl.Execute(w, states); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (d *Dashboard) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	states := d.provider.GetAllStates()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(states); err != nil {
		http.Error(w, "json error: "+err.Error(), http.StatusInternalServerError)
	}
}
