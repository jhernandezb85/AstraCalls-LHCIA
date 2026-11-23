package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// Mensagens enviadas PELO APARELHO (device_mode) e o "Reenviar" do Chatwoot.
//
// Aparelho: o que a conta escreve no celular aparece no Chatwoot conforme a
// device_mode da sessão:
//   - open (padrão): mensagem de SAÍDA com o título do aparelho e a conversa passa
//     para "aberta" — uma pessoa assumiu, então bots/Captain param de responder.
//   - keep: mensagem de SAÍDA com o título, sem mexer no status da conversa.
//   - note: nota privada com o título (comportamento antigo).
//
// Com device_bot_token, a mensagem é postada por um Agent Bot da conta: aparece
// com o nome do bot (ex.: "WhatsApp (celular)") e o Chatwoot não a conta como
// resposta humana — por isso "keep" consegue manter a conversa pendente.
//
// Reenviar: numa inbox API, o "Reenviar" do Chatwoot só troca o status da
// mensagem para "enviado" (evento message_updated) — nada chega ao WhatsApp. Aqui
// o AstraCalls reconhece esse evento e envia a mensagem de verdade.

const (
	deviceModeOpen = "open"
	deviceModeKeep = "keep"
	deviceModeNote = "note"

	defaultDeviceTitle = "📱 Enviado desde el celular"

	// janela em que um message_updated de mensagem nunca enviada é tratado como
	// "Reenviar" (evita reenviar mensagens antigas por qualquer outra atualização)
	cwRetryWindow = 48 * time.Hour
	// o Chatwoot espera o webhook por ~5s; acima disso ele marca "falhou"
	cwWebhookSlow = 4 * time.Second
	// tempo máximo de um envio Chatwoot -> WhatsApp (grupos grandes demoram)
	cwSendTimeout = 3 * time.Minute
)

func validDeviceMode(m string) bool {
	switch m {
	case "", deviceModeOpen, deviceModeKeep, deviceModeNote:
		return true
	}
	return false
}

// deviceMode devolve a device_mode efetiva (padrão: open).
func (c ChatwootConfig) deviceMode() string {
	if c.DeviceMode == "" {
		return deviceModeOpen
	}
	return c.DeviceMode
}

func (c ChatwootConfig) deviceTitle() string {
	if t := strings.TrimSpace(c.DeviceTitle); t != "" {
		return t
	}
	return defaultDeviceTitle
}

// ownMirrorJob monta o job de uma mensagem from_me espelhada (aparelho ou API com
// mirror_api). author é o prefixo de autor em grupo ("" em 1:1). O que saiu pela
// nossa API é sempre nota privada: uma saída "humana" abriria conversas pendentes
// e quebraria fluxos automatizados.
func (s *Session) ownMirrorJob(cfg ChatwootConfig, evt *events.Message, author string) cwJob {
	title := cfg.deviceTitle() + "\n"
	if _, viaUs := s.selfSentOrigin(evt.Info.ID); viaUs {
		return deliverContent(evt, title+author, true)
	}
	mode := cfg.deviceMode()
	if mode == deviceModeNote {
		return deliverContent(evt, title+author, true)
	}
	j := deliverContent(evt, title+author, false)
	j.Outgoing = true
	j.Device = true
	j.OpenConv = mode == deviceModeOpen
	return j
}

// postAsJob posta com o Agent Bot quando o job é do aparelho e há token de bot;
// se o bot for recusado (removido, sem acesso), cai para o token da conta.
func (s *Session) postAsJob(cfg ChatwootConfig, j cwJob, post func(ChatwootConfig) (int, error)) (int, error) {
	if j.Device && j.Outgoing && cfg.DeviceBotToken != "" {
		bc := cfg
		bc.AccountToken = cfg.DeviceBotToken
		id, err := post(bc)
		if err == nil || !isAuthHTTPErr(err) {
			return id, err
		}
		if s.log != nil {
			s.log.Warn("chatwoot: bot do aparelho recusado; postando com o token da conta", "err", err)
		}
	}
	return post(cfg)
}

func isAuthHTTPErr(err error) bool {
	m := err.Error()
	return strings.Contains(m, "http 401") || strings.Contains(m, "http 403") || strings.Contains(m, "http 404")
}

// afterJobPosted: guarda a relação WhatsApp ↔ Chatwoot (recibos das mensagens de
// saída; apagar recebidas — ver chatwoot_delete.go) e, em device_mode open, passa
// a conversa para "aberta". Notas privadas não entram (não têm tiques).
func (s *Session) afterJobPosted(cfg ChatwootConfig, j cwJob, convID, cwID int) {
	if j.Private {
		return
	}
	s.rememberCWMessage(j.SourceID, convID, cwID)
	if !j.Outgoing {
		return
	}
	if j.OpenConv {
		// abre com o bot (o aviso na timeline diz "reaberta por <bot>", não pelo
		// dono do token da conta); se o bot for recusado, usa o token da conta
		if cfg.DeviceBotToken != "" {
			bc := cfg
			bc.AccountToken = cfg.DeviceBotToken
			if code := bc.openConversation(convID); code > 0 && code < 300 {
				return
			}
		}
		cfg.openConversation(convID)
	}
}

