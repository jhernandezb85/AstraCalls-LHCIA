package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// fakeCW simula as rotas do Chatwoot usadas pelos recibos e registra as chamadas.
type fakeCW struct {
	mu    sync.Mutex
	calls []string // "METHOD path body"
}

func (f *fakeCW) log(r *http.Request) string {
	b, _ := io.ReadAll(r.Body)
	line := r.Method + " " + r.URL.Path
	if len(b) > 0 {
		line += " " + string(b)
	}
	f.mu.Lock()
	f.calls = append(f.calls, line)
	f.mu.Unlock()
	return line
}

func (f *fakeCW) has(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func (f *fakeCW) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func newFakeCW(t *testing.T) (*fakeCW, *httptest.Server) {
	f := &fakeCW{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.log(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/accounts/1/contacts/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"payload": []any{
				// resultado "parecido" (substring) que NÃO é o contato: deve ser ignorado
				map[string]any{"id": 90, "phone_number": "+5215512345678999", "contact_inboxes": []any{
					map[string]any{"source_id": "wrong", "inbox": map[string]any{"id": 7}},
				}},
				map[string]any{"id": 42, "phone_number": "+5215512345678", "identifier": "5215512345678@s.whatsapp.net",
					"contact_inboxes": []any{map[string]any{"source_id": "src-42", "inbox": map[string]any{"id": 7}}}},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/accounts/1/contacts/42/conversations":
			_ = json.NewEncoder(w).Encode(map[string]any{"payload": []any{
				map[string]any{"id": 300, "inbox_id": 8, "status": "open"},     // outra inbox
				map[string]any{"id": 301, "inbox_id": 7, "status": "resolved"}, // resolvida: não reabre
				map[string]any{"id": 302, "inbox_id": 7, "status": "pending"},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/accounts/1/inboxes/7":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7, "inbox_identifier": "IDENT7"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 555})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condição não atingida a tempo")
}

func TestReceiptStatusMapping(t *testing.T) {
	cases := map[types.ReceiptType]string{
		types.ReceiptTypeDelivered:  "delivered",
		types.ReceiptTypeRead:       "read",
		types.ReceiptTypePlayed:     "read",
		types.ReceiptTypeReadSelf:   "",
		types.ReceiptTypePlayedSelf: "",
		types.ReceiptTypeRetry:      "",
	}
	for in, want := range cases {
		if got := receiptStatus(in); got != want {
			t.Errorf("receiptStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJobDirection(t *testing.T) {
	if (cwJob{Outgoing: true}).dir() != cwOutgoing {
		t.Error("Outgoing deve postar como saída")
	}
	if (cwJob{Private: true}).dir() != cwPrivate {
		t.Error("Private deve postar como nota privada")
	}
	if (cwJob{}).dir() != cwIncoming {
		t.Error("padrão deve ser entrada")
	}
}

func TestPostTextIDReturnsMessageID(t *testing.T) {
	_, srv := newFakeCW(t)
	cfg := ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "tok", InboxID: 7}
	id, err := cfg.postTextID(302, "oi", cwOutgoing, "WAID1", "", 0, nil)
	if err != nil || id != 555 {
		t.Fatalf("postTextID = %d, %v; want 555, nil", id, err)
	}
}

func TestFindOpenConversationDoesNotCreate(t *testing.T) {
	f, srv := newFakeCW(t)
	cfg := ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "tok", InboxID: 7}
	sid, conv := cfg.findOpenConversation("5215512345678@s.whatsapp.net", "5215512345678", "")
	if sid != "src-42" || conv != 302 {
		t.Fatalf("got (%q, %d), want (src-42, 302)", sid, conv)
	}
	if f.has("POST ") {
		t.Fatal("não deve criar contato/conversa ao procurar")
	}
}

func TestReceiptsUpdateStatusAndLastSeen(t *testing.T) {
	f, srv := newFakeCW(t)
	s := &Session{id: "sessA"}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "tok", InboxID: 7, ReadReceipts: true})
	s.rememberCWMessage("WAMSG1", 302, 900)

	contact := types.NewJID("5215512345678", types.DefaultUserServer)
	src := types.MessageSource{Chat: contact, Sender: contact}

	// entregue -> PATCH status delivered (sem update_last_seen)
	s.chatwootOnReceipt(&events.Receipt{MessageSource: src, MessageIDs: []types.MessageID{"WAMSG1"}, Type: types.ReceiptTypeDelivered})
	if !f.has(`PATCH /api/v1/accounts/1/conversations/302/messages/900 {"status":"delivered"}`) {
		t.Fatalf("esperava PATCH delivered; chamadas: %v", f.calls)
	}
	if f.has("POST /public/") {
		t.Fatal("entregue não deve marcar a conversa como vista")
	}

	// lida -> PATCH read + update_last_seen pela API pública (identifier buscado na API)
	s.chatwootOnReceipt(&events.Receipt{MessageSource: src, MessageIDs: []types.MessageID{"WAMSG1"}, Type: types.ReceiptTypeRead})
	if !f.has(`PATCH /api/v1/accounts/1/conversations/302/messages/900 {"status":"read"}`) {
		t.Fatalf("esperava PATCH read; chamadas: %v", f.calls)
	}
	if !f.has("POST /public/api/v1/inboxes/IDENT7/contacts/src-42/conversations/302/update_last_seen") {
		t.Fatalf("esperava update_last_seen; chamadas: %v", f.calls)
	}

	// outra "lida" logo em seguida: limitada (não repete o update_last_seen)
	s.chatwootOnReceipt(&events.Receipt{MessageSource: src, MessageIDs: []types.MessageID{"OUTRA"}, Type: types.ReceiptTypeRead})
	if n := f.count("POST /public/"); n != 1 {
		t.Fatalf("update_last_seen deveria ser limitado; chamadas = %d", n)
	}
}

func TestReceiptsIgnoredWhenDisabledOrNotApplicable(t *testing.T) {
	f, srv := newFakeCW(t)
	contact := types.NewJID("5215512345678", types.DefaultUserServer)
	group := types.NewJID("120363000000000000", types.GroupServer)

	off := &Session{id: "off"}
	off.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "tok", InboxID: 7}) // read_receipts desligado
	off.rememberCWMessage("M1", 302, 900)
	off.chatwootOnReceipt(&events.Receipt{MessageSource: types.MessageSource{Chat: contact}, MessageIDs: []types.MessageID{"M1"}, Type: types.ReceiptTypeRead})

	on := &Session{id: "on"}
	on.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "tok", InboxID: 7, ReadReceipts: true})
	on.rememberCWMessage("M2", 302, 901)
	// recibo das próprias telas da conta, e recibo em grupo: ignorados
	on.chatwootOnReceipt(&events.Receipt{MessageSource: types.MessageSource{Chat: contact, IsFromMe: true}, MessageIDs: []types.MessageID{"M2"}, Type: types.ReceiptTypeRead})
	on.chatwootOnReceipt(&events.Receipt{MessageSource: types.MessageSource{Chat: group, IsGroup: true}, MessageIDs: []types.MessageID{"M2"}, Type: types.ReceiptTypeRead})
	on.chatwootOnReceipt(&events.Receipt{MessageSource: types.MessageSource{Chat: contact}, MessageIDs: []types.MessageID{"M2"}, Type: types.ReceiptTypeReadSelf})

	time.Sleep(50 * time.Millisecond)
	if len(f.calls) != 0 {
		t.Fatalf("nenhuma chamada esperada; houve: %v", f.calls)
	}
}

func TestMessageRefsAreScopedAndExpire(t *testing.T) {
	a, b := &Session{id: "A"}, &Session{id: "B"}
	a.rememberCWMessage("X", 1, 2)
	if _, ok := b.lookupCWMessage("X"); ok {
		t.Fatal("o mapa deve ser por sessão")
	}
	if ref, ok := a.lookupCWMessage("X"); !ok || ref.conv != 1 || ref.msg != 2 {
		t.Fatal("deveria achar a referência")
	}
	a.rememberCWMessage("", 1, 2) // incompleto: ignorado
	a.rememberCWMessage("Y", 0, 2)
	if _, ok := a.lookupCWMessage("Y"); ok {
		t.Fatal("referência incompleta não deve ser guardada")
	}
	cwMsgRefs.Store("A|OLD", cwMsgRef{conv: 1, msg: 2, at: time.Now().Add(-cwMsgRefTTL - time.Minute)})
	if _, ok := a.lookupCWMessage("OLD"); ok {
		t.Fatal("referência vencida não deve valer")
	}
	pruneCWMsgRefs(time.Now())
	if _, ok := cwMsgRefs.Load("A|OLD"); ok {
		t.Fatal("prune deveria apagar vencidos")
	}
}

func TestSetMessageStatusFailedCarriesError(t *testing.T) {
	f, srv := newFakeCW(t)
	cfg := ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "tok", InboxID: 7}
	cfg.setMessageStatus(302, 900, "failed", "sessão caída")
	waitFor(t, func() bool {
		return f.has(`PATCH /api/v1/accounts/1/conversations/302/messages/900 {"external_error":"sessão caída","status":"failed"}`)
	})
}

func TestEchoWithSourceIDIsNotRelayed(t *testing.T) {
	// mensagem do aparelho postada como SAÍDA normal volta no webhook com source_id:
	// não pode ser reenviada ao WhatsApp.
	body := map[string]any{"event": "message_created", "message_type": "outgoing", "private": false, "source_id": "WAID123"}
	if shouldRelayWebhook(body) {
		t.Fatal("eco com source_id não deve ser reenviado")
	}
}

func TestMirrorOutgoingOnlyForDevice(t *testing.T) {
	s := &Session{id: "m"}
	on := ChatwootConfig{MirrorAsOutgoing: true}
	if !s.mirrorOutgoing(on, "DEVICE1") {
		t.Fatal("mensagem do aparelho deve virar saída normal")
	}
	s.markSelfSent("API1", selfSentAPI)
	if s.mirrorOutgoing(on, "API1") {
		t.Fatal("mensagem enviada pela API deve continuar como nota privada")
	}
	if s.mirrorOutgoing(ChatwootConfig{}, "DEVICE2") {
		t.Fatal("desligado por padrão")
	}
}
