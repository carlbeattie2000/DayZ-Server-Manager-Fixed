// Copyright (c) 2026 Aristarh Ucolov.
//
// Steam Workshop Collection parser.
//
// A Steam Workshop collection page is a plain HTML listing of child items.
// Each child item has a filedetails link of the form
// `/sharedfiles/filedetails/?id=<publishedid>`. We do not need a full parser
// — a regex over the HTML is both sufficient and far smaller than pulling
// in an HTML tree.
//
// Network call is opt-in and isolated to this file so the rest of the mod
// package stays offline-friendly.
package mods

import (
	"context"
	"dayzmanager/internal/util"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	reCollectionID       = regexp.MustCompile(`[?&]id=(\d+)`)
	reChildIDs           = regexp.MustCompile(`(?:sharedfiles|workshop)/filedetails/\?id=(\d+)`)
	reCollectionChildIds = regexp.MustCompile(`sharedfile_(\d+)`)
	reIsCollection       = regexp.MustCompile(`id="mainContentsCollection"`)
)

type CollectionResolution struct {
	CollectionID string   `json:"collectionId"`
	ChildIDs     []string `json:"childIds"`
	// Resolved: Workshop mods actually present under !Workshop, keyed by ID.
	Resolved []ResolvedMod `json:"resolved"`
	// Missing IDs the user has not yet subscribed/downloaded in Steam.
	Missing []string `json:"missing"`
}

type ResolvedMod struct {
	PublishedID string `json:"publishedId"`
	ModName     string `json:"modName"` // @Foo
	DisplayName string `json:"displayName,omitempty"`
}

// ParseCollectionURL extracts the numeric workshop ID from any Steam URL.
// Accepts bare IDs too so users can paste "1234567890" if they prefer.
func ParseCollectionURL(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty url")
	}
	// Bare numeric ID.
	if _, err := fmt.Sscanf(input, "%d", new(uint64)); err == nil && !strings.ContainsAny(input, "/?=&") {
		return input, nil
	}
	if !strings.HasPrefix(input, "http") {
		// Let users paste without scheme ("steamcommunity.com/.../?id=...").
		input = "https://" + input
	}
	u, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	if id := u.Query().Get("id"); id != "" {
		return id, nil
	}
	if m := reCollectionID.FindStringSubmatch(input); len(m) == 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("could not find collection id in URL")
}

type WorkshopItemSearchResult struct {
	ParentID     string
	IsCollection bool
	SearchBody   string
}

// A collection and mod links are identical.
// When a workshop page is fetched, we look for collection|mod identifiers
// on the page. Collections have children and mods have required items.
// Searching for child links should be contained to the respective child item
// containers. If this is not done, a workshop link could be placed in the mods description
// and would be fetched as a child item
//
// We can only need a single map to track both visited and the mods found.
// Neither a collection or mod can have a child item be another collection
// So aslong as the collection ID is never added to the visited map, we can
// use that map for what will be downloaded
func FetchSteamWorkshopItems(workshopItemIDs []string) map[string]bool {
	visitedWorkshopIds := map[string]bool{}

	for _, id := range workshopItemIDs {
		if visitedWorkshopIds[id] {
			continue
		}

		workshopItemMetadata, err := FetchSteamWorkshopItem(id)

		if err != nil {
			continue
		}

		if workshopItemMetadata.IsCollection {
			for childID := range FetchCollectionChildren(workshopItemMetadata) {
				visitedWorkshopIds[childID] = true
			}
			continue
		}

		for childID := range FetchModChildren(workshopItemMetadata) {
			visitedWorkshopIds[childID] = true
		}

		visitedWorkshopIds[id] = true
	}

	return visitedWorkshopIds
}

func FetchSteamWorkshopItem(workshopItemID string) (*WorkshopItemSearchResult, error) {
	if workshopItemID == "" {
		return nil, fmt.Errorf("workshop item id required")
	}

	uri := "https://steamcommunity.com/sharedfiles/filedetails/?id=" + url.QueryEscape(workshopItemID)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", uri, nil)

	if err != nil {
		return nil, err
	}

	// Steam serves a cut-down page to unidentified UAs; a browser-like UA
	// returns the full list.
	req.Header.Set("User-Agent", "Mozilla/5.0 (dayz-manager; +https://github.com/)")
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}

	var isCollection = reIsCollection.Match(body)
	var searchBody string

	if isCollection {
		searchBody = util.ExtractHTMLContainerByAttr(string(body), "class", "collectionChildren")
	} else {
		searchBody = util.ExtractHTMLContainerByAttr(string(body), "id", "RequiredItems")
	}

	return &WorkshopItemSearchResult{
		ParentID:     workshopItemID,
		IsCollection: isCollection,
		SearchBody:   searchBody,
	}, nil
}

