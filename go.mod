module dayzmanager

go 1.23

// Go 1.23 reads Windows junctions as irregular files and stops resolving
// them; DayZ Launcher links !Workshop mods that way. Keep the old behaviour.
godebug winsymlink=0

require (
	github.com/ebitengine/purego v0.10.0
	github.com/jchv/go-webview2 v0.0.0-20260205173254-56598839c808
	golang.org/x/sys v0.28.0
)

require (
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	golang.org/x/net v0.33.0 // indirect
)
