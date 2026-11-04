package mods

import (
	"context"
	"sort"
	"strings"
	"time"
)

type CheckLocalModsForUpdatesResp struct {
	Error   string
	Results []map[string]any
}

// localUpdated is when the local copy was published on the Workshop: meta.cpp's
// timestamp when present (exact), else the source copy's newest file, else the
// server copy's.
func localUpdated(m *Mod) time.Time {
	if t := MetaTime(m.MetaTimestamp); !t.IsZero() {
		return t
	}
	if !m.WorkshopModifiedAt.IsZero() {
		return m.WorkshopModifiedAt
	}
	return m.ServerModifiedAt
}

func BuildCheckResults(idToMod map[string]*Mod, remote map[string]WorkshopRemote) []map[string]any {
	out := make([]map[string]any, 0, len(idToMod))
	for id, m := range idToMod {
		row := map[string]any{
			"name":         m.Name,
			"publishedId":  id,
			"localUpdated": localUpdated(m),
		}
		r, ok := remote[id]
		if !ok {
			row["status"] = "unknown"
			out = append(out, row)
			continue
		}
		row["title"] = r.Title
		row["remoteUpdated"] = r.TimeUpdated
		row["remoteResult"] = r.Result
		switch {
		case r.Result != 1:
			row["status"] = "missing" // ID was removed/private on Steam
		case r.TimeUpdated.IsZero():
			row["status"] = "unknown"
		case localUpdated(m).IsZero():
			row["status"] = "unknown"
		case r.TimeUpdated.After(localUpdated(m).Add(2 * time.Minute)):
			row["status"] = "outdated"
		default:
			row["status"] = "ok"
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, _ := out[i]["name"].(string)
		aj, _ := out[j]["name"].(string)
		return strings.ToLower(ai) < strings.ToLower(aj)
	})
	return out
}

func CheckLocalModsForUpdates(ctx context.Context, localMods []Mod) CheckLocalModsForUpdatesResp {
	idToMod := map[string]*Mod{}

	for i := range localMods {
		m := &localMods[i]

		if m.PublishedID == "" {
			continue
		}

		idToMod[m.PublishedID] = m
	}

	ids := map[string]bool{}

	for id := range idToMod {
		ids[id] = true
	}

	var errorMessage string
	remote, err := FetchWorkshopMeta(ctx, ids)

	if err != nil {
		errorMessage = err.Error()
	}

	return CheckLocalModsForUpdatesResp{
		Error:   errorMessage,
		Results: BuildCheckResults(idToMod, remote),
	}
}
