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

// fakeConvCW responde a lista de conversas do contato e registra os POSTs.
type fakeConvCW struct {
	mu     sync.Mutex
	convs  []map[string]any // devolvidas em GET /contacts/{id}/conversations
	posts  []string         // "path body"
	nextID int
}

func newFakeConvCW(t *testing.T, convs []map[string]any) (*fakeConvCW, ChatwootConfig) {
	f := &fakeConvCW{convs: convs, nextID: 900}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/conversations") {
			_ = json.NewEncoder(w).Encode(map[string]any{"payload": f.convs})
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.posts = append(f.posts, r.URL.Path+" "+string(b))
		if strings.HasSuffix(r.URL.Path, "/conversations") {
			f.nextID++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": f.nextID})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	t.Cleanup(srv.Close)
	return f, ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 7}
}

func (f *fakeConvCW) post(suffix string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.posts {
		path, body, _ := strings.Cut(p, " ")
		if strings.HasSuffix(path, suffix) {
			return body
		}
	}
	return ""
}

func TestConversationStatusConfig(t *testing.T) {
	for in, want := range map[string]string{"": "open", "open": "open", "pending": "pending"} {
		if got := (ChatwootConfig{ConversationStatus: in}).conversationStatus(); got != want {
			t.Errorf("%q -> %q, quería %q", in, got, want)
		}
	}
	for _, ok := range []string{"", "open", "pending"} {
		if !validConversationStatus(ok) {
			t.Errorf("%q deveria ser válido", ok)
		}
	}
	if validConversationStatus("resolved") || validConversationStatus("x") {
		t.Error("só open/pending são aceitos")
	}
	if publicChatwootCfg(ChatwootConfig{}).ConversationStatus != "open" {
		t.Error("o GET mostra o valor efetivo")
	}
}

func TestEnsureConversationUsesConfiguredStatus(t *testing.T) {
	for _, st := range []string{"", "pending"} {
		f, cfg := newFakeConvCW(t, nil)
		cfg.ConversationStatus = st
		if _, err := cfg.ensureConversation(5, "src"); err != nil {
			t.Fatal(err)
		}
		want := `"status":"` + (ChatwootConfig{ConversationStatus: st}).conversationStatus() + `"`
		if body := f.post("/conversations"); !strings.Contains(body, want) {
			t.Errorf("status %q: corpo %s", st, body)
		}
	}
	// conversa ativa da inbox é reaproveitada, sem criar
	f, cfg := newFakeConvCW(t, []map[string]any{{"id": 44, "inbox_id": 7, "status": "pending"}})
	cfg.ConversationStatus = "pending"
	if id, _ := cfg.ensureConversation(5, "src"); id != 44 || f.post("/conversations") != "" {
		t.Fatalf("deveria reaproveitar 44, id=%d posts=%v", id, f.posts)
	}
}

func TestEnsureImportConversation(t *testing.T) {
	// ativa: reaproveita e não resolve no fim
	_, cfg := newFakeConvCW(t, []map[string]any{{"id": 10, "inbox_id": 7, "status": "open"}})
	if id, res, _ := cfg.ensureImportConversation(5, "s"); id != 10 || res {
		t.Fatalf("ativa: id=%d resolve=%v", id, res)
	}
	// só resolvidas (e de outra inbox): usa a resolvida mais recente da inbox e resolve de novo
	_, cfg = newFakeConvCW(t, []map[string]any{
		{"id": 11, "inbox_id": 7, "status": "resolved"},
		{"id": 15, "inbox_id": 7, "status": "resolved"},
		{"id": 99, "inbox_id": 8, "status": "open"},
	})
	if id, res, _ := cfg.ensureImportConversation(5, "s"); id != 15 || !res {
		t.Fatalf("resolvida: id=%d resolve=%v", id, res)
	}
	// nenhuma: cria ABERTA mesmo com a inbox em pendente (o Captain não deve responder histórico)
	f, cfg := newFakeConvCW(t, nil)
	cfg.ConversationStatus = "pending"
	id, res, err := cfg.ensureImportConversation(5, "s")
	if err != nil || id == 0 || !res {
		t.Fatalf("nova: id=%d resolve=%v err=%v", id, res, err)
	}
	if body := f.post("/conversations"); !strings.Contains(body, `"status":"open"`) {
		t.Fatalf("importação cria aberta: %s", body)
	}
	if err := cfg.setConversationStatus(id, convStatusResolved); err != nil {
		t.Fatal(err)
	}
	if body := f.post("/toggle_status"); !strings.Contains(body, `"status":"resolved"`) {
		t.Fatalf("toggle_status: %s", body)
	}
}
