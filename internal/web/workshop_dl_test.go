// Copyright (c) 2026 Aristarh Ucolov.
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWorkshopIDs(t *testing.T) {
	got := workshopIDs("https://steamcommunity.com/sharedfiles/filedetails/?id=1559212036\n" +
		"https://steamcommunity.com/workshop/filedetails/?l=russian&id=2116151222, 1559212036; 1828439124\nnot a link 12")
	want := []string{"1559212036", "2116151222", "1828439124"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The Steam password is refused from another device over plain HTTP.
func TestWorkshopLoginRefusedWhenInsecure(t *testing.T) {
	h, _ := profileApp(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/workshop/dl/login", strings.NewReader(`{"user":"bob","password":"secret"}`))
	req.RemoteAddr = "192.168.1.50:5555"
	h.wsLogin(rec, req)
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), "workshop.login.insecure") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if h.app.Cfg().SteamUser != "" {
		t.Fatal("the account was stored although the login was refused")
	}
}

// A downloaded item installs under its meta.cpp name, brings its keys and
// is switched on — once, even when installed again (an update).
func TestWorkshopInstallFromSteamCMD(t *testing.T) {
	h, a := profileApp(t)
	content := filepath.Join(t.TempDir(), "221100")
	item := filepath.Join(content, "1559212036")
	for _, f := range []struct{ p, data string }{
		{"meta.cpp", "protocol = 1;\npublishedid = 1559212036;\nname = \"Community Framework\";\ntimestamp = 5249743004478005813;\n"},
		{"addons/cf.pbo", "pbo"},
		{"keys/cf.bikey", "key"},
	} {
		p := filepath.Join(item, filepath.FromSlash(f.p))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f.data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	name, err := h.wsInstall(content, "1559212036", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "@") {
		t.Fatalf("name %q", name)
	}
	if _, err := os.Stat(filepath.Join(a.ServerDir, name, "addons", "cf.pbo")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.ServerDir, "keys", "cf.bikey")); err != nil {
		t.Fatal("key not merged:", err)
	}
	if _, err := h.wsInstall(content, "1559212036", true); err != nil {
		t.Fatal(err)
	}
	cfg := a.Cfg()
	if len(cfg.Mods) != 1 || cfg.Mods[0] != name || len(cfg.ServerMods) != 0 {
		t.Fatalf("mods %v server mods %v", cfg.Mods, cfg.ServerMods)
	}
	if _, err := h.wsInstall(content, "999", false); err == nil {
		t.Fatal("a missing item installed")
	}
}

// "Install & update mods" of the Workshop source needs a Steam sign-in first.
func TestWorkshopSyncNeedsLogin(t *testing.T) {
	h, _ := profileApp(t)
	rec := httptest.NewRecorder()
	h.wsSync(rec, httptest.NewRequest("POST", "/api/workshop/dl/sync", strings.NewReader(`{}`)))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "workshop.needLogin") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.workshopSync(context.Background(), nil, true); err == nil {
		t.Fatal("a sync ran without SteamCMD or a Steam account")
	}
}

// The download list is a report: a mod deleted from the server since shows
// as gone, one row or the whole list can be cleared, never while running.
func TestWorkshopListClearAndGone(t *testing.T) {
	h, a := profileApp(t)
	h.server = &Server{app: a}
	st := h.ws()
	if err := os.MkdirAll(filepath.Join(a.ServerDir, "@Kept"), 0o755); err != nil {
		t.Fatal(err)
	}
	st.job = &wsJob{Kind: "download", State: "done", ServerID: a.ID, Items: []wsItem{
		{ID: "1", State: "done", Mod: "@Kept"}, {ID: "2", State: "done", Mod: "@Deleted"}, {ID: "3", State: "error"},
	}}
	status := func() string {
		rec := httptest.NewRecorder()
		h.wsStatus(rec, httptest.NewRequest("GET", "/api/workshop/dl/status", nil))
		return rec.Body.String()
	}
	if s := status(); strings.Count(s, `"gone":true`) != 1 || !strings.Contains(s, `"mod":"@Deleted","gone":true`) {
		t.Fatalf("status: %s", s)
	}
	clear := func(body string) int {
		rec := httptest.NewRecorder()
		h.wsClear(rec, httptest.NewRequest("POST", "/api/workshop/dl/clear", strings.NewReader(body)))
		return rec.Code
	}
	if code := clear(`{"id":"2"}`); code != 200 || len(st.job.Items) != 2 {
		t.Fatalf("remove one: %d %+v", code, st.job)
	}
	st.job.State = "running"
	if code := clear(`{}`); code != 409 {
		t.Fatalf("cleared a running job: %d", code)
	}
	st.job.State = "done"
	if code := clear(`{}`); code != 200 || st.job != nil {
		t.Fatalf("clear all: %d %+v", code, st.job)
	}
}

type wsLookupWorkshopItemRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Size      int64  `json:"size"`
	Updated   int64  `json:"updated"`
	Found     bool   `json:"found"`
	DayZ      bool   `json:"dayz"`
	Installed string `json:"installed,omitempty"`
	Preview   string `json:"preview,omitempty"`
}

type wsLookupResp struct {
	Items []wsLookupWorkshopItemRow
}

func TestWsLookupFindsModMetadata(t *testing.T) {
	handler, app := profileApp(t)
	handler.server = &Server{app: app}

	body := strings.NewReader(`{"text": "2545327648"}`)
	rec := httptest.NewRecorder()
	handler.wsLookup(rec, httptest.NewRequest("POST", "/api/workshop/dl/lookup", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("wsLookup failed with code: %d: %s", rec.Code, rec.Body.String())
	}

	var resp wsLookupResp

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("wsLookup bad json: %v: %s", err, rec.Body.String())
	}

	var mod wsLookupWorkshopItemRow = resp.Items[0]

	if !mod.Found {
		t.Fatalf("mod %s was not found", mod.ID)
	}

	if !mod.DayZ {
		t.Fatalf("mod %s is not a dayz mod", mod.ID)
	}
}

func TestWorkshopSingleModReturnsItself(t *testing.T) {
	handler, app := profileApp(t)
	handler.server = &Server{app: app}

	body := strings.NewReader(`{"text": "2545327648"}`)
	rec := httptest.NewRecorder()
	handler.wsLookup(rec, httptest.NewRequest("POST", "/api/workshop/dl/lookup", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("wsLookup failed with code: %d: %s", rec.Code, rec.Body.String())
	}

	var resp wsLookupResp

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("wsLookup bad json: %v: %s", err, rec.Body.String())
	}

	if len(resp.Items) != 1 {
		t.Fatalf("want exactly 1 ws item, got %d: %+v", len(resp.Items), resp.Items)
	}

	if resp.Items[0].ID != "2545327648" {
		t.Fatalf("want 2545327648 ws ID, got %s", resp.Items[0].ID)
	}

	if !resp.Items[0].Found {
		t.Fatalf("mod %s was not found", resp.Items[0].ID)
	}
}

func TestWorkshopModWithLinksButNoDependenciesReturnsItself(t *testing.T) {
	handler, app := profileApp(t)
	handler.server = &Server{app: app}

	body := strings.NewReader(`{"text": "1567720365"}`)
	rec := httptest.NewRecorder()
	handler.wsLookup(rec, httptest.NewRequest("POST", "/api/workshop/dl/lookup", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("wsLookup failed with code: %d: %s", rec.Code, rec.Body.String())
	}

	var resp wsLookupResp

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("wsLookup bad json: %v: %s", err, rec.Body.String())
	}

	if len(resp.Items) != 1 {
		t.Fatalf("want exactly 1 ws item, got %d: %+v", len(resp.Items), resp.Items)
	}

	if resp.Items[0].ID != "1567720365" {
		t.Fatalf("want 1567720365 ws ID, got %s", resp.Items[0].ID)
	}
}

func TestWorkshopModReturnsRequiredItemsOnly(t *testing.T) {
	handler, app := profileApp(t)
	handler.server = &Server{app: app}

	body := strings.NewReader(`{"text": "1932611410"}`)
	rec := httptest.NewRecorder()
	handler.wsLookup(rec, httptest.NewRequest("POST", "/api/workshop/dl/lookup", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("wsLookup failed with code: %d: %s", rec.Code, rec.Body.String())
	}

	var resp wsLookupResp

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("wsLookup bad json: %v: %s", err, rec.Body.String())
	}

	if len(resp.Items) != 3 {
		t.Fatalf("want exactly 3 ws items, got %d: %+v", len(resp.Items), resp.Items)
	}

	expected := []string{"1932611410", "3690289718", "1559212036"}

	for _, item := range resp.Items {
		if !slices.Contains(expected, item.ID) {
			t.Fatalf("missing expected ID")
		}
	}
}

func TestWorkshopCollectionReturnsChildren(t *testing.T) {
	handler, app := profileApp(t)
	handler.server = &Server{app: app}

	body := strings.NewReader(`{"text": "https://steamcommunity.com/workshop/filedetails/?id=1600413591"}`)
	rec := httptest.NewRecorder()
	handler.wsLookup(rec, httptest.NewRequest("POST", "/api/workshop/dl/lookup", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("wsLookup failed with code: %d: %s", rec.Code, rec.Body.String())
	}

	var resp wsLookupResp

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("wsLookup bad json: %v: %s", err, rec.Body.String())
	}

	if slices.ContainsFunc(resp.Items, func(item wsLookupWorkshopItemRow) bool {
		return item.ID == "1600413591"
	}) {
		t.Fatalf("should not include collection ID: %+v", resp.Items)
	}

	if len(resp.Items) != 8 {
		t.Fatalf("want exactly 8 ws items, got %d: %+v", len(resp.Items), resp.Items)
	}
}
