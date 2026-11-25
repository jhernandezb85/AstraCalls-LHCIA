package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestIsPlaceholderName(t *testing.T) {
	for _, n := range []string{"", "  ", "51977415846", "+51 977 415 846", "(51) 977-415-846"} {
		if !isPlaceholderName(n, "51977415846") {
			t.Errorf("%q deveria ser só o número", n)
		}
	}
	for _, n := range []string{"José", "Tienda 24", "@SoyJoseLuisHB", "✨", "51977415846 Juan"} {
		if isPlaceholderName(n, "51977415846") {
			t.Errorf("%q é um nome de verdade", n)
		}
	}
}

// fakeNamesCW devolve contatos na busca e registra os PUTs de nome.
func fakeNamesCW(t *testing.T, contacts []map[string]any) (*[]string, ChatwootConfig) {
	var mu sync.Mutex
	puts := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contacts/search") {
			_ = json.NewEncoder(w).Encode(map[string]any{"payload": contacts})
			return
		}
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			*puts = append(*puts, r.URL.Path+" "+string(b))
			mu.Unlock()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	t.Cleanup(srv.Close)
	return puts, ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 7}
}

func TestFixContactName(t *testing.T) {
	puts, cfg := fakeNamesCW(t, []map[string]any{
		{"id": 1, "name": "51977415846", "phone_number": "+51977415846", "identifier": "51977415846@s.whatsapp.net"},
		{"id": 2, "name": "Nombre a mano", "phone_number": "+51977415846"},                          // nunca se pisa
		{"id": 3, "name": "51977415846", "phone_number": "+51900000000"},                           // otro teléfono
		{"id": 4, "name": "51977415846", "phone_number": "", "identifier": "51977415846-1@g.us"}, // grupo
	})
	if n := cfg.fixContactName("51977415846", "José Luis"); n != 1 {
		t.Fatalf("corrigidos = %d, quería 1; puts=%v", n, *puts)
	}
	if len(*puts) != 1 || !strings.HasSuffix(strings.Fields((*puts)[0])[0], "/contacts/1") || !strings.Contains((*puts)[0], `"name":"José Luis"`) {
		t.Fatalf("PUT inesperado: %v", *puts)
	}
}

func TestEnsureContactFixesPlaceholderName(t *testing.T) {
	puts, cfg := fakeNamesCW(t, []map[string]any{{
		"id": 9, "name": "51977415846", "phone_number": "+51977415846",
		"identifier": "51977415846@s.whatsapp.net",
		"custom_attributes": map[string]any{cwChatIDAttr: "51977415846@s.whatsapp.net"},
		"contact_inboxes":   []any{map[string]any{"source_id": "src", "inbox": map[string]any{"id": 7}}},
	}})
	if _, _, err := cfg.ensureContact("51977415846@s.whatsapp.net", "51977415846", "Ana", ""); err != nil {
		t.Fatal(err)
	}
	if len(*puts) != 1 || !strings.Contains((*puts)[0], `"name":"Ana"`) {
		t.Fatalf("deveria corrigir o nome: %v", *puts)
	}
	*puts = nil
	// nome que chega é só o número: não mexe
	_, _, _ = cfg.ensureContact("51977415846@s.whatsapp.net", "51977415846", "51977415846", "")
	if len(*puts) != 0 {
		t.Fatalf("não deveria atualizar: %v", *puts)
	}
}
