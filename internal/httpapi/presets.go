// SPDX-License-Identifier: GPL-3.0-or-later

package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// validPresetName matches safe preset names.
var validPresetName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// presetEntry is a single entry in the preset list response.
type presetEntry struct {
	// Name is the preset name (filename without .json).
	Name string `json:"name"`

	// Description is the human-readable description.
	Description string `json:"description"`
}

// handleListDPIPresets handles GET /api/presets/dpi by listing
// available DPI preset files from the data/dpi/ directory.
func (h *Handler) handleListDPIPresets(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(h.dpiDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var presets []presetEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if !validPresetName.MatchString(name) {
			continue
		}

		// Read the file to extract the description.
		data, err := os.ReadFile(filepath.Join(h.dpiDir, entry.Name()))
		if err != nil {
			continue
		}
		var preset dpiPreset
		if err := json.Unmarshal(data, &preset); err != nil {
			continue
		}

		presets = append(presets, presetEntry{
			Name:        name,
			Description: preset.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presets)
}

// handleGetDPIPreset handles GET /api/presets/dpi/{name} by returning
// the full preset JSON file.
func (h *Handler) handleGetDPIPreset(w http.ResponseWriter, r *http.Request) {
	// Name is validated by readDPIPreset.
	preset, err := h.readDPIPreset(r.PathValue("name"))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "preset not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preset)
}

// readDPIPreset reads and parses a DPI preset file by name.
// The name is validated against [validPresetName] to prevent
// path traversal.
func (h *Handler) readDPIPreset(name string) (*dpiPreset, error) {
	if !validPresetName.MatchString(name) {
		return nil, fmt.Errorf("invalid preset name: %q", name)
	}
	data, err := os.ReadFile(filepath.Join(h.dpiDir, name+".json"))
	if err != nil {
		return nil, err
	}
	var preset dpiPreset
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, err
	}
	return &preset, nil
}

// handleApplyDPIPreset handles POST /api/presets/dpi/{name}/apply
// by reading the named preset file and applying its rules.
func (h *Handler) handleApplyDPIPreset(w http.ResponseWriter, r *http.Request) {
	// Name is validated by readDPIPreset.
	name := r.PathValue("name")
	preset, err := h.readDPIPreset(name)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "preset not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := applyDPIRules(h.dpi, preset.Rules); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.activeMu.Lock()
	h.activeName = name
	h.activePreset = preset
	h.activeMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}
