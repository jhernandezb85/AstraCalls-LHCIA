package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Inbox apagada no Chatwoot -> sessão apagada no AstraCalls.
//
// O Chatwoot não avisa quando uma inbox é excluída. A cada orphanCheckEvery o
// AstraCalls confere se a inbox de cada sessão ainda existe. Só um 404 do
// Chatwoot conta (erro de rede, 5xx, 401 de token trocado NÃO contam), e só
// depois de orphanStrikes 404 seguidos — uma queda ou um deploy do Chatwoot
// nunca apaga nada. Aí desvincula o número e apaga a sessão (como o botão
// "Excluir" do painel). E só para inboxes que o AstraCalls VIU existir desde que
// subiu: uma sessão com config antiga (inbox apagada há tempo, número ainda em
// uso pela API) nunca é apagada por causa desta verificação.

var (
	orphanCheckEvery = 10 * time.Minute
	orphanStrikes    = 3
)

// inboxExists: (true, nil) existe; (false, nil) o Chatwoot respondeu 404; erro
// quando não deu para saber.
func (c ChatwootConfig) inboxExists() (bool, error) {
	_, code, err := c.req(http.MethodGet, fmt.Sprintf("/inboxes/%d", c.InboxID), nil)
	if err != nil {
		return false, err
	}
	switch {
	case code == http.StatusNotFound:
		return false, nil
	case code >= 200 && code < 300:
		return true, nil
	default:
		return false, fmt.Errorf("inbox http %d", code)
	}
}

type orphanState struct {
	strikes map[string]int
	seen    map[string]bool // a inbox respondeu que existe ao menos uma vez
}

func newOrphanState() *orphanState {
	return &orphanState{strikes: map[string]int{}, seen: map[string]bool{}}
}

func (m *SessionManager) runOrphanInboxCheck(ctx context.Context) {
	st := newOrphanState()
	t := time.NewTicker(orphanCheckEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.checkOrphanInboxes(ctx, st)
		}
	}
}

// deleteSession é trocável nos testes.
var deleteOrphanSession = func(m *SessionManager, ctx context.Context, id string) error { return m.Delete(ctx, id) }

func (m *SessionManager) checkOrphanInboxes(ctx context.Context, st *orphanState) {
	strikes := st.strikes
	m.mu.RLock()
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	m.mu.RUnlock()
	alive := map[string]bool{}
	for _, s := range list {
		alive[s.id] = true
		cfg := s.getChatwoot()
		if !cfg.valid() {
			delete(strikes, s.id)
			continue
		}
		key := fmt.Sprintf("%s|%s|%d|%d", s.id, cfg.URL, cfg.AccountID, cfg.InboxID)
		ok, err := cfg.inboxExists()
		if ok {
			st.seen[key] = true
		}
		if err != nil || ok || !st.seen[key] {
			delete(strikes, s.id)
			continue
		}
		strikes[s.id]++
		m.log.Warn("chatwoot: a inbox da sessão não existe mais", "session", s.id, "inbox", cfg.InboxID, "strike", strikes[s.id], "of", orphanStrikes)
		if strikes[s.id] < orphanStrikes {
			continue
		}
		delete(strikes, s.id)
		m.log.Warn("chatwoot: inbox excluída no Chatwoot; apagando a sessão", "session", s.id, "inbox", cfg.InboxID)
		if err := deleteOrphanSession(m, ctx, s.id); err != nil {
			m.log.Error("chatwoot: apagar sessão órfã falhou", "session", s.id, "err", err)
		}
	}
	for id := range strikes {
		if !alive[id] {
			delete(strikes, id)
		}
	}
	for k := range st.seen {
		if id, _, _ := strings.Cut(k, "|"); !alive[id] {
			delete(st.seen, k)
		}
	}
}
