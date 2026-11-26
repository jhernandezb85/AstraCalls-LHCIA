package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEditCheck(t *testing.T) {
	now := time.Now()
	text := json.RawMessage(`{"conversation":"hola"}`)
	ext := json.RawMessage(`{"extendedTextMessage":{"text":"hola"},"messageContextInfo":{}}`)
	img := json.RawMessage(`{"imageMessage":{"caption":"x"}}`)
	cases := []struct {
		name   string
		waID   string
		fromMe bool
		raw    json.RawMessage
		sent   time.Time
		text   string
		want   string
	}{
		{"ok texto", "W1", true, text, now.Add(-5 * time.Minute), "nuevo", ""},
		{"ok extendido", "W1", true, ext, now.Add(-14 * time.Minute), "nuevo", ""},
		{"no salió por WhatsApp", "", true, text, now, "nuevo", editNotSent},
		{"de otro", "W1", false, text, now, "nuevo", editNotOwn},
		{"adjunto", "W1", true, img, now, "nuevo", editNotText},
		{"más de 15 min", "W1", true, text, now.Add(-16 * time.Minute), "nuevo", editTooLate},
		{"sin fecha", "W1", true, text, time.Time{}, "nuevo", editTooLate},
		{"vacío", "W1", true, text, now, "  ", editBadInput},
	}
	for _, c := range cases {
		if got := editCheck(c.waID, c.fromMe, c.raw, c.sent, now, c.text); got != c.want {
			t.Errorf("%s: %q, quería %q", c.name, got, c.want)
		}
	}
}

// El endpoint de edición no es parte del widget: con la llave de widget, 401.
func TestEditRouteNotInWidgetSurface(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/sessions/s1/chatwoot/messages/5/edit", nil)
	if widgetAllowed(r) {
		t.Fatal("editar solo con la llave maestra")
	}
}
