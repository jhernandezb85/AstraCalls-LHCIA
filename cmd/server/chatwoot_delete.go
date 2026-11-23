package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Mensagens apagadas, nos dois sentidos.
//
// WhatsApp -> Chatwoot ("apagar para todos"):
//   - o CONTATO apagou: conforme contact_delete — "note" (padrão) mantém a mensagem e
//     posta uma nota privada citando-a; "delete" apaga também no Chatwoot.
//   - a CONTA apagou (no celular): apaga no Chatwoot.
//   Se a mensagem não é conhecida (antiga, ou entrou como nota), fica a nota.
//
// Chatwoot -> WhatsApp: o atendente apagou uma mensagem de SAÍDA no Chatwoot (o
// Chatwoot troca o conteúdo e manda message_updated com deleted=true) -> "apagar
// para todos" no WhatsApp. Passado o limite do WhatsApp, só avisa com uma nota.

const (
	contactDeleteNote   = "note"
	contactDeleteDelete = "delete"

	// o WhatsApp só deixa "apagar para todos" por ~2 dias e meio após o envio
	waRevokeWindow = 60 * time.Hour
	// tempo em que um apagamento feito por nós no Chatwoot é ignorado no webhook
	cwDeleteEchoTTL = 2 * time.Minute
)

func validContactDelete(m string) bool {
	return m == "" || m == contactDeleteNote || m == contactDeleteDelete
}

// contactDelete devolve o modo efetivo (padrão: note).
func (c ChatwootConfig) contactDelete() string {
	if c.ContactDelete == "" {
		return contactDeleteNote
	}
	return c.ContactDelete
}

var cwDeletedByUs sync.Map // "<sessão>|#<id no Chatwoot>" -> time.Time

// ---------- WhatsApp -> Chatwoot ----------

// chatwootOnRevoke reflete no Chatwoot um "apagar para todos" do WhatsApp.
func (s *Session) chatwootOnRevoke(evt *events.Message) {
	cfg := s.getChatwoot()
	if !cfg.valid() {
		return
	}
	origID := evt.Message.GetProtocolMessage().GetKey().GetID()
	if origID == "" {
		return
	}
	switch evt.Info.Chat.Server {
	case types.DefaultUserServer, types.HiddenUserServer:
	case types.GroupServer:
		if !cfg.Groups || (!evt.Info.IsFromMe && cfg.GroupsSkipIncoming) {
			return
		}
	default:
		return
	}
	ref, known := s.lookupCWMessage(origID)
	if evt.Info.IsFromMe {
		// a conta apagou: some do Chatwoot também. Se não a conhecemos (entrou como
		// nota, saiu pela API sem espelho...), nada a fazer.
		if known && !s.deleteCWMessage(cfg, ref) {
			s.chatwootSend(cfg, s.revokeNoteJob(evt, origID))
		}
		return
	}
	if cfg.contactDelete() == contactDeleteDelete && known && s.deleteCWMessage(cfg, ref) {
		return
	}
	s.chatwootSend(cfg, s.revokeNoteJob(evt, origID))
}

// revokeNoteJob monta a nota privada "mensagem apagada", citando a original.
func (s *Session) revokeNoteJob(evt *events.Message, origID string) cwJob {
	text := "🗑️ _El contacto eliminó este mensaje en WhatsApp_"
	if evt.Info.IsFromMe {
		text = "🗑️ _Eliminaste este mensaje en WhatsApp_"
	}
	j := cwJob{Private: true, Text: text, SourceID: origID + ":del", InReplyTo: origID}
	chat := evt.Info.Chat
	if chat.Server == types.GroupServer {
		j.ChatID, j.Name, j.Avatar = s.groupJobTarget(chat)
		if !evt.Info.IsFromMe {
			author := evt.Info.PushName
			if author == "" {
				author = s.realPhone(evt.Info.Sender)
			}
			j.Text = "🗑️ _*" + author + "* eliminó este mensaje en WhatsApp_"
		}
		return j
	}
	alt := evt.Info.SenderAlt
	if evt.Info.IsFromMe {
		alt = evt.Info.RecipientAlt
	}
	phone, chatID, altID, _ := s.directIdentity(chat, alt)
	j.ChatID, j.Phone, j.AltID = chatID, phone, altID
	j.Name = phone
	if !evt.Info.IsFromMe && evt.Info.PushName != "" {
		j.Name = evt.Info.PushName
	}
	if j.Name == "" {
		j.Name = chat.User
	}
	j.Avatar = s.cachedAvatar(chat)
	return j
}

