// Copyright (c) 2026 Aristarh Ucolov.
//
// Steam Workshop "is this item updated?" probe. Hits the public
// ISteamRemoteStorage/GetPublishedFileDetails endpoint — no API key needed
// for public files. Used by the Mods page to flag mods whose Workshop copy
// is out of date relative to Steam (i.e. the user hasn't run Steam to pull
// the new files yet).

package mods_test

import (
	"context"
	"dayzmanager/internal/mods"
	"testing"
)

func TestFetchWorkshopMeta(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		ids     map[string]bool
		want    map[string]mods.WorkshopRemote
		wantErr bool
	}{
		{
			name: "Finds correct mod metadata",
			ids:  map[string]bool{"1710977250": true},
			want: map[string]mods.WorkshopRemote{
				"1710977250": {
					PublishedID: "1710977250",
					Title:       "BaseBuildingPlus",
					Result:      1,
					AppID:       221100,
				},
			},
			wantErr: false,
		},
		{
			name:    "Returns error when mod not found",
			ids:     map[string]bool{"00000000000000": true},
			want:    map[string]mods.WorkshopRemote{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := mods.FetchWorkshopMeta(context.Background(), tt.ids)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("FetchWorkshopMeta() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("FetchWorkshopMeta() succeeded unexpectedly")
			}

			for key, value := range got {
				wantMeta := tt.want[key]

				if value.PublishedID != wantMeta.PublishedID || value.Title != wantMeta.Title || value.AppID != wantMeta.AppID {
					t.Errorf("FetchWorkshopMeta() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
