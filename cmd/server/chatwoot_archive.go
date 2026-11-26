package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Espelho arquivar (WhatsApp) <-> resolver (Chatwoot). Opção por inbox
// (ChatwootConfig.ArchiveSync), desligada por padrão:
//
//   WhatsApp arquiva    -> conversa no Chatwoot fica resolvida (ou pendente, se a opção for "pending")
//   WhatsApp desarquiva -> conversa resolvida no Chatwoot reabre
//   Chatwoot resolve    -> chat arquivado no WhatsApp
//   Chatwoot reabre     -> chat desarquivado no WhatsApp
//
// Anti-eco: cada mudança que o AstraCalls faz de um lado é anotada por um tempo;
// o evento que volta (webhook do Chatwoot ou app state do WhatsApp) e casa com a
// anotação é ignorado. Assim a importação de histórico (que resolve conversas),
// a abertura por resposta humana e o próprio espelho não disparam o outro lado.
// A sincronização completa do app state ao parear (FromFullSync) é ignorada:
// não resolve em massa as conversas dos chats que já estavam arquivados.

const (
	archiveSyncResolved = "resolved"
	archiveSyncPending  = "pending"
)

func validArchiveSync(m string) bool {
	return m == "" || m == "off" || m == archiveSyncResolved || m == archiveSyncPending
}

// archiveSync devolve o modo efetivo: "" (desligado), "resolved" ou "pending".
func (c ChatwootConfig) archiveSync() string {
	if c.ArchiveSync == archiveSyncResolved || c.ArchiveSync == archiveSyncPending {
		return c.ArchiveSync
	}
	return ""
}

// ---------- anti-eco ----------

var echoTTL = 90 * time.Second

type echoMark struct {
	val string
	at  time.Time
}

var (
	echoMu    sync.Mutex
	echoMarks = map[string]echoMark{}
)

func markEcho(key, val string) {
	echoMu.Lock()
	defer echoMu.Unlock()
	now := time.Now()
	for k, m := range echoMarks {
		if now.Sub(m.at) > echoTTL {
			delete(echoMarks, k)
		}
	}
	echoMarks[key] = echoMark{val: val, at: now}
}

// takeEcho diz se o evento (key, val) é eco de uma mudança feita pelo AstraCalls.
func takeEcho(key, val string) bool {
	echoMu.Lock()
	defer echoMu.Unlock()
	m, ok := echoMarks[key]
	if !ok || time.Since(m.at) > echoTTL || m.val != val {
		return false
	}
	delete(echoMarks, key)
	return true
}

func cwStatusEchoKey(c ChatwootConfig, convID int) string {
	return fmt.Sprintf("cw|%s|%d|%d", strings.TrimRight(c.URL, "/"), c.AccountID, convID)
}

func waArchiveEchoKey(sessionID, chat string) string {
	return "wa|" + sessionID + "|" + chat
}

// ---------- WhatsApp -> Chatwoot ----------

func (s *Session) chatwootArchiveEvent(evt *events.Archive) {
	if evt == nil || evt.FromFullSync || evt.Action == nil {
		return
	}
	cfg := s.getChatwoot()
	mode := cfg.archiveSync()
	if !cfg.valid() || mode == "" {
		return
	}
	jid := evt.JID.ToNonAD()
	if jid.Server == types.BroadcastServer || jid.Server == types.NewsletterServer || jid.IsEmpty() {
		return
	}
	archived := evt.Action.GetArchived()
	s.log.Info("whatsapp: chat (des)arquivado", "chat", jid.String(), "archived", archived)
	aliases := []string{jid.String()}
	if s.client != nil {
		aliases = s.chatAliases(jid)
	}
	for _, alias := range aliases {
		if takeEcho(waArchiveEchoKey(s.id, alias), fmt.Sprint(archived)) {
			s.log.Info("whatsapp: arquivamento é eco do Chatwoot; ignorado", "chat", alias)
			return
		}
	}
	chatID, phone := jid.String(), ""
	if jid.Server != types.GroupServer {
		var resolved bool
		phone, chatID, _, resolved = s.directIdentity(jid, types.EmptyJID)
		if !resolved {
			phone = ""
		}
	}
	contactID := cfg.findContactID(chatID, phone)
	if contactID == 0 {
		s.log.Info("chatwoot: arquivamento sem contato no Chatwoot", "chat", chatID, "phone", phone)
		return // sem contato no Chatwoot: nada a espelhar
	}
	convID, status := cfg.latestConversation(contactID)
	if convID == 0 {
		s.log.Info("chatwoot: arquivamento sem conversa nesta inbox", "contact", contactID)
		return
	}
	target := ""
	if archived {
		switch status {
		case convStatusOpen, "snoozed":
			target = mode
		case convStatusPending:
			if mode == archiveSyncResolved {
				target = convStatusResolved
			}
		}
	} else if status == convStatusResolved {
		target = convStatusOpen
	}
	if target == "" {
		s.log.Info("chatwoot: arquivamento sem mudança de status", "conv", convID, "status", status, "archived", archived)
		return
	}
	if err := cfg.setConversationStatus(convID, target); err != nil {
		s.log.Warn("chatwoot: espelhar arquivamento falhou", "conv", convID, "err", err)
		return
	}
	s.log.Info("chatwoot: arquivamento do WhatsApp espelhado", "conv", convID, "archived", archived, "status", target)
}

