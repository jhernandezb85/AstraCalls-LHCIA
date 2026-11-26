package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestValidArchiveSync(t *testing.T) {
	for _, ok := range []string{"", "off", "resolved", "pending"} {
		if !validArchiveSync(ok) {
			t.Errorf("%q deveria ser válido", ok)
		}
	}
	if validArchiveSync("archive") {
		t.Error("valor inválido aceito")
	}
	if (ChatwootConfig{ArchiveSync: "off"}).archiveSync() != "" || (ChatwootConfig{}).archiveSync() != "" {
		t.Error("padrão deve ser desligado")
	}
}

func TestEchoMarks(t *testing.T) {
	markEcho("k1", "resolved")
	if takeEcho("k1", "open") {
		t.Error("valor diferente não é eco")
	}
	if !takeEcho("k1", "resolved") {
		t.Error("deveria ser eco")
	}
	if takeEcho("k1", "resolved") {
		t.Error("eco só é consumido uma vez")
	}
}

func TestStatusChangeChat(t *testing.T) {
	mk := func(sender map[string]any) map[string]any {
		return map[string]any{"meta": map[string]any{"sender": sender}}
	}
	if j, ok := statusChangeChat(mk(map[string]any{"identifier": "5215550001234@s.whatsapp.net"})); !ok || j.User != "5215550001234" {
		t.Errorf("identifier PN: %v %v", j, ok)
	}
	if j, ok := statusChangeChat(mk(map[string]any{"identifier": "120363000000000000@g.us"})); !ok || j.Server != types.GroupServer {
		t.Errorf("grupo: %v %v", j, ok)
	}
	if j, ok := statusChangeChat(mk(map[string]any{"phone_number": "+52 155 5000 1234"})); !ok || j.User != "5215550001234" {
		t.Errorf("telefone: %v %v", j, ok)
	}
	if _, ok := statusChangeChat(mk(map[string]any{"name": "AstraCalls (sistema)"})); ok {
		t.Error("contato sem identificador não deve virar chat")
	}
}

// archFakeCW: um contato (id 5) com uma conversa (id 77, status inicial) na inbox 7.
func archFakeCW(t *testing.T, status string) (*httptest.Server, func() []string) {
	var mu sync.Mutex
	var toggles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/contacts/search"):
			_, _ = w.Write([]byte(`{"payload":[{"id":5,"identifier":"5215550001234@s.whatsapp.net","phone_number":"+5215550001234"}]}`))
		case strings.HasSuffix(r.URL.Path, "/contacts/5/conversations"):
			mu.Lock()
			st := status
			mu.Unlock()
			_, _ = w.Write([]byte(`{"payload":[{"id":77,"inbox_id":7,"status":"` + st + `"}]}`))
		case strings.HasSuffix(r.URL.Path, "/toggle_status"):
			b, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			mu.Lock()
			toggles = append(toggles, asStr(m["status"]))
			status = asStr(m["status"])
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), toggles...) }
}

func archiveEvt(archived, fullSync bool) *events.Archive {
	return &events.Archive{
		JID:          types.NewJID("5215550001234", types.DefaultUserServer),
		Action:       &waSyncAction.ArchiveChatAction{Archived: proto.Bool(archived)},
		FromFullSync: fullSync,
	}
}

func archiveSession(url, mode string) *Session {
	s := &Session{id: "arch-" + mode, log: slog.Default()}
	s.setChatwoot(ChatwootConfig{URL: url, AccountID: 1, AccountToken: "t", InboxID: 7, ArchiveSync: mode})
	return s
}

func TestArchiveResolvesAndUnarchiveReopens(t *testing.T) {
	srv, toggles := archFakeCW(t, "open")
	s := archiveSession(srv.URL, archiveSyncResolved)
	s.chatwootArchiveEvent(archiveEvt(true, false))
	s.chatwootArchiveEvent(archiveEvt(false, false))
	if got := toggles(); len(got) != 2 || got[0] != "resolved" || got[1] != "open" {
		t.Fatalf("toggles: %v", got)
	}
}

func TestArchivePendingMode(t *testing.T) {
	srv, toggles := archFakeCW(t, "open")
	s := archiveSession(srv.URL, archiveSyncPending)
	s.chatwootArchiveEvent(archiveEvt(true, false))
	if got := toggles(); len(got) != 1 || got[0] != "pending" {
		t.Fatalf("toggles: %v", got)
	}
}

func TestArchiveIgnoredWhenOffOrFullSync(t *testing.T) {
	srv, toggles := archFakeCW(t, "open")
	archiveSession(srv.URL, "").chatwootArchiveEvent(archiveEvt(true, false))
	archiveSession(srv.URL, archiveSyncResolved).chatwootArchiveEvent(archiveEvt(true, true))
	if got := toggles(); len(got) != 0 {
		t.Fatalf("não deveria mudar nada: %v", got)
	}
}

func TestStatusChangeEchoIsIgnored(t *testing.T) {
	srv, _ := archFakeCW(t, "open")
	s := archiveSession(srv.URL, archiveSyncResolved)
	cfg := s.getChatwoot()
	// o AstraCalls resolveu a conversa (ex.: importação): o webhook que volta é eco
	_ = cfg.setConversationStatus(77, convStatusResolved)
	if !takeEcho(cwStatusEchoKey(cfg, 77), convStatusResolved) {
		t.Fatal("setConversationStatus deveria marcar o eco")
	}
}
