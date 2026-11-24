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
	want := map[string]string{"ctwa_clid": "CLID1", "meta_ad_id": "120247", "anuncio_titulo": "Mezcladora", "anuncio_origen": "FB_Ads", "anuncio_enlace": "https://fb.me/x"}
	if len(got) != len(want) {
		t.Fatalf("atributos = %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %s", k, got[k], v)
		}
	}
	if src := adAttributes(map[string]any{"sourceApp": "instagram"})["anuncio_origen"]; src != "instagram" {
		t.Errorf("sem utmSource usa o app de origem: %v", src)
	}
}

func TestMergeConversationAttrsKeepsOthers(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/accounts/1/conversations/55":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 55, "custom_attributes": map[string]any{"ciudad": "LIMA", "meta_ad_id": "viejo"}})
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
	if err := cfg.mergeConversationAttrs(55, map[string]any{"meta_ad_id": "nuevo", "ctwa_clid": "C"}); err != nil {
		t.Fatal(err)
	}
	if posted["ciudad"] != "LIMA" || posted["meta_ad_id"] != "nuevo" || posted["ctwa_clid"] != "C" {
		t.Fatalf("deve somar aos atributos existentes (o anúncio mais recente vence): %v", posted)
	}
}

func TestAccountAdAttributesOnlyDefinedKeys(t *testing.T) {
	defs := []map[string]any{{"attribute_key": "ciudad"}, {"attribute_key": "meta_ad_id"}}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(defs)
	}))
	defer srv.Close()
	ref := map[string]any{"ctwaClid": "C", "sourceId": "1", "title": "Mezcladora", "sourceUrl": "https://fb.me/x"}
	off := ChatwootConfig{URL: srv.URL, AccountID: 8, AccountToken: "t", InboxID: 1}
	if got := off.accountAdAttributes(ref); len(got) != 0 {
		t.Fatalf("sem ctwa_clid definido na conta: não grava nada, veio %v", got)
	}
	defs = append(defs, map[string]any{"attribute_key": "ctwa_clid"}, map[string]any{"attribute_key": "anuncio_titulo"})
	on := ChatwootConfig{URL: srv.URL, AccountID: 9, AccountToken: "t", InboxID: 2}
	got := on.accountAdAttributes(ref)
	if len(got) != 3 || got["ctwa_clid"] != "C" || got["meta_ad_id"] != "1" || got["anuncio_titulo"] != "Mezcladora" {
		t.Fatalf("só os atributos que a conta tem: %v", got)
	}
	_ = on.accountAdAttributes(ref)
	if calls != 2 {
		t.Fatalf("as definições ficam em cache por conta: %d chamadas", calls)
	}
}
