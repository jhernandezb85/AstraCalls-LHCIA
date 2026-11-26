package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestOrphanInboxDeletesAfterStrikes(t *testing.T) {
	var code atomic.Int32
	code.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(code.Load()))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	s := &Session{id: "orph1", log: slog.Default()}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 9})
	m := &SessionManager{log: slog.Default(), sessions: map[string]*Session{"orph1": s}}

	var deleted []string
	old := deleteOrphanSession
	deleteOrphanSession = func(_ *SessionManager, _ context.Context, id string) error { deleted = append(deleted, id); return nil }
	defer func() { deleteOrphanSession = old }()

	strikes := newOrphanState()
	m.checkOrphanInboxes(context.Background(), strikes) // existe: fica "vista"
	code.Store(404)
	m.checkOrphanInboxes(context.Background(), strikes)
	m.checkOrphanInboxes(context.Background(), strikes)
	code.Store(500) // Chatwoot caído: zera a contagem, não apaga
	m.checkOrphanInboxes(context.Background(), strikes)
	code.Store(404)
	m.checkOrphanInboxes(context.Background(), strikes)
	m.checkOrphanInboxes(context.Background(), strikes)
	if len(deleted) != 0 {
		t.Fatalf("não pode apagar antes de %d 404 seguidos: %v", orphanStrikes, deleted)
	}
	m.checkOrphanInboxes(context.Background(), strikes)
	if len(deleted) != 1 || deleted[0] != "orph1" {
		t.Fatalf("deveria apagar a sessão órfã: %v", deleted)
	}
}

func TestOrphanInboxKeepsExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":9}`))
	}))
	defer srv.Close()
	s := &Session{id: "orph2", log: slog.Default()}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 9})
	m := &SessionManager{log: slog.Default(), sessions: map[string]*Session{"orph2": s}}
	var deleted int
	old := deleteOrphanSession
	deleteOrphanSession = func(_ *SessionManager, _ context.Context, _ string) error { deleted++; return nil }
	defer func() { deleteOrphanSession = old }()
	strikes := newOrphanState()
	for i := 0; i < 5; i++ {
		m.checkOrphanInboxes(context.Background(), strikes)
	}
	if deleted != 0 {
		t.Fatal("inbox existente nunca apaga")
	}
}

func TestOrphanNeverSeenInboxIsKept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	s := &Session{id: "orph3", log: slog.Default()}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 9})
	m := &SessionManager{log: slog.Default(), sessions: map[string]*Session{"orph3": s}}
	var deleted int
	old := deleteOrphanSession
	deleteOrphanSession = func(_ *SessionManager, _ context.Context, _ string) error { deleted++; return nil }
	defer func() { deleteOrphanSession = old }()
	st := newOrphanState()
	for i := 0; i < 6; i++ {
		m.checkOrphanInboxes(context.Background(), st)
	}
	if deleted != 0 {
		t.Fatal("inbox que nunca se viu existir (config antiga) não apaga a sessão")
	}
}
