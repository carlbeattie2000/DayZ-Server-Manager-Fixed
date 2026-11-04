package web

import (
	"dayzmanager/internal/mods"
	"net/http"
)

// ---------------------------------------------------------------------------
// Scheduled announcements.

// (The old /api/announcements endpoint was removed — scheduled announcements
// are now part of the manager config and saved via /api/config, item #2.)

// ---------------------------------------------------------------------------
// Mods: Workshop staleness check.
//
// Compares the *Workshop-side* mod folder mtime (i.e. the user's local Steam
// download) against Steam's published time_updated. A mod is "outdated" when
// the local Workshop copy is older than what Steam reports — meaning the
// user needs to run Steam to pull the new files before SyncAll can copy them
// into the server dir. We surface PublishedID, local time, remote time, and
// the diff so the frontend can render a clear stale/up-to-date state.
//
// Mods without a meta.cpp PublishedID are skipped (e.g. hand-rolled @Mods).

func (h *handlers) modsCheckUpdates(w http.ResponseWriter, r *http.Request) {
	// Works without a mod source too (server-only hosting): the installed
	// copies' meta.cpp timestamps are compared with Steam, so the admin still
	// learns which mods need a fresh upload.
	list, err := mods.List(h.app.ServerDir, h.app.Cfg().ModSourceDir())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	modsMeta := mods.CheckLocalModsForUpdates(r.Context(), list)

	if modsMeta.Error != "" {
		writeJSON(w, map[string]any{
			"error":   modsMeta.Error,
			"results": modsMeta.Results,
		})
	}

	writeJSON(w, map[string]any{
		"results": modsMeta.Results,
	})
}
