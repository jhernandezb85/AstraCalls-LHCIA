package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Recibos do WhatsApp -> Chatwoot (read_receipts).
//
// Duas camadas, que se complementam:
//  1. status por mensagem: cada mensagem de SAÍDA que o AstraCalls conhece (enviada
//     pelo agente no Chatwoot, ou espelhada do aparelho com mirror_as_outgoing) fica
//     num mapa waMsgID -> (conversa, mensagem) do Chatwoot. Ao chegar o recibo
//     "entregue"/"lida", o status dela é atualizado (PATCH .../messages/{id}, só
//     inboxes API) — são os tiques ✓✓ na timeline.
//  2. last_seen da conversa: no recibo "lida", marca a conversa como vista pelo
//     contato via API pública (update_last_seen). É o que a Evolution API faz, e
//     cobre também mensagens que não estão no mapa (ex.: após reiniciar).
//
// O mapa é em memória com validade de cwMsgRefTTL: os recibos chegam em segundos
// ou horas; perder o mapa num restart só perde o tique "entregue" das mensagens
// antigas (o "lida" segue funcionando pelo last_seen).

const (
	cwMsgRefTTL       = 7 * 24 * time.Hour
	cwMsgRefPruneEach = 500              // a cada N inserções, limpa os vencidos
	cwSeenThrottle    = 5 * time.Second  // no máx. 1 update_last_seen por chat nesse intervalo
	cwIdentCacheTTL   = 30 * time.Minute // cache do inbox_identifier buscado na API
)

type cwMsgRef struct {
	conv, msg int
	at        time.Time
}

var (
	cwMsgRefs      sync.Map // "<sessão>|<waMsgID>" -> cwMsgRef
	cwMsgRefInsert atomic.Int64
	cwLastSeen     sync.Map // "<sessão>|<chat>" -> time.Time
	cwInboxIdents  sync.Map // "<url>|<conta>|<inbox>" -> cwIdent
)

type cwIdent struct {
	ident string
	at    time.Time
}

// rememberCWMessage registra que a mensagem waID do WhatsApp é a msgID da conversa
// convID no Chatwoot. No-op com dados incompletos.
func (s *Session) rememberCWMessage(waID string, convID, msgID int) {
	if waID == "" || convID == 0 || msgID == 0 {
		return
	}
	cwMsgRefs.Store(s.id+"|"+waID, cwMsgRef{conv: convID, msg: msgID, at: time.Now()})
	if cwMsgRefInsert.Add(1)%cwMsgRefPruneEach == 0 {
		pruneCWMsgRefs(time.Now())
	}
}

func (s *Session) lookupCWMessage(waID string) (cwMsgRef, bool) {
	v, ok := cwMsgRefs.Load(s.id + "|" + waID)
	if !ok {
		return cwMsgRef{}, false
	}
	ref := v.(cwMsgRef)
	if time.Since(ref.at) > cwMsgRefTTL {
		cwMsgRefs.Delete(s.id + "|" + waID)
		return cwMsgRef{}, false
	}
	return ref, true
}

func pruneCWMsgRefs(now time.Time) {
	cwMsgRefs.Range(func(k, v any) bool {
		if now.Sub(v.(cwMsgRef).at) > cwMsgRefTTL {
			cwMsgRefs.Delete(k)
		}
		return true
	})
}

// receiptStatus traduz o tipo de recibo do WhatsApp no status do Chatwoot ("" =
// ignorar). Recibos das PRÓPRIAS telas da conta (read-self etc.) não contam.
func receiptStatus(t types.ReceiptType) string {
	switch t {
	case types.ReceiptTypeDelivered:
		return "delivered"
	case types.ReceiptTypeRead, types.ReceiptTypePlayed:
		return "read"
	}
	return ""
}

// chatwootOnReceipt aplica um recibo do WhatsApp no Chatwoot (só com read_receipts,
// só conversas 1:1 — em grupo cada participante manda o seu e não há um "lido").
func (s *Session) chatwootOnReceipt(evt *events.Receipt) {
	cfg := s.getChatwoot()
	if !cfg.valid() || !cfg.ReadReceipts || evt.IsFromMe || evt.IsGroup {
		return
	}
	if evt.Chat.Server != types.DefaultUserServer && evt.Chat.Server != types.HiddenUserServer {
		return
	}
	status := receiptStatus(evt.Type)
	if status == "" {
		return
	}
	for _, id := range evt.MessageIDs {
		if ref, ok := s.lookupCWMessage(id); ok {
			cfg.setMessageStatus(ref.conv, ref.msg, status, "")
		}
	}
	if status == "read" {
		s.chatwootMarkSeen(cfg, evt.Chat, evt.SenderAlt)
	}
}