// deleteCWMessage apaga a mensagem no Chatwoot; true se o Chatwoot aceitou.
func (s *Session) deleteCWMessage(cfg ChatwootConfig, ref cwMsgRef) bool {
	key := s.cwIDKey(ref.msg)
	cwDeletedByUs.Store(key, time.Now())
	_, code, err := cfg.req(http.MethodDelete, fmt.Sprintf("/conversations/%d/messages/%d", ref.conv, ref.msg), nil)
	if err != nil || code >= 300 {
		cwDeletedByUs.Delete(key)
		if s.log != nil {
			s.log.Warn("chatwoot: apagar mensagem falhou", "err", err, "http", code, "cw_msg", ref.msg)
		}
		return false
	}
	return true
}

// ---------- Chatwoot -> WhatsApp ----------

// isDeleteCandidate: o Chatwoot apagou uma mensagem de SAÍDA (não nota). Qual
// mensagem do WhatsApp apagar é resolvido em revokeTarget.
func isDeleteCandidate(body map[string]any) bool {
	if asStr(body["event"]) != "message_updated" || asStr(body["message_type"]) != "outgoing" {
		return false
	}
	if b, ok := body["private"].(bool); ok && b {
		return false
	}
	ca := asMap(body["content_attributes"])
	v, _ := ca["deleted"].(bool)
	return v
}

// revokeTarget devolve o ID do WhatsApp da mensagem apagada no Chatwoot: o
// source_id gravado nela ou, se faltar (o endpoint que o grava não existe em todo
// Chatwoot), a relação guardada por nós ao enviar. "" = nunca saiu pelo WhatsApp.
func (s *Session) revokeTarget(body map[string]any) string {
	if wa := revokableSourceID(asStr(body["source_id"])); wa != "" {
		return wa
	}
	return s.waIDForCW(asInt(body["id"]))
}

// revokableSourceID devolve o ID da mensagem do WhatsApp, ou "" para source_ids
// derivados (edição, nota de anúncio, nota de apagada...).
func revokableSourceID(sid string) string {
	if sid == "" || strings.Contains(sid, ":") {
		return ""
	}
	return sid
}

// takeDeletedByUs diz se o apagamento veio de nós (eco do chatwootOnRevoke).
func (s *Session) takeDeletedByUs(cwID int) bool {
	v, ok := cwDeletedByUs.LoadAndDelete(s.cwIDKey(cwID))
	return ok && time.Since(v.(time.Time)) < cwDeleteEchoTTL
}

// chatwootRevoke apaga no WhatsApp ("para todos") a mensagem apagada no Chatwoot.
func (s *Session) chatwootRevoke(body map[string]any) {
	cwID := asInt(body["id"])
	if s.takeDeletedByUs(cwID) {
		return
	}
	cfg := s.getChatwoot()
	if !cfg.valid() {
		return
	}
	waID := s.revokeTarget(body)
	if waID == "" {
		if s.log != nil {
			s.log.Info("chatwoot->wa: mensagem apagada no Chatwoot não saiu pelo WhatsApp; nada a apagar", "cw_msg", cwID)
		}
		return
	}
	if s.log != nil {
		s.log.Info("chatwoot->wa: apagando no WhatsApp", "cw_msg", cwID, "wa_msg", waID)
	}
	convID := asInt(asMap(body["conversation"])["id"])
	if created, err := time.Parse(time.RFC3339Nano, asStr(body["created_at"])); err == nil && time.Since(created) > waRevokeWindow {
		s.revokeFailedNote(cfg, convID, waID, "WhatsApp solo permite eliminar para todos durante unos 2 días.")
		return
	}
	jid, err := resolveRecipient(chatIDFromWebhook(body))
	if err != nil {
		return
	}
	if s.client == nil {
		s.revokeFailedNote(cfg, convID, waID, "")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cwSendTimeout)
	defer cancel()
	revoke := s.client.BuildRevoke(jid, types.EmptyJID, waID)
	_, err = s.sendResolvingLID(ctx, jid, revoke)
	if err != nil && ctx.Err() == nil && !errors.Is(err, whatsmeow.ErrNotLoggedIn) && !errors.Is(err, whatsmeow.ErrNotConnected) {
		// falha passageira (ex.: consulta de aparelhos sem resposta): tenta mais uma vez
		time.Sleep(3 * time.Second)
		_, err = s.sendResolvingLID(ctx, jid, revoke)
	}
	if err != nil {
		if s.log != nil {
			s.log.Warn("chatwoot->wa: apagar no WhatsApp falhou", "err", err, "wa_msg", waID)
		}
		reason := ""
		if errors.Is(err, whatsmeow.ErrNotLoggedIn) || errors.Is(err, whatsmeow.ErrNotConnected) {
			reason = "El número no está conectado a WhatsApp."
		}
		s.revokeFailedNote(cfg, convID, waID, reason)
	}
}

func (s *Session) revokeFailedNote(cfg ChatwootConfig, convID int, waID, reason string) {
	if convID == 0 {
		return
	}
	note := strings.TrimSpace("⚠️ _No se pudo eliminar en WhatsApp: el contacto todavía ve este mensaje._ " + reason)
	_ = cfg.postText(convID, note, cwPrivate, "", waID, 0, nil)
}
