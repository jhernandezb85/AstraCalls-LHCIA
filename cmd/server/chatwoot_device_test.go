package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func boolPtr(b bool) *bool { return &b }

func textEvt(id, text string) *events.Message {
	u := types.NewJID("5215512345678", types.DefaultUserServer)
	return &events.Message{
		Info:    types.MessageInfo{ID: id, MessageSource: types.MessageSource{Chat: u, Sender: u, IsFromMe: true}},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

func TestChatwootDefaults(t *testing.T) {
	var old ChatwootConfig
	// config gravada antes destes campos existirem
	if err := json.Unmarshal([]byte(`{"url":"https://cw","account_id":1,"account_token":"t","inbox_id":2}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.deviceMode() != deviceModeOpen || !old.readReceipts() {
		t.Fatal("padrão: aparelho como mensagem que abre a conversa e recibos ligados")
	}
	if (ChatwootConfig{ReadReceipts: boolPtr(false)}).readReceipts() {
		t.Fatal("desligar explicitamente deve valer")
	}
	for _, m := range []string{"", "open", "keep", "note"} {
		if !validDeviceMode(m) {
			t.Errorf("%q deveria ser válido", m)
		}
	}
	if validDeviceMode("outro") {
		t.Error("modo desconhecido deve ser recusado")
	}
	pub := publicChatwootCfg(ChatwootConfig{AccountToken: "secreto", DeviceBotToken: "bot"})
	if pub.AccountToken != "" || pub.DeviceBotToken != "" {
		t.Fatal("tokens nunca são devolvidos")
	}
	if pub.DeviceMode != "open" || pub.ReadReceipts == nil || !*pub.ReadReceipts {
		t.Fatal("a resposta mostra os valores efetivos")
	}
}

func TestOwnMirrorJobModes(t *testing.T) {
	s := &Session{id: "dev"}
	cases := []struct {
		mode               string
		outgoing, private  bool
		openConv, isDevice bool
	}{
		{"", true, false, true, true},
		{"open", true, false, true, true},
		{"keep", true, false, false, true},
		{"note", false, true, false, false},
	}
	for _, c := range cases {
		j := s.ownMirrorJob(ChatwootConfig{DeviceMode: c.mode}, textEvt("D-"+c.mode, "hola"), "")
		if j.Outgoing != c.outgoing || j.Private != c.private || j.OpenConv != c.openConv || j.Device != c.isDevice {
			t.Errorf("modo %q: job = %+v", c.mode, j)
		}
		if j.Prefix != defaultDeviceTitle+"\n" {
			t.Errorf("modo %q: título = %q", c.mode, j.Prefix)
		}
	}
	// título configurável e prefixo de autor em grupo
	j := s.ownMirrorJob(ChatwootConfig{DeviceTitle: "📱 Enviado desde el celular"}, textEvt("D2", "x"), "*Ana (você)*:\n")
	if j.Prefix != "📱 Enviado desde el celular\n*Ana (você)*:\n" {
		t.Fatalf("prefixo = %q", j.Prefix)
	}
	// o que saiu pela nossa API (mirror_api) segue como nota privada
	s.markSelfSent("API1", selfSentAPI)
	if j := s.ownMirrorJob(ChatwootConfig{}, textEvt("API1", "x"), ""); !j.Private || j.Outgoing || j.OpenConv {
		t.Fatalf("API deve ser nota privada: %+v", j)
	}
}

func TestBurstFromChatwootIsNotDevice(t *testing.T) {
	// n8n/agente manda várias fotos seguidas pelo Chatwoot: os ecos não podem ser
	// tratados como "escritos no celular" (duplicariam e abririam a conversa).
	s := &Session{id: "burst"}
	cfg := ChatwootConfig{}
	for _, id := range []string{"F1", "F2", "F3", "F4"} {
		s.markSelfSent(id, selfSentChatwoot)
	}
	for _, id := range []string{"F1", "F2", "F3", "F4"} {
		if s.shouldMirrorOwn(id, cfg) {
			t.Fatalf("%s veio do Chatwoot e não deve ser espelhada", id)
		}
	}
	if !s.shouldMirrorOwn("CEL1", cfg) {
		t.Fatal("o que não saiu por nós veio do celular")
	}
}

func TestPostAsJobUsesBotAndFallsBack(t *testing.T) {
	s := &Session{id: "bot"}
	cfg := ChatwootConfig{AccountToken: "user", DeviceBotToken: "bot"}
	var used []string
	post := func(fail map[string]error) func(ChatwootConfig) (int, error) {
		return func(c ChatwootConfig) (int, error) {
			used = append(used, c.AccountToken)
			return 1, fail[c.AccountToken]
		}
	}
	device := cwJob{Outgoing: true, Device: true}
	if _, err := s.postAsJob(cfg, device, post(nil)); err != nil || strings.Join(used, ",") != "bot" {
		t.Fatalf("deveria postar com o bot: %v %v", used, err)
	}
	used = nil
	if _, err := s.postAsJob(cfg, device, post(map[string]error{"bot": errors.New("post message http 401")})); err != nil || strings.Join(used, ",") != "bot,user" {
		t.Fatalf("bot recusado deve cair para a conta: %v %v", used, err)
	}
	used = nil
	if _, err := s.postAsJob(cfg, device, post(map[string]error{"bot": errors.New("post message http 500")})); err == nil || strings.Join(used, ",") != "bot" {
		t.Fatalf("erro que não é de acesso não troca de token (a fila reentrega): %v %v", used, err)
	}
	used = nil
	_, _ = s.postAsJob(cfg, cwJob{Outgoing: true}, post(nil)) // não é do aparelho
	_, _ = s.postAsJob(ChatwootConfig{AccountToken: "user"}, device, post(nil))
	if strings.Join(used, ",") != "user,user" {
		t.Fatalf("sem bot ou fora do aparelho usa a conta: %v", used)
	}
}

// fakeCWState simula conversas/mensagens para os testes de status e Reenviar.
type fakeCWState struct {
	mu       sync.Mutex
	calls    []string
	tokens   []string
	messages map[int]map[string]any // id -> {status, source_id}
	denyBot  bool                   // responde 401 ao token "bot"
}

func newFakeCWState(t *testing.T) (*fakeCWState, *httptest.Server) {
	f := &fakeCWState{messages: map[int]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		f.tokens = append(f.tokens, r.Header.Get("api_access_token"))
		deny := f.denyBot && r.Header.Get("api_access_token") == "bot"
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if deny {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/conversations/302/messages") {
			f.mu.Lock()
			var list []any
			for id, m := range f.messages {
				list = append(list, map[string]any{"id": id, "status": m["status"], "source_id": m["source_id"]})
			}
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"payload": list})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeCWState) has(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, call) {
			return true
		}
	}
	return false
}

func TestAfterJobPostedOpensConversationOnlyInOpenMode(t *testing.T) {
	f, srv := newFakeCWState(t)
	s := &Session{id: "open"}
	cfg := ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "user", InboxID: 7, DeviceBotToken: "bot"}

	s.afterJobPosted(cfg, cwJob{Outgoing: true, Device: true, SourceID: "K1"}, 302, 700) // keep
	if f.has("POST /api/v1/accounts/1/conversations/302/toggle_status") {
		t.Fatal("keep não muda o status")
	}
	s.afterJobPosted(cfg, cwJob{Outgoing: true, Device: true, OpenConv: true, SourceID: "O1"}, 302, 701)
	if !f.has("POST /api/v1/accounts/1/conversations/302/toggle_status") {
		t.Fatalf("open passa a conversa para aberta; chamadas: %v", f.calls)
	}
	// o status é mudado pelo bot (o aviso diz "reaberta por WhatsApp (celular)")
	if f.tokens[len(f.tokens)-1] != "bot" {
		t.Fatalf("toggle_status deve usar o bot; tokens: %v", f.tokens)
	}
	// bot recusado -> cai para o token da conta
	f.mu.Lock()
	f.tokens = nil
	f.denyBot = true
	f.mu.Unlock()
	s.afterJobPosted(cfg, cwJob{Outgoing: true, Device: true, OpenConv: true, SourceID: "O2"}, 302, 702)
	if strings.Join(f.tokens, ",") != "bot,user" {
		t.Fatalf("sem acesso do bot deve usar a conta; tokens: %v", f.tokens)
	}
	if !s.knownCWMessage(700) || !s.knownCWMessage(701) {
		t.Fatal("mensagens postadas ficam registradas (recibos e proteção do Reenviar)")
	}
}

func retryBody(id int, created time.Time) map[string]any {
	return map[string]any{
		"event": "message_updated", "message_type": "outgoing", "private": false, "id": id,
		"created_at":   created.UTC().Format(time.RFC3339Nano),
		"conversation": map[string]any{"id": 302},
	}
}

func TestIsRetryCandidate(t *testing.T) {
	ok := retryBody(10, time.Now())
	if !isRetryCandidate(ok) {
		t.Fatal("message_updated de saída sem source_id é candidato")
	}
	for name, mut := range map[string]func(map[string]any){
		"criação":    func(b map[string]any) { b["event"] = "message_created" },
		"entrada":    func(b map[string]any) { b["message_type"] = "incoming" },
		"nota":       func(b map[string]any) { b["private"] = true },
		"já enviada": func(b map[string]any) { b["source_id"] = "WA1" },
		"apagada":    func(b map[string]any) { b["content_attributes"] = map[string]any{"deleted": true} },
		"rica":       func(b map[string]any) { b["content_attributes"] = map[string]any{"astracall_rich_sent": true} },
	} {
		b := retryBody(10, time.Now())
		mut(b)
		if isRetryCandidate(b) {
			t.Errorf("%s não é candidato", name)
		}
	}
}

func TestClaimChatwootRetry(t *testing.T) {
	f, srv := newFakeCWState(t)
	s := &Session{id: "retry"}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "user", InboxID: 7})

	f.messages[20] = map[string]any{"status": "failed"}
	if s.claimChatwootRetry(retryBody(20, time.Now())) {
		t.Fatal("ainda falhada (ex.: o próprio Chatwoot marcando falha): não reenvia")
	}
	f.messages[20] = map[string]any{"status": "sent"} // atendente clicou em Reenviar
	if !s.claimChatwootRetry(retryBody(20, time.Now())) {
		t.Fatal("falhou -> enviado sem nunca ter saído: é Reenviar")
	}
	f.messages[21] = map[string]any{"status": "sent", "source_id": "WA21"}
	if s.claimChatwootRetry(retryBody(21, time.Now())) {
		t.Fatal("com source_id já saiu")
	}
	f.messages[22] = map[string]any{"status": "sent"}
	s.rememberCWMessage("WA22", 302, 22)
	if s.claimChatwootRetry(retryBody(22, time.Now())) {
		t.Fatal("já enviada por nós: não duplica")
	}
	f.messages[23] = map[string]any{"status": "sent"}
	if s.claimChatwootRetry(retryBody(23, time.Now().Add(-cwRetryWindow-time.Hour))) {
		t.Fatal("mensagem antiga não é reenviada por qualquer atualização")
	}
}

func TestRelayIsExclusivePerMessage(t *testing.T) {
	s := &Session{id: "lock"}
	if !s.beginRelay(5) {
		t.Fatal("primeira reserva deve passar")
	}
	if s.beginRelay(5) {
		t.Fatal("a mesma mensagem não pode ser enviada em paralelo")
	}
	s.endRelay(5)
	if !s.beginRelay(5) {
		t.Fatal("após terminar, pode de novo")
	}
	s.endRelay(5)
}
