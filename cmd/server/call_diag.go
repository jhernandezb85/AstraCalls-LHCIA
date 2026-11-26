package main

import (
	"strings"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types/events"
)

// Diagnóstico de llamadas hechas o contestadas en el celular.
//
// Objetivo: saber qué recibe el dispositivo vinculado cuando la llamada no pasa
// por AstraCalls (se contesta, se cuelga o se hace en el celular). Se registran
// todas las señales de llamada (offer/accept/terminate/…) y el registro de
// llamadas que el celular sincroniza (CallLogAction por app state y
// CallLogMessage por mensaje). Solo registra: no cambia el comportamiento.

func nodeSummary(n *waBinary.Node) (tag string, attrs map[string]string) {
	attrs = map[string]string{}
	if n == nil {
		return "", attrs
	}
	tag = n.Tag
	for k, v := range n.Attrs {
		switch k {
		case "reason", "call-creator", "call-id", "count", "platform", "from":
			if s, ok := v.(string); ok {
				attrs[k] = s
			} else {
				attrs[k] = stringify(v)
			}
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
	own := ""
	if s.client != nil && s.client.Store != nil && s.client.Store.ID != nil {
		own = s.client.Store.ID.User
	}
	logStanza := func(kind, from, creator, callID string, data *waBinary.Node) {
		tag, attrs := nodeSummary(data)
		_, known := s.reg.get(callID)
		s.log.Info("diag llamada: señal", "kind", kind, "from", from, "creator", creator,
			"call_id", callID, "tag", tag, "attrs", attrs, "children", childTags(data),
			"from_own", own != "" && strings.HasPrefix(from, own), "known", known)
	}
	switch evt := rawEvt.(type) {
	case *events.CallOffer:
		logStanza("offer", evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallAccept:
		logStanza("accept", evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallPreAccept:
		logStanza("preaccept", evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallTerminate:
		logStanza("terminate:"+evt.Reason, evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallReject:
		logStanza("reject", evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallOfferNotice:
		logStanza("offer_notice", evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
	case *events.CallRelayLatency:
		logStanza("relaylatency", evt.From.String(), evt.CallCreator.String(), evt.CallID, evt.Data)
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
		if evt.SyncActionValue == nil || evt.GetCallLogAction() == nil {
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