// openConversation passa a conversa para "aberta" (no-op se já estiver) e
// devolve o status HTTP (0 em erro de rede).
func (c ChatwootConfig) openConversation(convID int) int {
	if convID == 0 {
		return 0
	}
	_, code, _ := c.req(http.MethodPost, fmt.Sprintf("/conversations/%d/toggle_status", convID), map[string]any{"status": "open"})
	return code
}

// ---------- Reenviar ----------

// isRetryCandidate: um message_updated de mensagem de SAÍDA do agente (não nota,
// sem source_id, não apagada) pode ser um "Reenviar". A confirmação vem em
// claimChatwootRetry.
func isRetryCandidate(body map[string]any) bool {
	if asStr(body["event"]) != "message_updated" || asStr(body["message_type"]) != "outgoing" {
		return false
	}
	if b, ok := body["private"].(bool); ok && b {
		return false
	}
	if asStr(body["source_id"]) != "" || asInt(body["id"]) == 0 {
		return false
	}
	if ca := asMap(body["content_attributes"]); ca != nil {
		if v, _ := ca["astracall_rich_sent"].(bool); v {
			return false
		}
		if v, _ := ca["deleted"].(bool); v {
			return false
		}
	}
	return true
}

// claimChatwootRetry confirma que o message_updated é um "Reenviar": a mensagem é
// recente, nunca saiu por nós e o Chatwoot a mostra agora como "enviada" (o
// Reenviar troca falhou -> enviado). Atualizações dos nossos próprios status
// (entregue/lida/falhou) não passam: ou a mensagem é conhecida ou o status difere.
func (s *Session) claimChatwootRetry(body map[string]any) bool {
	cwID := asInt(body["id"])
	if s.knownCWMessage(cwID) {
		return false
	}
	created, err := time.Parse(time.RFC3339Nano, asStr(body["created_at"]))
	if err != nil || time.Since(created) > cwRetryWindow {
		return false
	}
	cfg := s.getChatwoot()
	if !cfg.valid() {
		return false
	}
	convID := asInt(asMap(body["conversation"])["id"])
	status, sourceID, found := cfg.messageState(convID, cwID)
	return found && status == "sent" && sourceID == ""
}

// messageState lê o status e o source_id atuais de uma mensagem do Chatwoot.
func (c ChatwootConfig) messageState(convID, msgID int) (status, sourceID string, found bool) {
	if convID == 0 || msgID == 0 {
		return "", "", false
	}
	res, code, err := c.req(http.MethodGet, fmt.Sprintf("/conversations/%d/messages?before=%d", convID, msgID+1), nil)
	if err != nil || code != http.StatusOK {
		return "", "", false
	}
	for _, it := range asList(res["payload"]) {
		m := asMap(it)
		if asInt(m["id"]) == msgID {
			return asStr(m["status"]), asStr(m["source_id"]), true
		}
	}
	return "", "", false
}

// chatwootFailReason traduz a falha de envio num motivo curto para o atendente.
func chatwootFailReason(err error) string {
	switch {
	case err == nil:
		return "No se pudo enviar por WhatsApp. Pulsa Reenviar para intentarlo de nuevo."
	case errors.Is(err, whatsmeow.ErrNotLoggedIn), errors.Is(err, whatsmeow.ErrNotConnected):
		return "El número no está conectado a WhatsApp. Reconéctalo y pulsa Reenviar."
	case errors.Is(err, whatsmeow.ErrRecipientNotOnWhatsApp):
		return "Este número no tiene WhatsApp."
	case errors.Is(err, whatsmeow.ErrNoPermissionInGroup):
		return "Solo los administradores pueden escribir en este grupo."
	}
	return "No se pudo enviar por WhatsApp. Pulsa Reenviar para intentarlo de nuevo."
}

var cwRelaying sync.Map // "<sessão>|<id da msg no Chatwoot>" -> struct{}

// beginRelay reserva a mensagem do Chatwoot para envio; false se já está em curso.
func (s *Session) beginRelay(cwID int) bool {
	if cwID == 0 {
		return true
	}
	_, busy := cwRelaying.LoadOrStore(fmt.Sprintf("%s|%d", s.id, cwID), struct{}{})
	return !busy
}

func (s *Session) endRelay(cwID int) {
	if cwID != 0 {
		cwRelaying.Delete(fmt.Sprintf("%s|%d", s.id, cwID))
	}
}
