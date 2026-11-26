package call

import (
	"log/slog"
	"testing"

	"wacalls/internal/voip/core"
)

// Chamada recebida tocando aqui e atendida no celular: encerra só localmente.
func TestAnsweredElsewhereEndsRingingIncoming(t *testing.T) {
	m := incomingRingingManager()
	var ended *CallInfo
	m.OnEnded = func(c *CallInfo) { ended = c }
	if !m.HandleAnsweredElsewhere() {
		t.Fatal("deve encerrar a chamada que tocava")
	}
	if ended == nil || ended.StateData.EndReason != EndReasonAcceptedElsewhere {
		t.Fatalf("motivo accepted_elsewhere: %+v", ended)
	}
	if !m.CurrentCall().IsEnded() {
		t.Fatal("chamada encerrada")
	}
	if m.HandleAnsweredElsewhere() {
		t.Fatal("segunda vez não faz nada")
	}
}

// Chamada de saída ou já atendida aqui não é afetada.
func TestAnsweredElsewhereIgnoresOutgoing(t *testing.T) {
	m := NewCallManager(&fakeSock{}, slog.Default())
	m.relay = fakeRelay{}
	m.currentCall = NewOutgoingCall("OUT1", testIncomingPeer, "5511@s.whatsapp.net", core.CallMediaTypeAudio)
	if m.HandleAnsweredElsewhere() {
		t.Fatal("chamada de saída não é encerrada")
	}
}