func FetchModChildren(modSearchResult *WorkshopItemSearchResult) map[string]bool {
	seen := map[string]bool{modSearchResult.ParentID: true}

	for _, match := range reChildIDs.FindAllStringSubmatch(modSearchResult.SearchBody, -1) {
		seen[match[1]] = true
	}

	return seen
}

func FetchCollectionChildren(collectionSearchResult *WorkshopItemSearchResult) map[string]bool {
	seen := map[string]bool{collectionSearchResult.ParentID: true}

	for _, match := range reCollectionChildIds.FindAllStringSubmatch(collectionSearchResult.SearchBody, -1) {
		seen[match[1]] = true
	}

	delete(seen, collectionSearchResult.ParentID)

	return seen
}

// ResolveCollection matches a list of publishedids to @Mod directories in
// the mod source (client !Workshop, custom folder or SteamCMD content). Three-pass strategy:
//
//  1. Match by meta.cpp PublishedID (cheapest, exact).
//  2. For unmatched IDs, hit Steam GetPublishedFileDetails to learn the title
//     and match by normalized folder name ("@FooBar" ↔ "Foo Bar"). This
//     catches mods where the user's local meta.cpp is stale or missing,
//     which is common for older DayZ mods.
//  3. Anything still unmatched lands in Missing (user genuinely needs to
//     subscribe in Steam).
//
// The Steam call is best-effort — if it fails, fallback IDs stay missing.
func ResolveCollection(ctx context.Context, srcDir string, ids []string) (*CollectionResolution, error) {
	if srcDir == "" {
		return nil, ErrNoSource
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}

	// Snapshot source mods once with their meta.cpp so we don't re-stat.
	type wsEntry struct {
		Folder      string // "@CommunityFramework"
		PublishedID string // "" if no meta.cpp / no publishedid line
		Name        string // human-readable from meta.cpp; "" if absent
	}
	usedFolder := map[string]bool{}
	var folders []wsEntry
	for _, sm := range scanSourceList(srcDir) {
		meta := readMeta(sm.path)
		pid := meta.PublishedID
		if pid == "" && sm.steamcmd {
			pid = filepath.Base(sm.path) // SteamCMD folders ARE the published id
		}
		folders = append(folders, wsEntry{Folder: sm.name, PublishedID: pid, Name: meta.Name})
	}

	byID := map[string]ResolvedMod{}

	// Pass 1: exact PublishedID match.
	for _, f := range folders {
		if f.PublishedID == "" || !want[f.PublishedID] {
			continue
		}
		byID[f.PublishedID] = ResolvedMod{PublishedID: f.PublishedID, ModName: f.Folder, DisplayName: f.Name}
		usedFolder[f.Folder] = true
	}

	// Pass 2: Steam-aided name match for the leftover IDs.
	leftover := make([]string, 0)
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			leftover = append(leftover, id)
		}
	}
	if len(leftover) > 0 {
		// Bound the Steam call so a slow upstream never hangs the request.
		sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		var leftoverIdsMap map[string]bool
		for _, id := range leftover {
			leftoverIdsMap[id] = true
		}
		remote, err := FetchWorkshopMeta(sctx, leftoverIdsMap)
		if err == nil {
			// Build lookup from normalized folder name → wsEntry.
			folderNorm := map[string]wsEntry{}
			for _, f := range folders {
				if usedFolder[f.Folder] {
					continue
				}
				folderNorm[normalizeName(strings.TrimPrefix(f.Folder, "@"))] = f
			}
			for id, r := range remote {
				if r.Result != 1 || r.Title == "" {
					continue
				}
				key := normalizeName(r.Title)
				if f, ok := folderNorm[key]; ok && !usedFolder[f.Folder] {
					byID[id] = ResolvedMod{PublishedID: id, ModName: f.Folder, DisplayName: f.Name}
					usedFolder[f.Folder] = true
					delete(folderNorm, key)
				}
			}
		}
	}

	res := &CollectionResolution{ChildIDs: ids}
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			res.Resolved = append(res.Resolved, r)
		} else {
			res.Missing = append(res.Missing, id)
		}
	}
	return res, nil
}

// normalizeName lowercases and strips non-alphanumerics so "Community-Online
// Tools!" and "@CommunityOnlineTools" match. DayZ mod authors are creative
// with separators, so this is the cheapest fuzzy match that is still safe.
func normalizeName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