// findContactID acha o contato (sem criar): por telefone nas conversas 1:1, ou
// pelo identifier/atributo exato nos grupos.
func (c ChatwootConfig) findContactID(chatID, phone string) int {
	queries := []string{chatID}
	if phone != "" {
		queries = []string{phone}
		if v := brNinthDigitVariant(phone); v != "" {
			queries = append(queries, v)
		}
	}
	for _, q := range queries {
		res, code, err := c.searchContacts(q)
		if err != nil || code != 200 {
			continue
		}
		for _, it := range asList(res["payload"]) {
			m := asMap(it)
			ident := asStr(m["identifier"])
			attr := ""
			if ca := asMap(m["custom_attributes"]); ca != nil {
				attr = asStr(ca[cwChatIDAttr])
			}
			if phone == "" {
				if ident != chatID && attr != chatID {
					continue
				}
			} else if isGroupChatID(ident) || isGroupChatID(attr) {
				continue
			}
			if id := asInt(m["id"]); id != 0 {
				return id
			}
		}
	}
	return 0
}

// latestConversation devolve a conversa mais recente do contato nesta inbox.
func (c ChatwootConfig) latestConversation(contactID int) (convID int, status string) {
	res, code, err := c.req("GET", fmt.Sprintf("/contacts/%d/conversations", contactID), nil)
	if err != nil || code != 200 {
		return 0, ""
	}
	for _, it := range asList(res["payload"]) {
		m := asMap(it)
		if asInt(m["inbox_id"]) != c.InboxID {
			continue
		}
		if id := asInt(m["id"]); id > convID {
			convID, status = id, asStr(m["status"])
		}
	}
	return convID, status
}

// ---------- Chatwoot -> WhatsApp ----------

// chatwootStatusChanged trata o webhook conversation_status_changed da inbox.
func (s *Session) chatwootStatusChanged(body map[string]any) {
	cfg := s.getChatwoot()
	if !cfg.valid() || cfg.archiveSync() == "" {
		return
	}
	convID := asInt(body["id"])
	status := asStr(body["status"])
	if convID == 0 {
		return
	}
	s.log.Info("chatwoot: status da conversa mudou", "conv", convID, "status", status)
	if takeEcho(cwStatusEchoKey(cfg, convID), status) {
		return // mudança feita pelo próprio AstraCalls
	}
	var archive bool
	switch status {
	case convStatusResolved:
		archive = true
	case convStatusOpen:
		archive = false
	default:
		return // pendente/adiada: o WhatsApp não tem equivalente
	}
	jid, ok := statusChangeChat(body)
	if !ok {
		s.log.Info("chatwoot: status mudou mas a conversa não tem chat do WhatsApp", "conv", convID)
		return
	}
	if err := s.setWAArchive(jid, archive); err != nil {
		s.log.Warn("chatwoot: espelhar status no WhatsApp falhou", "conv", convID, "err", err)
		return
	}
	s.log.Info("chatwoot: status espelhado no WhatsApp", "conv", convID, "archived", archive)
}

// statusChangeChat extrai o chat do WhatsApp do contato da conversa no webhook.
func statusChangeChat(body map[string]any) (types.JID, bool) {
	sender := asMap(asMap(body["meta"])["sender"])
	if sender == nil {
		return types.EmptyJID, false
	}
	cands := []string{asStr(sender["identifier"])}
	if ca := asMap(sender["custom_attributes"]); ca != nil {
		cands = append(cands, asStr(ca[cwChatIDAttr]))
	}
	for _, c := range cands {
		if c == "" || !strings.Contains(c, "@") {
			continue
		}
		if j, err := types.ParseJID(c); err == nil && (j.Server == types.DefaultUserServer || j.Server == types.GroupServer || j.Server == types.HiddenUserServer) {
			return j, true
		}
	}
	if p := digitsOnly(asStr(sender["phone_number"])); p != "" {
		return types.NewJID(p, types.DefaultUserServer), true
	}
	return types.EmptyJID, false
}

// setWAArchive arquiva/desarquiva o chat no WhatsApp. Usa a forma do JID (PN ou
// LID) da última mensagem guardada, que é como o WhatsApp conhece o chat.
func (s *Session) setWAArchive(jid types.JID, archive bool) error {
	if s.client == nil || !s.client.IsLoggedIn() {
		return fmt.Errorf("sessão desconectada")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	target := jid
	var ts time.Time
	var key *waCommon.MessageKey
	if s.mgr != nil && s.mgr.store != nil {
		if msgs, err := s.mgr.store.listMessagesIn(ctx, s.id, s.chatAliases(jid), 1, 0, false); err == nil && len(msgs) > 0 {
			m := msgs[0]
			if j, err := types.ParseJID(m.ChatJID); err == nil {
				target = j
			}
			ts = time.UnixMilli(m.Timestamp)
			key = &waCommon.MessageKey{RemoteJID: proto.String(m.ChatJID), FromMe: proto.Bool(m.FromMe), ID: proto.String(m.MsgID)}
			if target.Server == types.GroupServer && !m.FromMe && m.SenderJID != "" {
				key.Participant = proto.String(m.SenderJID)
			}
		}
	}
	markEcho(waArchiveEchoKey(s.id, target.String()), fmt.Sprint(archive))
	return s.client.SendAppState(ctx, appstate.BuildArchive(target, archive, ts, key))
}
