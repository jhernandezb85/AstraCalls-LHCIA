package main

import (
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

// Nomes de contato vindos da agenda do celular.
//
// Ao parear, o WhatsApp manda o histórico ANTES da agenda: a importação cria os
// contatos no Chatwoot só com o número como nome. Quando cada contato da agenda
// chega (events.Contact), procuramos o contato no Chatwoot pelo telefone e, se o
// nome ainda for só o número, trocamos pelo nome da agenda. Nunca cria contatos
// e nunca troca um nome de verdade (pode ter sido digitado à mão).

// cwNameMu serializa as correções (a agenda chega em rajadas de centenas de
// contatos) e cwNameGap espaça as chamadas para não estourar o rate-limit.
var (
	cwNameMu  sync.Mutex
	cwNameGap = 120 * time.Millisecond
)

func (s *Session) chatwootContactName(evt *events.Contact) {
	if evt == nil || evt.Action == nil {
		return
	}
	cfg := s.getChatwoot()
	if !cfg.valid() {
		return
	}
	name := strings.TrimSpace(firstNonEmptyOf(evt.Action.GetFullName(), evt.Action.GetFirstName()))
	if name == "" {
		return
	}
	phone, ok := s.resolvedPhone(evt.JID)
	if !ok || isPlaceholderName(name, phone) {
		return
	}
	cwNameMu.Lock()
	defer cwNameMu.Unlock()
	cfg.fixContactName(phone, name)
	time.Sleep(cwNameGap)
}

// fixContactName troca o nome do contato 1:1 com esse telefone se ele ainda for
// só o número. Devolve quantos contatos foram corrigidos.
func (c ChatwootConfig) fixContactName(phone, name string) int {
	res, code, err := c.searchContacts(phone)
	if err != nil || code != 200 {
		return 0
	}
	fixed := 0
	for _, it := range asList(res["payload"]) {
		m := asMap(it)
		if isGroupChatID(asStr(m["identifier"])) {
			continue
		}
		p := digitsOnly(asStr(m["phone_number"]))
		if p != phone && p != brNinthDigitVariant(phone) {
			continue
		}
		if ex := asStr(m["name"]); ex != "" && isPlaceholderName(ex, phone) && asInt(m["id"]) != 0 {
			id := asInt(m["id"])
			c.updateContactName(id, name)
			fixed++
		}
	}
	return fixed
}