// setMessageStatus atualiza o status de uma mensagem (sent/delivered/read/failed).
// O Chatwoot só aceita em inboxes API e ignora retrocessos (ex.: read -> delivered).
func (c ChatwootConfig) setMessageStatus(convID, msgID int, status, externalErr string) {
	body := map[string]any{"status": status}
	if externalErr != "" {
		body["external_error"] = externalErr
	}
	_, _, _ = c.req(http.MethodPatch, fmt.Sprintf("/conversations/%d/messages/%d", convID, msgID), body)
}

// chatwootMarkSeen marca a conversa aberta do contato como vista por ele (os tiques
// azuis das mensagens do agente), via API pública do Chatwoot. Best-effort e com
// limite de frequência por chat.
func (s *Session) chatwootMarkSeen(cfg ChatwootConfig, chat, alt types.JID) {
	key := s.id + "|" + chat.ToNonAD().String()
	if v, ok := cwLastSeen.Load(key); ok && time.Since(v.(time.Time)) < cwSeenThrottle {
		return
	}
	cwLastSeen.Store(key, time.Now())

	phone, chatID, altID, _ := s.directIdentity(chat, alt)
	sourceID, convID := cfg.findOpenConversation(chatID, phone, altID)
	if sourceID == "" || convID == 0 {
		return
	}
	ident := cfg.inboxIdentifier()
	if ident == "" {
		return
	}
	u := strings.TrimRight(cfg.URL, "/") + "/public/api/v1/inboxes/" + url.PathEscape(ident) +
		"/contacts/" + url.PathEscape(sourceID) + "/conversations/" + strconv.Itoa(convID) + "/update_last_seen"
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return
	}
	if resp, err := cwHTTP.Do(req); err == nil {
		resp.Body.Close()
	}
}

// inboxIdentifier devolve o identificador público da inbox: o da config ou, se
// vazio, o buscado na API (com cache).
func (c ChatwootConfig) inboxIdentifier() string {
	if c.InboxIdentifier != "" {
		return c.InboxIdentifier
	}
	key := fmt.Sprintf("%s|%d|%d", c.URL, c.AccountID, c.InboxID)
	if v, ok := cwInboxIdents.Load(key); ok && time.Since(v.(cwIdent).at) < cwIdentCacheTTL {
		return v.(cwIdent).ident
	}
	res, code, err := c.req(http.MethodGet, fmt.Sprintf("/inboxes/%d", c.InboxID), nil)
	if err != nil || code != http.StatusOK {
		return ""
	}
	ident := asStr(res["inbox_identifier"])
	if ident != "" {
		cwInboxIdents.Store(key, cwIdent{ident: ident, at: time.Now()})
	}
	return ident
}

// findOpenConversation acha (SEM criar nada) o contato do chat nesta inbox e a sua
// conversa aberta/pendente/adiada. Devolve o source_id do contato na inbox e o ID
// da conversa, ou vazio/0.
func (c ChatwootConfig) findOpenConversation(chatID, phone, altID string) (string, int) {
	queries := make([]string, 0, 4)
	if phone != "" {
		queries = append(queries, phone)
		if v := brNinthDigitVariant(phone); v != "" {
			queries = append(queries, v)
		}
	}
	queries = append(queries, chatID, altID)
	seen := map[string]bool{}
	for _, q := range queries {
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		res, code, err := c.req(http.MethodGet, "/contacts/search?q="+url.QueryEscape(q), nil)
		if err != nil || code != http.StatusOK {
			continue
		}
		for _, it := range asList(res["payload"]) {
			m := asMap(it)
			if !contactMatches(m, chatID, phone, altID) {
				continue
			}
			sid := sourceIDForInbox(m, c.InboxID)
			if sid == "" {
				continue
			}
			if conv := c.openConversationOf(asInt(m["id"])); conv != 0 {
				return sid, conv
			}
		}
	}
	return "", 0
}

// contactMatches confere que o resultado da busca (por substring) é mesmo o contato.
func contactMatches(m map[string]any, chatID, phone, altID string) bool {
	ident := asStr(m["identifier"])
	attr := ""
	if ca := asMap(m["custom_attributes"]); ca != nil {
		attr = asStr(ca[cwChatIDAttr])
	}
	if isGroupChatID(ident) || isGroupChatID(attr) {
		return false
	}
	for _, want := range []string{chatID, altID} {
		if want != "" && (ident == want || attr == want) {
			return true
		}
	}
	if phone != "" {
		p := digitsOnly(asStr(m["phone_number"]))
		if p == phone || (p != "" && p == brNinthDigitVariant(phone)) {
			return true
		}
	}
	return false
}

func (c ChatwootConfig) openConversationOf(contactID int) int {
	if contactID == 0 {
		return 0
	}
	res, code, err := c.req(http.MethodGet, fmt.Sprintf("/contacts/%d/conversations", contactID), nil)
	if err != nil || code != http.StatusOK {
		return 0
	}
	for _, it := range asList(res["payload"]) {
		m := asMap(it)
		if asInt(m["inbox_id"]) != c.InboxID {
			continue
		}
		switch asStr(m["status"]) {
		case "open", "pending", "snoozed":
			return asInt(m["id"])
		}
	}
	return 0
}
