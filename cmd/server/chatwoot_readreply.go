package main

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// "Lido ao responder".
//
// O WhatsApp só mostra os tiques azuis ao contato quando alguém marca as mensagens
// como lidas. O Chatwoot não avisa quando um atendente abre a conversa, então o
// melhor sinal é a RESPOSTA: quem respondeu, leu. Guardamos as mensagens recebidas
// ainda não lidas de cada contato (1:1) e, quando sai uma resposta pelo Chatwoot,
// marcamos todas como lidas. Com read_messages ligado (lê tudo ao chegar) isto não
// é necessário. Se a conta responder pelo celular, o próprio aparelho já marca
// como lidas — e a lista é descartada.

const unreadMaxPerChat = 30

type unreadChat struct {
	ids    []types.MessageID
	chat   types.JID
	sender types.JID
	last   time.Time
}

var (
	unreadMu     sync.Mutex
	unreadByChat = map[string]*unreadChat{} // "<sessão>|<telefone>"
)

func unreadKey(sessionID, phone string) string { return sessionID + "|" + phone }

// trackUnread anota uma mensagem recebida 1:1 que ainda não foi lida; uma
// mensagem enviada pela própria conta (celular) limpa a lista daquele contato.
func (s *Session) trackUnread(evt *events.Message) {
	if evt == nil || s.readMessagesEnabled() {
		return
	}
	chat := evt.Info.Chat
	if chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer {
		return
	}
	phone, ok := s.resolvedPhone(chat)
	if !ok {
		return
	}
	key := unreadKey(s.id, phone)
	unreadMu.Lock()
	defer unreadMu.Unlock()
	if evt.Info.IsFromMe {
		delete(unreadByChat, key)
		return
	}
	u := unreadByChat[key]
	if u == nil {
		u = &unreadChat{}
		unreadByChat[key] = u
	}
	u.ids = append(u.ids, evt.Info.ID)
	if len(u.ids) > unreadMaxPerChat {
		u.ids = u.ids[len(u.ids)-unreadMaxPerChat:]
	}
	u.chat, u.sender = chat, evt.Info.Sender
	if evt.Info.Timestamp.After(u.last) {
		u.last = evt.Info.Timestamp
	}
}

// takeUnread devolve e esquece as mensagens não lidas do contato.
func takeUnread(sessionID, phone string) *unreadChat {
	unreadMu.Lock()
	defer unreadMu.Unlock()
	key := unreadKey(sessionID, phone)
	u := unreadByChat[key]
	delete(unreadByChat, key)
	return u
}

// markReadOnReply marca como lidas as mensagens do contato depois que uma
// resposta saiu pelo Chatwoot (só conversas 1:1).
func (s *Session) markReadOnReply(to types.JID) {
	if to.Server != types.DefaultUserServer || s.client == nil {
		return
	}
	u := takeUnread(s.id, to.User)
	if u == nil || len(u.ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.client.MarkRead(ctx, u.ids, time.Now(), u.chat, u.sender); err != nil {
		s.log.Debug("lido ao responder: mark read falhou", "err", err)
	}
}
