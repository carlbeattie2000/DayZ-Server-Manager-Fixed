package mods_test

import (
	"dayzmanager/internal/mods"
	"testing"
)

func TestBuildRecords(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		fileContents string
		want         map[string]int64
	}{
		{
			name:         "single file entry",
			fileContents: "1564026768 1793273839",
			want: map[string]int64{
				"1564026768": 1793273839,
			},
		},
		{
			name:         "multiple file entries",
			fileContents: "1564026768 1793273839\n2430317812 1792592577",
			want: map[string]int64{
				"1564026768": 1793273839,
				"2430317812": 1792592577,
			},
		},
		{
			name:         "handles invalid line",
			fileContents: "1564026768 1793273839\n2430317812",
			want: map[string]int64{
				"1564026768": 1793273839,
			},
		},
		{
			name:         "handles line with more than required parts",
			fileContents: "1564026768 1793273839\n2430317812 1793273839 1793273839",
			want: map[string]int64{
				"1564026768": 1793273839,
				"2430317812": 1793273839,
			},
		},
		{
			name:         "handles empty files",
			fileContents: "",
			want:         map[string]int64{},
		},
		{
			name:         "handles invalid timestamp",
			fileContents: "1564026768 invalid_timestamp",
			want:         map[string]int64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mods.BuildRecords(tt.fileContents)

			for k, v := range got {
				wantVal, ok := tt.want[k]

				if !ok || v != wantVal {
					t.Errorf("BuildRecords() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
