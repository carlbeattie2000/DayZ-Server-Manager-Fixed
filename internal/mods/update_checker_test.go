package mods_test

import (
	"dayzmanager/internal/mods"
	"testing"
	"time"
)

func TestBuildCheckResults(t *testing.T) {
	tests := []struct {
		name string

		idToMod map[string]*mods.Mod
		remote  map[string]mods.WorkshopRemote
		want    []map[string]any
	}{
		{
			name: "valid mod that does not need updating",
			idToMod: map[string]*mods.Mod{
				"1710977250": {
					Name:                "@BaseBuildingPlus",
					InstalledInServer:   true,
					AvailableInWorkshop: true,
					WorkshopPath:        "test\\path\\@BaseBuildingPlus",
					ServerPath:          "test\\path\\server\\@BaseBuildingPlus",
					SizeBytes:           320464000,
					KeyCount:            1,
					ServerModifiedAt:    time.Now(),
					WorkshopModifiedAt:  time.Now(),
					UpdateAvailable:     false,
					DisplayName:         "BaseBuildingPlus",
					PublishedID:         "1710977250",
					MetaTimestamp:       time.Now().Unix(),
					CaseProblems:        []string{},
				},
			},
			remote: map[string]mods.WorkshopRemote{
				"1710977250": {
					PublishedID: "1710977250",
					Title:       "BaseBuildingPlus",
					TimeUpdated: time.Now(),
					Result:      1,
					FileSize:    320464000,
					AppID:       2110000,
					Preview:     "",
				},
			},
			want: []map[string]any{
				{
					"publishedId":   "1710977250",
					"status":        "ok",
					"remoteUpdated": time.Now(),
				},
			},
		},
		{
			name: "mod that has upstream update avaliable",
			idToMod: map[string]*mods.Mod{
				"1710977250": {
					Name:                "@BaseBuildingPlus",
					InstalledInServer:   true,
					AvailableInWorkshop: true,
					WorkshopPath:        "test\\path\\@BaseBuildingPlus",
					ServerPath:          "test\\path\\server\\@BaseBuildingPlus",
					SizeBytes:           320464000,
					KeyCount:            1,
					ServerModifiedAt:    time.Now().Add(-(time.Hour * 10)),
					WorkshopModifiedAt:  time.Now().Add(-(time.Hour * 10)),
					UpdateAvailable:     false,
					DisplayName:         "BaseBuildingPlus",
					PublishedID:         "1710977250",
					MetaTimestamp:       time.Now().Add(-(time.Hour * 10)).Unix(),
					CaseProblems:        []string{},
				},
			},
			remote: map[string]mods.WorkshopRemote{
				"1710977250": {
					PublishedID: "1710977250",
					Title:       "BaseBuildingPlus",
					TimeUpdated: time.Now(),
					Result:      1,
					FileSize:    320464000,
					AppID:       2110000,
					Preview:     "",
				},
			},
			want: []map[string]any{
				{
					"publishedId":   "1710977250",
					"status":        "outdated",
					"remoteUpdated": time.Now(),
				},
			},
		},
		{
			name: "mod that has upstream update avaliable without a meta timestamp",
			idToMod: map[string]*mods.Mod{
				"1710977250": {
					Name:                "@BaseBuildingPlus",
					InstalledInServer:   true,
					AvailableInWorkshop: true,
					WorkshopPath:        "test\\path\\@BaseBuildingPlus",
					ServerPath:          "test\\path\\server\\@BaseBuildingPlus",
					SizeBytes:           320464000,
					KeyCount:            1,
					ServerModifiedAt:    time.Now().Add(-(time.Hour * 10)),
					WorkshopModifiedAt:  time.Now().Add(-(time.Hour * 10)),
					UpdateAvailable:     false,
					DisplayName:         "BaseBuildingPlus",
					PublishedID:         "1710977250",
					CaseProblems:        []string{},
				},
			},
			remote: map[string]mods.WorkshopRemote{
				"1710977250": {
					PublishedID: "1710977250",
					Title:       "BaseBuildingPlus",
					TimeUpdated: time.Now(),
					Result:      1,
					FileSize:    320464000,
					AppID:       2110000,
					Preview:     "",
				},
			},
			want: []map[string]any{
				{
					"publishedId":   "1710977250",
					"status":        "outdated",
					"remoteUpdated": time.Now(),
				},
			},
		},
		{
			name: "mod that has upstream update avaliable without a server and meta timestamp",
			idToMod: map[string]*mods.Mod{
				"1710977250": {
					Name:                "@BaseBuildingPlus",
					InstalledInServer:   true,
					AvailableInWorkshop: true,
					WorkshopPath:        "test\\path\\@BaseBuildingPlus",
					ServerPath:          "test\\path\\server\\@BaseBuildingPlus",
					SizeBytes:           320464000,
					KeyCount:            1,
					WorkshopModifiedAt:  time.Now().Add(-(time.Hour * 10)),
					UpdateAvailable:     false,
					DisplayName:         "BaseBuildingPlus",
					PublishedID:         "1710977250",
					CaseProblems:        []string{},
				},
			},
			remote: map[string]mods.WorkshopRemote{
				"1710977250": {
					PublishedID: "1710977250",
					Title:       "BaseBuildingPlus",
					TimeUpdated: time.Now(),
					Result:      1,
					FileSize:    320464000,
					AppID:       2110000,
					Preview:     "",
				},
			},
			want: []map[string]any{
				{
					"publishedId":   "1710977250",
					"status":        "outdated",
					"remoteUpdated": time.Now(),
				},
			},
		},
		{
			name: "mod that has upstream update avaliable without a workshop and meta timestamp",
			idToMod: map[string]*mods.Mod{
				"1710977250": {
					Name:                "@BaseBuildingPlus",
					InstalledInServer:   true,
					AvailableInWorkshop: true,
					WorkshopPath:        "test\\path\\@BaseBuildingPlus",
					ServerPath:          "test\\path\\server\\@BaseBuildingPlus",
					SizeBytes:           320464000,
					KeyCount:            1,
					ServerModifiedAt:    time.Now().Add(-(time.Hour * 10)),
					UpdateAvailable:     false,
					DisplayName:         "BaseBuildingPlus",
					PublishedID:         "1710977250",
					CaseProblems:        []string{},
				},
			},
			remote: map[string]mods.WorkshopRemote{
				"1710977250": {
					PublishedID: "1710977250",
					Title:       "BaseBuildingPlus",
					TimeUpdated: time.Now(),
					Result:      1,
					FileSize:    320464000,
					AppID:       2110000,
					Preview:     "",
				},
			},
			want: []map[string]any{
				{
					"publishedId":   "1710977250",
					"status":        "outdated",
					"remoteUpdated": time.Now(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mods.BuildCheckResults(tt.idToMod, tt.remote)

			for index := range tt.want {
				wantMod := tt.want[index]
				gotMod := got[index]

				if wantMod["publishedId"] != gotMod["publishedId"] || wantMod["remoteUpdated"] != gotMod["remoteUpdated"] || wantMod["status"] != gotMod["status"] {
					t.Errorf("BuildCheckResults() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
