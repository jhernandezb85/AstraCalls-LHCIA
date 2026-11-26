package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeWatch: inboxWatchStore em memória.
type fakeWatch struct {
	mu sync.Mutex
	w  map[string]inboxWatch
}

func (f *fakeWatch) getInboxWatch(_ context.Context, sid string) (inboxWatch, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.w[sid]
	return w, ok
}
func (f *fakeWatch) setInboxSeen(_ context.Context, sid, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.w[sid] = inboxWatch{key: key, seen: true, orphanSince: f.w[sid].orphanSince}
	return nil
}
func (f *fakeWatch) clearInboxOrphan(_ context.Context, sid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if w, ok := f.w[sid]; ok {
		w.orphanSince = 0
		f.w[sid] = w
	}
	return nil
}
func (f *fakeWatch) setInboxOrphan(_ context.Context, sid, key string, since int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if w, ok := f.w[sid]; ok && w.key == key && w.seen {
		w.orphanSince = since
		f.w[sid] = w
	}
	return nil
}
func (f *fakeWatch) orphansBefore(_ context.Context, before int64) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for id, w := range f.w {
		if w.orphanSince != 0 && w.orphanSince < before {
			out = append(out, id)
		}
	}
	return out, nil
}

type orphanHarness struct {
	code    atomic.Int32
	watch   *fakeWatch
	m       *SessionManager
	s       *Session
	logouts []string
	deletes []string
	linked  atomic.Bool
}

func newOrphanHarness(t *testing.T) *orphanHarness {
	h := &orphanHarness{watch: &fakeWatch{w: map[string]inboxWatch{}}}
	h.code.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(h.code.Load()))
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	h.s = &Session{id: "orph", log: slog.Default()}
	h.s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 9})
	h.m = &SessionManager{log: slog.Default(), sessions: map[string]*Session{"orph": h.s}}

	oldStore, oldSleep, oldLogout, oldDelete, oldLinked := orphanStore, orphanSleep, orphanLogout, orphanDelete, orphanLinked
	orphanLinked = func(*Session) bool { return h.linked.Load() }
	orphanStore = func(*SessionManager) inboxWatchStore { return h.watch }
	orphanSleep = func(context.Context, time.Duration) bool { return true }
	orphanLogout = func(_ *SessionManager, _ context.Context, id string) error { h.logouts = append(h.logouts, id); return nil }
	orphanDelete = func(_ *SessionManager, _ context.Context, id string) error { h.deletes = append(h.deletes, id); return nil }
	t.Cleanup(func() {
		orphanStore, orphanSleep, orphanLogout, orphanDelete, orphanLinked = oldStore, oldSleep, oldLogout, oldDelete, oldLinked
	})
	return h
}

func TestOrphanSeenThenDeletedLogsOutAndPurgesAfterGrace(t *testing.T) {
	h := newOrphanHarness(t)
	ctx := context.Background()
	h.m.checkInbox(ctx, h.s) // existe: fica "vista"
	if w, _ := h.watch.getInboxWatch(ctx, "orph"); !w.seen {
		t.Fatal("inbox existente deve ficar marcada como vista")
	}
	h.code.Store(404)
	h.m.checkInbox(ctx, h.s)
	if len(h.logouts) != 1 {
		t.Fatalf("404 confirmado deve desvincular o número: %v", h.logouts)
	}
	w, _ := h.watch.getInboxWatch(ctx, "orph")
	if w.orphanSince == 0 {
		t.Fatal("deve marcar a sessão como órfã")
	}
	h.m.purgeOrphans(ctx)
	if len(h.deletes) != 0 {
		t.Fatal("não apaga antes de 24 h")
	}
	// passaram 25 h
	h.watch.w["orph"] = inboxWatch{key: w.key, seen: true, orphanSince: time.Now().Add(-25 * time.Hour).UnixMilli()}
	h.m.purgeOrphans(ctx)
	if len(h.deletes) != 1 {
		t.Fatalf("após 24 h apaga a sessão: %v", h.deletes)
	}
	// verificar de novo não desvincula outra vez
	h.m.checkInbox(ctx, h.s)
	if len(h.logouts) != 1 {
		t.Fatal("sessão já órfã não é desvinculada de novo")
	}
}

