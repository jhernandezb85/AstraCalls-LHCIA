package main

import (
	"testing"
	"time"
)

func i64(v int64) *int64 { return &v }

func TestCallNoteText(t *testing.T) {
	cases := []struct {
		rec  CallRecord
		want string
	}{
		{CallRecord{Direction: "inbound", ConnectedAt: i64(1_000), EndedAt: i64(84_000)}, "📞 Llamada entrante · contestada · 1:23"},
		{CallRecord{Direction: "outbound", ConnectedAt: i64(0), EndedAt: i64(4_500)}, "📞 Llamada saliente · contestada · 0:04"},
		{CallRecord{Direction: "inbound", EndReason: "timeout"}, "📞 Llamada entrante · perdida"},
		{CallRecord{Direction: "inbound", EndReason: "declined"}, "📞 Llamada entrante · rechazada"},
		{CallRecord{Direction: "outbound", EndReason: "timeout"}, "📞 Llamada saliente · sin respuesta"},
		{CallRecord{Direction: "outbound", EndReason: "busy"}, "📞 Llamada saliente · ocupado"},
	}
	for _, c := range cases {
		if got := callNoteText(c.rec); got != c.want {
			t.Errorf("%+v: %q, quería %q", c.rec, got, c.want)
		}
	}
}

func TestBrokerConnectedAtAndOnCallEnded(t *testing.T) {
	b := NewBroker()
	got := make(chan CallRecord, 2)
	b.OnCallEnded = func(r CallRecord) { got <- r }

	b.upsertCall(CallRecord{SessionID: "s", CallID: "c1", Direction: "inbound", Status: StatusRinging})
	if c, _ := b.getCall("c1"); c.ConnectedAt != nil {
		t.Fatal("tocando ainda não foi atendida")
	}
	b.upsertCall(CallRecord{SessionID: "s", CallID: "c1", Direction: "inbound", Status: StatusConnected})
	c, _ := b.getCall("c1")
	if c.ConnectedAt == nil {
		t.Fatal("atendida deve marcar ConnectedAt")
	}
	first := *c.ConnectedAt
	// uma atualização posterior (ex.: espera) não perde nem move o ConnectedAt
	b.upsertCall(CallRecord{SessionID: "s", CallID: "c1", Direction: "inbound", Status: StatusConnected, Held: true})
	if c, _ := b.getCall("c1"); c.ConnectedAt == nil || *c.ConnectedAt != first {
		t.Fatal("ConnectedAt deve ser preservado")
	}

	b.endCall("c1", "user_ended")
	b.endCall("c1", "user_ended") // 2º encerramento do mesmo id é ignorado
	select {
	case r := <-got:
		if r.CallID != "c1" || r.ConnectedAt == nil || r.EndedAt == nil || r.Status != StatusEnded {
			t.Fatalf("registro final inesperado: %+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnCallEnded não foi chamado")
	}
	select {
	case r := <-got:
		t.Fatalf("OnCallEnded chamado 2x: %+v", r)
	case <-time.After(100 * time.Millisecond):
	}
}
