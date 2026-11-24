package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Renomear uma sessão (o nome que aparece no painel e nos avisos do Chatwoot).

const maxSessionName = 60

var errRenameScoped = errors.New("esta sessão ainda depende do nome para saber de qual conta/caixa é (padrão inboxN_accM do AstraChat); conecte o Chatwoot antes de renomear")

// displayName devolve o nome atual da sessão.
func (s *Session) displayName() string {
	s.nameMu.RLock()
	defer s.nameMu.RUnlock()
	return s.name
}

func (s *Session) setName(name string) {
	s.nameMu.Lock()
	s.name = name
	s.nameMu.Unlock()
}

// cleanSessionName normaliza o nome digitado; "" se ficar vazio.
func cleanSessionName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	for utf8.RuneCountInString(name) > maxSessionName {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return strings.TrimSpace(name)
}

// Rename troca o nome da sessão. Uma sessão criada pelo AstraChat com nome
// "inboxN_accM" e ainda sem Chatwoot usa esse nome para o escopo do QR: não pode
// ser renomeada até o Chatwoot estar configurado.
func (m *SessionManager) Rename(ctx context.Context, id, name string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no session %s", id)
	}
	if acct, _ := scopeFromName("", s.displayName()); acct != 0 && !s.getChatwoot().valid() {
		return errRenameScoped
	}
	if m.store != nil && m.store.db != nil {
		if err := m.store.setName(ctx, id, name); err != nil {
			return err
		}
	}
	s.setName(name)
	if m.broker != nil {
		m.broker.emitSessionList(m.infos())
	}
	return nil
}

// PATCH /api/sessions/{sid}  {name}
func (s *server) handleSessionRename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	name := cleanSessionName(body.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if err := s.sessions.Rename(r.Context(), r.PathValue("sid"), name); err != nil {
		code := http.StatusNotFound
		if errors.Is(err, errRenameScoped) {
			code = http.StatusConflict
		} else if !strings.HasPrefix(err.Error(), "no session") {
			code = http.StatusInternalServerError
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": r.PathValue("sid"), "name": name})
}
