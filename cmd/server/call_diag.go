package main

import (
	"strings"
	"sync"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Diagnóstico de llamadas hechas o contestadas en el celular.
//
// Objetivo: saber qué recibe el dispositivo vinculado cuando la llamada no pasa
// por AstraCalls (se contesta, se cuelga o se hace en el celular). Se registran
// todas las señales de llamada (offer/accept/terminate/…) y el registro de
// llamadas que el celular sincroniza (CallLogAction por app state y
// CallLogMessage por mensaje). Solo registra: no cambia el comportamiento.

// diagSyncSeen limita el log de sincronizaciones a una línea por tipo y minuto
// (al reconectar llegan miles de golpe).
var (
	diagSyncMu   sync.Mutex
	diagSyncSeen = map[string]time.Time{}
)

func diagSyncOnce(kind string) bool {
	diagSyncMu.Lock()
	defer diagSyncMu.Unlock()
	if t, ok := diagSyncSeen[kind]; ok && time.Since(t) < time.Minute {
		return false
	}
	diagSyncSeen[kind] = time.Now()
	return true
}

func nodeSummary(n *waBinary.Node) (tag string, attrs map[string]string) {
	attrs = map[string]string{}
	if n == nil {
		return "", attrs
	}
	tag = n.Tag
	for k, v := range n.Attrs {
		if s, ok := v.(string); ok {
			attrs[k] = s
		} else {
			attrs[k] = stringify(v)
		}
	}
	return tag, attrs
}

func stringify(v any) string {
	if st, ok := v.(interface{ String() string }); ok {
		return st.String()
	}
	return ""
}

func childTags(n *waBinary.Node) string {
	if n == nil {
		return ""
	}
	var tags []string
	for _, c := range n.GetChildren() {
		tags = append(tags, c.Tag)
	}
	return strings.Join(tags, ",")
}

// diagCallEvent registra los eventos de llamada y el registro de llamadas.
func (s *Session) diagCallEvent(rawEvt any) {
	logStanza := func(kind string, fromJID types.JID, creator, callID string, data *waBinary.Node) {
		from, fromOwn := fromJID.String(), s.isOwnJID(fromJID)
		tag, attrs := nodeSummary(data)
		_, known := s.reg.get(callID)
		args := []any{"kind", kind, "from", from, "creator", creator,
			"call_id", callID, "tag", tag, "attrs", attrs, "children", childTags(data),
			"from_own", fromOwn, "known", known}
		// La señal completa (salvo el offer, que es largo y trae claves): para
		// comparar el terminate de "colgó el cliente" con el de "contestada en el celular".
		if data != nil && kind != "offer" {
			x := data.String()
			if len(x) > 2000 {
				x = x[:2000]
			}
			args = append(args, "xml", x)
		}
		s.log.Info("diag llamada: señal", args...)
	}
	switch evt := rawEvt.(type) {
	case *events.CallOffer:
		logStanza("offer", evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallAccept:
		logStanza("accept", evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallPreAccept:
		logStanza("preaccept", evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallTerminate:
		logStanza("terminate:"+evt.Reason, evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallReject:
		logStanza("reject", evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallOfferNotice:
		logStanza("offer_notice", evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallRelayLatency:
		logStanza("relaylatency", evt.From, evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.UnknownCallEvent:
		if evt.Node != nil {
			from := ""
			if v, ok := evt.Node.Attrs["from"]; ok {
				from = stringify(v)
			}
			s.log.Info("diag llamada: señal desconocida", "from", from, "call_id", callIDFromNode(evt.Node),
				"children", childTags(evt.Node))
		}
	case *events.AppState:
		if evt.SyncActionValue == nil {
			return
		}
		if evt.GetCallLogAction() == nil {
			// Cualquier otra sincronización del celular: solo el tipo, para ver si el
			// registro de llamadas llega con otro nombre.
			kind := ""
			if len(evt.Index) > 0 {
				kind = evt.Index[0]
			}
			if diagSyncOnce(kind) {
				s.log.Info("diag llamada: sync", "kind", kind)
			}
			return
		}
		r := evt.GetCallLogAction().GetCallLogRecord()
		var parts []string
		for _, p := range r.GetParticipants() {
			parts = append(parts, p.GetUserJID()+"="+p.GetCallResult().String())
		}
		s.log.Info("diag llamada: registro (app state)", "index", strings.Join(evt.Index, "|"),
			"call_id", r.GetCallID(), "result", r.GetCallResult().String(), "incoming", r.GetIsIncoming(),
			"video", r.GetIsVideo(), "duration", r.GetDuration(), "start", r.GetStartTime(),
			"creator", r.GetCallCreatorJID(), "participants", strings.Join(parts, ";"))
	case *events.Message:
		cl := evt.Message.GetCallLogMesssage()
		if cl == nil {
			return
		}
		s.log.Info("diag llamada: registro (mensaje)", "chat", evt.Info.Chat.String(),
			"from_me", evt.Info.IsFromMe, "outcome", cl.GetCallOutcome().String(),
			"duration", cl.GetDurationSecs(), "video", cl.GetIsVideo())
	}
}

// isOwnJID: o JID é do próprio número da sessão (por número ou por LID), ou seja,
// outro dispositivo da mesma conta (ex.: o celular).
func (s *Session) isOwnJID(j types.JID) bool {
	if s.client == nil || s.client.Store == nil || j.User == "" {
		return false
	}
	if id := s.client.Store.ID; id != nil && id.User == j.User {
		return true
	}
	return !s.client.Store.LID.IsEmpty() && s.client.Store.LID.User == j.User
}

// nodeAcceptedElsewhere: o <call> traz um <terminate reason="accepted_elsewhere">.
func nodeAcceptedElsewhere(n *waBinary.Node) bool {
	if n == nil {
		return false
	}
	for _, c := range n.GetChildren() {
		if c.Tag == "terminate" {
			if r, ok := c.Attrs["reason"].(string); ok && r == "accepted_elsewhere" {
				return true
			}
		}
	}
	return false
}
