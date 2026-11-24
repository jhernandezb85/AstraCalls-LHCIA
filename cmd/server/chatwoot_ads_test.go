package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdAttributes(t *testing.T) {
	got := adAttributes(map[string]any{
		"ctwaClid": "CLID1", "sourceId": "120247", "title": " Mezcladora ", "utmSource": "FB_Ads",
		"sourceUrl": "https://fb.me/x", "body": "no va",
	})
	want := map[string]string{"ctwa_clid": "CLID1", "ad_id": "120247", "ad_title": "Mezcladora", "ad_source": "FB_Ads", "ad_url": "https://fb.me/x"}
	if len(got) != len(want) {
		t.Fatalf("atributos = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %s", k, got[k], v)
		}
	}
	if src := adAttributes(map[string]any{"sourceApp": "instagram"})["ad_source"]; src != "instagram" {
		t.Errorf("sem utmSource usa o app de origem: %v", src)
	}
}

func TestMergeConversationAttrsKeepsOthers(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/accounts/1/conversations/55":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 55, "custom_attributes": map[string]any{"ciudad": "LIMA", "ad_id": "viejo"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/accounts/1/conversations/55/custom_attributes":
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			posted = asMap(b["custom_attributes"])
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	cfg := ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 2}
	if err := cfg.mergeConversationAttrs(55, map[string]any{"ad_id": "nuevo", "ctwa_clid": "C"}); err != nil {
		t.Fatal(err)
	}
	if posted["ciudad"] != "LIMA" || posted["ad_id"] != "nuevo" || posted["ctwa_clid"] != "C" {
		t.Fatalf("deve somar aos atributos existentes (o anúncio mais recente vence): %v", posted)
	}
}
