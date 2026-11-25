package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"google.golang.org/protobuf/proto"
)

func histChunk(order uint32, typ waHistorySync.HistorySync_HistorySyncType) *waHistorySync.HistorySync {
	return &waHistorySync.HistorySync{
		SyncType:      typ.Enum(),
		ChunkOrder:    proto.Uint32(order),
		Conversations: []*waHistorySync.Conversation{{ID: proto.String("51900000000@s.whatsapp.net")}},
	}
}

// gateHarness: Chatwoot falso que registra os avisos e um importChunk falso que
// registra a ordem dos chunks importados.
func gateHarness(t *testing.T, id string) (*Session, func() []uint32, func() string) {
	var mu sync.Mutex
	var order []uint32
	var bodies strings.Builder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies.Write(b)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	oldChunk, oldWait, oldIdle := importChunk, importAgendaWait, importIdleDone
	importChunk = func(_ *Session, d *waHistorySync.HistorySync) (int, int) {
		mu.Lock()
		order = append(order, d.GetChunkOrder())
		mu.Unlock()
		return 1, 10
	}
	importAgendaWait, importIdleDone = 300*time.Millisecond, 150*time.Millisecond
	t.Cleanup(func() { importChunk, importAgendaWait, importIdleDone = oldChunk, oldWait, oldIdle })

	s := &Session{id: id, log: slog.Default()}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "t", InboxID: 7, ImportHistory: true})
	s.resetImportGate()
	got := func() []uint32 { mu.Lock(); defer mu.Unlock(); return append([]uint32(nil), order...) }
	text := func() string { mu.Lock(); defer mu.Unlock(); return bodies.String() }
	return s, got, text
}

func gateWait(t *testing.T, cond func() bool, what string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout: " + what)
}

func TestImportWaitsForAgenda(t *testing.T) {
	s, got, text := gateHarness(t, "gate1")
	s.historyChunk(histChunk(1, waHistorySync.HistorySync_INITIAL_BOOTSTRAP))
	s.historyChunk(histChunk(2, waHistorySync.HistorySync_RECENT))
	gateWait(t, func() bool { return strings.Contains(text(), "ya comenzó") }, "aviso de início")
	if len(got()) != 0 {
		t.Fatal("não deve importar antes da agenda")
	}
	s.releaseImport() // agenda chegou
	gateWait(t, func() bool { return len(got()) == 2 }, "importar a fila")
	if o := got(); o[0] != 1 || o[1] != 2 {
		t.Fatalf("ordem de chegada: %v", o)
	}
	// depois da agenda, um chunk novo vai direto
	s.historyChunk(histChunk(3, waHistorySync.HistorySync_RECENT))
	gateWait(t, func() bool { return len(got()) == 3 }, "chunk após a agenda")
	gateWait(t, func() bool { return strings.Contains(text(), "3 conversaciones y 30 mensajes") }, "aviso de fim com total")
	if strings.Count(text(), "ya comenzó") != 1 {
		t.Fatal("o aviso de início sai uma vez só")
	}
}

func TestImportAgendaTimeout(t *testing.T) {
	s, got, _ := gateHarness(t, "gate2")
	s.historyChunk(histChunk(1, waHistorySync.HistorySync_INITIAL_BOOTSTRAP))
	gateWait(t, func() bool { return len(got()) == 1 }, "importar mesmo sem agenda após o tempo máximo")
}

func TestImportOnDemandSkipsGate(t *testing.T) {
	s, got, text := gateHarness(t, "gate3")
	s.historyChunk(histChunk(9, waHistorySync.HistorySync_ON_DEMAND))
	if o := got(); len(o) != 1 || o[0] != 9 {
		t.Fatalf("sob demanda importa na hora: %v", o)
	}
	if strings.Contains(text(), "ya comenzó") {
		t.Fatal("sob demanda não anuncia início de importação ao parear")
	}
}

func TestImportGateIgnoresWhenDisabled(t *testing.T) {
	s, got, text := gateHarness(t, "gate4")
	cfg := s.getChatwoot()
	cfg.ImportHistory = false
	s.setChatwoot(cfg)
	s.historyChunk(histChunk(1, waHistorySync.HistorySync_INITIAL_BOOTSTRAP))
	time.Sleep(400 * time.Millisecond)
	if len(got()) != 0 || text() != "" {
		t.Fatal("sem import_history não faz nada")
	}
}
