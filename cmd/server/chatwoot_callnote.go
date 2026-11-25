package main

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Nota de chamada no Chatwoot.
//
// Antes só ficava registro quando havia GRAVAÇÃO (≥3 s de áudio): chamadas
// perdidas, recusadas, sem resposta ou muito curtas não deixavam rastro. Agora
// toda chamada encerrada deixa uma nota privada na conversa do contato com a
// direção, o resultado e a duração. A gravação (quando há) continua chegando como
// outra nota, com o áudio.

// callNoteText monta o texto da nota (em espanhol: é o que o atendente lê).
func callNoteText(rec CallRecord) string {
	incoming := rec.Direction == "inbound"
	head := "📞 Llamada saliente"
	if incoming {
		head = "📞 Llamada entrante"
	}
	if rec.ConnectedAt != nil {
		end := time.Now().UnixMilli()
		if rec.EndedAt != nil {
			end = *rec.EndedAt
		}
		secs := int((end - *rec.ConnectedAt) / 1000)
		if secs < 0 {
			secs = 0
		}
		return head + " · contestada · " + fmtDuration(secs)
	}
	switch rec.EndReason {
	case "declined":
		return head + " · rechazada"
	case "busy":
		return head + " · ocupado"
	}
	if incoming {
		return head + " · perdida"
	}
	return head + " · sin respuesta"
}

// chatwootCallNote deixa a nota da chamada na conversa do contato.
func (s *Session) chatwootCallNote(rec CallRecord) {
	cfg := s.getChatwoot()
	if !cfg.valid() {
		return
	}
	phone := digitsOnly(rec.Peer)
	if phone == "" {
		return // peer desconhecido (LID não resolvido): não há a quem anexar
	}
	chatID := phone + "@" + types.DefaultUserServer
	contactID, sourceID, err := cfg.ensureContact(chatID, phone, phone, "")
	if err != nil {
		s.log.Error("nota de chamada: ensure contact falhou", "err", err)
		return
	}
	convID, err := cfg.ensureConversation(contactID, sourceID)
	if err != nil {
		s.log.Error("nota de chamada: ensure conversation falhou", "err", err)
		return
	}
	if err := cfg.postText(convID, callNoteText(rec), cwPrivate, "", "", 0, nil); err != nil {
		s.log.Error("nota de chamada: postar falhou", "err", err)
	}
}
