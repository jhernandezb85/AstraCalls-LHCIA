package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Editar desde Chatwoot un mensaje ya enviado por WhatsApp.
//
// Chatwoot (hasta 4.20) no deja editar el contenido de un mensaje. El botón
// "Editar" lo pone el script del provisioner, que valida al agente con su propia
// sesión de Chatwoot y llama a este endpoint con la llave maestra (no es parte
// de la superficie del widget). AstraCalls edita el mensaje en WhatsApp y deja
// en la conversación una nota privada con el texto nuevo, porque el mensaje de
// Chatwoot no se puede cambiar.

// waEditWindow: WhatsApp solo permite editar durante 15 minutos.
const waEditWindow = 15 * time.Minute

// motivos por los que no se puede editar (códigos de error de la API)
const (
	editNotSent  = "not_sent"  // ese mensaje de Chatwoot no salió por WhatsApp
	editNotOwn   = "not_own"   // no es un mensaje de la cuenta
	editNotText  = "not_text"  // solo mensajes de texto (no adjuntos)
	editTooLate  = "too_late"  // pasaron los 15 minutos
	editBadInput = "bad_input" // texto vacío o demasiado largo
)

// editCheck decide si el mensaje se puede editar. "" = se puede.
func editCheck(waID string, fromMe bool, raw json.RawMessage, sentAt, now time.Time, text string) string {
	if strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 4096 {
		return editBadInput
	}
	if waID == "" {
		return editNotSent
	}
	if !fromMe {
		return editNotOwn
	}
	if !isPlainTextMessage(raw) {
		return editNotText
	}
	if sentAt.IsZero() || now.Sub(sentAt) > waEditWindow {
		return editTooLate
	}
	return ""
}

// isPlainTextMessage: el mensaje guardado es solo texto (conversation o extendedText).
func isPlainTextMessage(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	for k := range m {
		switch k {
		case "conversation", "extendedTextMessage", "messageContextInfo":
		default:
			return false
		}
	}
	_, conv := m["conversation"]
	_, ext := m["extendedTextMessage"]
	return conv || ext
}

var editErrorText = map[string]string{
	editNotSent:  "Este mensaje no se envió por WhatsApp.",
	editNotOwn:   "Solo se pueden editar los mensajes enviados por la cuenta.",
	editNotText:  "Solo se pueden editar mensajes de texto.",
	editTooLate:  "WhatsApp solo permite editar durante 15 minutos después de enviar.",
	editBadInput: "Escribe el texto nuevo (máximo 4096 caracteres).",
}

// POST /api/sessions/{sid}/chatwoot/messages/{cwid}/edit  {"text": "...", "conversationId": 123}
func (s *server) handleChatwootEdit(w http.ResponseWriter, r *http.Request) {
	sess := s.sessionByID(w, r.PathValue("sid"))
	if sess == nil {
		return
	}
	var b struct {
		Text           string `json:"text"`
		ConversationID int    `json:"conversationId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&b)
	b.Text = strings.TrimSpace(b.Text)
	cwID := asInt(r.PathValue("cwid"))
	fail := func(code int, reason string) {
		writeJSON(w, code, map[string]string{"error": reason, "message": editErrorText[reason]})
	}
	if b.Text == "" || cwID <= 0 {
		fail(http.StatusBadRequest, editBadInput)
		return
	}
	cfg := sess.getChatwoot()
	st := sess.store()
	if !cfg.valid() || st == nil || sess.client == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "not_ready", "message": "La cuenta no está conectada a Chatwoot o a WhatsApp."})
		return
	}
	waID := sess.waIDForCW(cwID)
	var (
		chat   string
		fromMe bool
		raw    json.RawMessage
		sentAt time.Time
		convID int
	)
	if waID != "" {
		var err error
		chat, _, fromMe, raw, err = st.findMessage(r.Context(), sess.id, waID)
		if err != nil {
			waID = "" // no está en el historial: no se puede resolver el chat
		}
		if ref, ok := sess.lookupCWMessage(waID); ok {
			sentAt, convID = ref.at, ref.conv
		}
		// el mensaje debe ser de la conversación que dice el agente
		if b.ConversationID > 0 && convID > 0 && convID != b.ConversationID {
			waID = ""
		}
	}
	if reason := editCheck(waID, fromMe, raw, sentAt, time.Now(), b.Text); reason != "" {
		code := http.StatusConflict
		if reason == editNotSent {
			code = http.StatusNotFound
		}
		fail(code, reason)
		return
	}
	jid, err := types.ParseJID(chat)
	if err != nil {
		fail(http.StatusNotFound, editNotSent)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), cwSendTimeout)
	defer cancel()
	edit := sess.client.BuildEdit(jid, waID, &waE2E.Message{Conversation: proto.String(b.Text)})
	if _, err := sess.sendResolvingLID(ctx, jid, edit); err != nil {
		sess.log.Warn("chatwoot->wa: editar en WhatsApp falló", "err", err, "wa_msg", waID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "send_failed", "message": "WhatsApp no aceptó la edición. Inténtalo de nuevo."})
		return
	}
	sess.log.Info("chatwoot->wa: mensaje editado en WhatsApp", "cw_msg", cwID, "wa_msg", waID)
	// el mensaje de Chatwoot no cambia: queda una nota con el texto nuevo
	if convID > 0 {
		note := "✏️ _Editado en WhatsApp:_\n" + b.Text
		if err := cfg.postText(convID, note, cwPrivate, "", waID, 0, nil); err != nil {
			sess.log.Warn("chatwoot: nota de edición falló", "err", err, "cw_msg", cwID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"edited": true})
}
