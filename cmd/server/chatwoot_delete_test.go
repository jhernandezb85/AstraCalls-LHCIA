package main

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func revokeEvt(origID string, fromMe bool) *events.Message {
	u := types.NewJID("5215512345678", types.DefaultUserServer)
	return &events.Message{
		Info: types.MessageInfo{ID: "REV-" + origID, MessageSource: types.MessageSource{Chat: u, Sender: u, IsFromMe: fromMe}},
		Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
			Key:  &waCommon.MessageKey{ID: proto.String(origID)},
		}},
	}
}

func TestContactDeleteDefaults(t *testing.T) {
	if (ChatwootConfig{}).contactDelete() != contactDeleteNote {
		t.Fatal("padrão: conservar com nota")
	}
	for _, m := range []string{"", "note", "delete"} {
		if !validContactDelete(m) {
			t.Errorf("%q deveria ser válido", m)
		}
	}
	if validContactDelete("x") {
		t.Error("modo desconhecido deve ser recusado")
	}
	if publicChatwootCfg(ChatwootConfig{}).ContactDelete != "note" {
		t.Error("a resposta mostra o valor efetivo")
	}
}

func deleteBody(sourceID string) map[string]any {
	return map[string]any{
		"event": "message_updated", "message_type": "outgoing", "private": false, "id": 40,
		"source_id": sourceID, "content_attributes": map[string]any{"deleted": true},
		"created_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"conversation": map[string]any{"id": 302},
	}
}

func TestIsDeleteCandidate(t *testing.T) {
	if !isDeleteCandidate(deleteBody("WA40")) {
		t.Fatal("saída apagada com source_id deve ir ao WhatsApp")
	}
	for name, mut := range map[string]func(map[string]any){
		"não apagada": func(b map[string]any) { b["content_attributes"] = map[string]any{} },
		"entrada":     func(b map[string]any) { b["message_type"] = "incoming" },
		"nota":        func(b map[string]any) { b["private"] = true },
		"sem envio":   func(b map[string]any) { b["source_id"] = "" },
		"derivada":    func(b map[string]any) { b["source_id"] = "WA1:edit:123" },
		"criação":     func(b map[string]any) { b["event"] = "message_created" },
	} {
		b := deleteBody("WA40")
		mut(b)
		if isDeleteCandidate(b) {
			t.Errorf("%s não deve apagar no WhatsApp", name)
		}
	}
	if isRetryCandidate(deleteBody("")) {
		t.Fatal("apagada nunca é Reenviar")
	}
}

func TestRevokeDeletesInChatwoot(t *testing.T) {
	f, srv := newFakeCWState(t)
	s := &Session{id: "rev"}
	s.setChatwoot(ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "user", InboxID: 7, ContactDelete: "delete"})

	// a conta apagou no celular -> apaga no Chatwoot
	s.rememberCWMessage("OWN1", 302, 901)
	s.chatwootOnRevoke(revokeEvt("OWN1", true))
	if !f.has("DELETE /api/v1/accounts/1/conversations/302/messages/901") {
		t.Fatalf("deveria apagar no Chatwoot; chamadas: %v", f.calls)
	}
	// o eco (message_updated deleted) não volta ao WhatsApp
	if !s.takeDeletedByUs(901) {
		t.Fatal("apagamento nosso deve ser reconhecido no webhook")
	}
	// o contato apagou, modo delete -> apaga no Chatwoot
	s.rememberCWMessage("IN1", 302, 902)
	s.chatwootOnRevoke(revokeEvt("IN1", false))
	if !f.has("DELETE /api/v1/accounts/1/conversations/302/messages/902") {
		t.Fatal("modo delete apaga a mensagem do contato")
	}
	// a conta apagou algo que não está no Chatwoot -> nada
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
	s.chatwootOnRevoke(revokeEvt("DESCONHECIDA", true))
	if len(f.calls) != 0 {
		t.Fatalf("mensagem desconhecida da conta não gera chamadas: %v", f.calls)
	}
}

func TestIncomingPostsAreRemembered(t *testing.T) {
	_, srv := newFakeCWState(t)
	s := &Session{id: "inmem"}
	cfg := ChatwootConfig{URL: srv.URL, AccountID: 1, AccountToken: "user", InboxID: 7}
	s.afterJobPosted(cfg, cwJob{SourceID: "IN9"}, 302, 909)
	if _, ok := s.lookupCWMessage("IN9"); !ok {
		t.Fatal("mensagem recebida fica registrada (para apagar depois)")
	}
	s.afterJobPosted(cfg, cwJob{Private: true, SourceID: "NOTE9"}, 302, 910)
	if _, ok := s.lookupCWMessage("NOTE9"); ok {
		t.Fatal("nota privada não é registrada")
	}
}