func TestOrphanNeverSeenIsKept(t *testing.T) {
	h := newOrphanHarness(t)
	h.code.Store(404)
	for i := 0; i < 3; i++ {
		h.m.checkInbox(context.Background(), h.s)
	}
	if len(h.logouts) != 0 {
		t.Fatal("inbox nunca vista existindo (config antiga) não desvincula")
	}
}

func TestOrphanTransientErrorsIgnored(t *testing.T) {
	h := newOrphanHarness(t)
	ctx := context.Background()
	h.m.checkInbox(ctx, h.s)
	for _, c := range []int32{500, 502, 401} {
		h.code.Store(c)
		h.m.checkInbox(ctx, h.s)
	}
	if len(h.logouts) != 0 {
		t.Fatal("erros que não são 404 nunca desvinculam")
	}
}

func TestOrphanRelinkedInboxIsNotOrphan(t *testing.T) {
	h := newOrphanHarness(t)
	ctx := context.Background()
	h.m.checkInbox(ctx, h.s)
	h.code.Store(404)
	// durante a confirmação, a sessão é ligada a outra inbox (recuperada)
	orphanSleep = func(context.Context, time.Duration) bool {
		cfg := h.s.getChatwoot()
		cfg.InboxID = 10
		h.s.setChatwoot(cfg)
		return true
	}
	h.m.checkInbox(ctx, h.s)
	if len(h.logouts) != 0 {
		t.Fatal("sessão religada a outra inbox não é desvinculada")
	}
}

// orfaniza a sessão: inbox vista e depois 404 confirmado.
func (h *orphanHarness) orphanize(t *testing.T) inboxWatch {
	ctx := context.Background()
	h.code.Store(200)
	h.m.checkInbox(ctx, h.s)
	h.code.Store(404)
	h.m.checkInbox(ctx, h.s)
	w, _ := h.watch.getInboxWatch(ctx, "orph")
	if w.orphanSince == 0 {
		t.Fatal("deveria ficar órfã")
	}
	return w
}

// Recuperação começada (nova inbox) mas QR nunca escaneado: após 24 h apaga igual.
func TestOrphanRecoveryWithoutRelinkStillPurges(t *testing.T) {
	h := newOrphanHarness(t)
	ctx := context.Background()
	w := h.orphanize(t)
	cfg := h.s.getChatwoot()
	cfg.InboxID = 10
	h.s.setChatwoot(cfg)
	h.watch.w["orph"] = inboxWatch{key: inboxWatchKey(cfg), seen: false, orphanSince: w.orphanSince}
	h.code.Store(200)
	h.m.checkInbox(ctx, h.s) // a inbox nova existe: vista, mas segue órfã
	if got, _ := h.watch.getInboxWatch(ctx, "orph"); !got.seen || got.orphanSince == 0 {
		t.Fatalf("inbox nova vista, mas o prazo continua: %+v", got)
	}
	h.watch.w["orph"] = inboxWatch{key: inboxWatchKey(cfg), seen: true, orphanSince: time.Now().Add(-25 * time.Hour).UnixMilli()}
	h.m.purgeOrphans(ctx)
	if len(h.deletes) != 1 {
		t.Fatalf("sem vincular de novo, apaga após 24 h: %v", h.deletes)
	}
}

// Número vinculado de novo (QR): sai da espera e não é apagado.
func TestOrphanRelinkedIsRecovered(t *testing.T) {
	h := newOrphanHarness(t)
	ctx := context.Background()
	h.s.mgr = h.m
	h.orphanize(t)
	h.s.orphanRelinked(ctx)
	if got, _ := h.watch.getInboxWatch(ctx, "orph"); got.orphanSince != 0 {
		t.Fatal("ao vincular de novo, deixa de ser órfã")
	}
	// marca antiga que ficou (ex.: reinício no meio) + número vinculado: não apaga
	w, _ := h.watch.getInboxWatch(ctx, "orph")
	h.watch.w["orph"] = inboxWatch{key: w.key, seen: true, orphanSince: time.Now().Add(-25 * time.Hour).UnixMilli()}
	h.linked.Store(true)
	h.m.purgeOrphans(ctx)
	if len(h.deletes) != 0 {
		t.Fatal("sessão vinculada nunca é apagada")
	}
	if got, _ := h.watch.getInboxWatch(ctx, "orph"); got.orphanSince != 0 {
		t.Fatal("limpa a marca de órfã")
	}
}
