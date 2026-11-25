package main

import (
	"fmt"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func inMsg(id string, fromMe bool) *events.Message {
	u := types.NewJID("51977415846", types.DefaultUserServer)
	return &events.Message{Info: types.MessageInfo{
		ID: id, Timestamp: time.Now(),
		MessageSource: types.MessageSource{Chat: u, Sender: u, IsFromMe: fromMe},
	}}
}

func TestTrackUnread(t *testing.T) {
	s := &Session{id: "rr1"}
	s.trackUnread(inMsg("A", false))
	s.trackUnread(inMsg("B", false))
	u := takeUnread("rr1", "51977415846")
	if u == nil || len(u.ids) != 2 || u.ids[0] != "A" || u.ids[1] != "B" {
		t.Fatalf("esperava A,B: %+v", u)
	}
	if takeUnread("rr1", "51977415846") != nil {
		t.Fatal("takeUnread deve esvaziar")
	}

	// resposta pelo celular limpa a lista
	s.trackUnread(inMsg("C", false))
	s.trackUnread(inMsg("D", true))
	if takeUnread("rr1", "51977415846") != nil {
		t.Fatal("mensagem do próprio aparelho deve limpar")
	}

	// teto por contato
	for i := 0; i < unreadMaxPerChat+5; i++ {
		s.trackUnread(inMsg(fmt.Sprintf("M%d", i), false))
	}
	if u := takeUnread("rr1", "51977415846"); len(u.ids) != unreadMaxPerChat || u.ids[0] != "M5" {
		t.Fatalf("teto: %d %v", len(u.ids), u.ids[0])
	}

	// com read_messages ligado não precisa guardar
	s2 := &Session{id: "rr2"}
	s2.setChatwoot(ChatwootConfig{ReadMessages: true})
	s2.trackUnread(inMsg("X", false))
	if takeUnread("rr2", "51977415846") != nil {
		t.Fatal("read_messages já marca tudo como lido")
	}

	// grupos ficam de fora
	g := inMsg("G", false)
	g.Info.Chat = types.NewJID("123-456", types.GroupServer)
	s.trackUnread(g)
	if takeUnread("rr1", "123-456") != nil {
		t.Fatal("só 1:1")
	}
}
