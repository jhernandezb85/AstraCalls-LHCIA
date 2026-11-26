package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Inbox excluída no Chatwoot -> número desvinculado; sessão apagada 24 h depois.
//
// O Chatwoot não avisa quando uma inbox é excluída. O AstraCalls confere:
//   - na hora, quando uma entrega ao Chatwoot falha (no máximo 1x a cada 30 min
//     por sessão), e
//   - de reserva, a cada 12 h, para números sem movimento.
//
// Só um 404 conta (rede, 5xx, 401 não), confirmado por uma segunda consulta
// alguns minutos depois, e só para inboxes que o AstraCalls JÁ VIU existir (uma
// sessão com config antiga nunca é tocada). Confirmado: desvincula o número do
// WhatsApp (para de receber e de gerar erros) e marca a sessão como órfã. A
// sessão (config, chaves, histórico) fica 24 h esperando: se nesse tempo for
// religada a uma inbox nova (provisioner → "Recuperar", com QR), segue igual.
// Passadas 24 h, é apagada por completo. Tudo fica no banco: sobrevive a restart.

var (
	orphanSweepEvery   = 12 * time.Hour
	orphanPurgeEvery   = time.Hour // só consulta o banco local
	orphanGrace        = 24 * time.Hour
	orphanConfirmAfter = 3 * time.Minute
	orphanSoonEvery    = 30 * time.Minute
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

func inboxWatchKey(c ChatwootConfig) string {
	return fmt.Sprintf("%s|%d|%d", c.URL, c.AccountID, c.InboxID)
}

// ---------- banco ----------

type inboxWatch struct {
	key         string
	seen        bool
	orphanSince int64 // ms; 0 = não órfã
}

func (st *sessionStore) getInboxWatch(ctx context.Context, sid string) (inboxWatch, bool) {
	var w inboxWatch
	var since sql.NullInt64
	err := st.db.QueryRowContext(ctx, `SELECT cfg_key, seen, orphan_since FROM chatwoot_inbox_watch WHERE session_id = $1`, sid).
		Scan(&w.key, &w.seen, &since)
	if err != nil {
		return inboxWatch{}, false
	}
	w.orphanSince = since.Int64
	return w, true
}

func (st *sessionStore) setInboxSeen(ctx context.Context, sid, key string) error {
	_, err := st.db.ExecContext(ctx, `INSERT INTO chatwoot_inbox_watch (session_id, cfg_key, seen, orphan_since)
		VALUES ($1, $2, true, NULL)
		ON CONFLICT (session_id) DO UPDATE SET cfg_key = EXCLUDED.cfg_key, seen = true, orphan_since = NULL`, sid, key)
	return err
}

func (st *sessionStore) setInboxOrphan(ctx context.Context, sid, key string, since int64) error {
	_, err := st.db.ExecContext(ctx, `UPDATE chatwoot_inbox_watch SET orphan_since = $3
		WHERE session_id = $1 AND cfg_key = $2 AND seen`, sid, key, since)
	return err
}

// resetInboxWatch: a sessão foi ligada a outra inbox (recuperada): começa do zero.
func (st *sessionStore) resetInboxWatch(ctx context.Context, sid, key string) error {
	_, err := st.db.ExecContext(ctx, `INSERT INTO chatwoot_inbox_watch (session_id, cfg_key, seen, orphan_since)
		VALUES ($1, $2, false, NULL)
		ON CONFLICT (session_id) DO UPDATE SET cfg_key = EXCLUDED.cfg_key, seen = false, orphan_since = NULL`, sid, key)
	return err
}

func (st *sessionStore) orphansBefore(ctx context.Context, before int64) ([]string, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT session_id FROM chatwoot_inbox_watch
		WHERE orphan_since IS NOT NULL AND orphan_since < $1`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// orphanRow é uma sessão em espera (para o provisioner oferecer "Recuperar").
type orphanRow struct {
	SessionID   string `json:"sessionId"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	ChatwootURL string `json:"chatwootUrl"`
	AccountID   int    `json:"accountId"`
	OrphanSince int64  `json:"orphanSince"`
	PurgeAt     int64  `json:"purgeAt"`
}

func (st *sessionStore) listOrphans(ctx context.Context) (map[string]int64, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT session_id, orphan_since FROM chatwoot_inbox_watch WHERE orphan_since IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var since int64
		if rows.Scan(&id, &since) == nil {
			out[id] = since
		}
	}
	return out, rows.Err()
}

// inboxWatchStore: o que a verificação usa do banco (trocável nos testes).
type inboxWatchStore interface {
	getInboxWatch(ctx context.Context, sid string) (inboxWatch, bool)
	setInboxSeen(ctx context.Context, sid, key string) error
	setInboxOrphan(ctx context.Context, sid, key string, since int64) error
	orphansBefore(ctx context.Context, before int64) ([]string, error)
}

var orphanStore = func(m *SessionManager) inboxWatchStore {
	if m.store == nil {
		return nil
	}
	return m.store
}

// ---------- verificação ----------

var (
	soonMu   sync.Mutex
	soonLast = map[string]time.Time{}
)

// inboxCheckSoon: uma entrega ao Chatwoot falhou — confere se a inbox ainda
// existe (no máximo 1x a cada orphanSoonEvery por sessão).
func (s *Session) inboxCheckSoon() {
	soonMu.Lock()
	if t, ok := soonLast[s.id]; ok && time.Since(t) < orphanSoonEvery {
		soonMu.Unlock()
		return
	}
	soonLast[s.id] = time.Now()
	soonMu.Unlock()
	if s.mgr != nil {
		go s.mgr.checkInbox(s.mgr.appCtx, s)
	}
}

// trocáveis nos testes
var (
	orphanSleep  func(ctx context.Context, d time.Duration) bool
	orphanLogout func(m *SessionManager, ctx context.Context, id string) error
	orphanDelete func(m *SessionManager, ctx context.Context, id string) error
)

func init() { // em init() para não criar ciclo de inicialização com os métodos
	orphanSleep = sleepCtx
	orphanLogout = func(m *SessionManager, ctx context.Context, id string) error { return m.Logout(ctx, id) }
	orphanDelete = func(m *SessionManager, ctx context.Context, id string) error { return m.Delete(ctx, id) }
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// checkInbox confere a inbox de uma sessão e, se ela foi excluída (404
// confirmado, e já vista existindo), desvincula o número e marca como órfã.
func (m *SessionManager) checkInbox(ctx context.Context, s *Session) {
	st := orphanStore(m)
	if st == nil {
		return
	}
	cfg := s.getChatwoot()
	if !cfg.valid() {
		return
	}
	key := inboxWatchKey(cfg)
	ok, err := cfg.inboxExists()
	if err != nil {
		return
	}
	if ok {
		if w, found := st.getInboxWatch(ctx, s.id); !found || !w.seen || w.key != key || w.orphanSince != 0 {
			_ = st.setInboxSeen(ctx, s.id, key)
		}
		return
	}
	w, found := st.getInboxWatch(ctx, s.id)
	if !found || !w.seen || w.key != key || w.orphanSince != 0 {
		return // nunca vista (config antiga) ou já órfã
	}
	// confirma alguns minutos depois (não reage a um 404 passageiro)
	if !orphanSleep(ctx, orphanConfirmAfter) {
		return
	}
	if cfg2 := s.getChatwoot(); inboxWatchKey(cfg2) != key {
		return // foi religada a outra inbox nesse meio-tempo
	}
	if ok, err := cfg.inboxExists(); err != nil || ok {
		return
	}
	now := time.Now().UnixMilli()
	if err := st.setInboxOrphan(ctx, s.id, key, now); err != nil {
		m.log.Error("chatwoot: marcar sessão órfã falhou", "session", s.id, "err", err)
		return
	}
	m.log.Warn("chatwoot: inbox excluída no Chatwoot; número desvinculado, sessão apagada em 24 h se não for recuperada",
		"session", s.id, "inbox", cfg.InboxID)
	if err := orphanLogout(m, ctx, s.id); err != nil {
		m.log.Warn("chatwoot: desvincular sessão órfã falhou", "session", s.id, "err", err)
	}
}

// purgeOrphans apaga as sessões órfãs há mais de orphanGrace.
func (m *SessionManager) purgeOrphans(ctx context.Context) {
	st := orphanStore(m)
	if st == nil {
		return
	}
	ids, err := st.orphansBefore(ctx, time.Now().Add(-orphanGrace).UnixMilli())
	if err != nil {
		return
	}
	for _, id := range ids {
		if _, ok := m.Get(id); !ok {
			continue
		}
		m.log.Warn("chatwoot: sessão órfã há mais de 24 h; apagando", "session", id)
		if err := orphanDelete(m, ctx, id); err != nil {
			m.log.Error("chatwoot: apagar sessão órfã falhou", "session", id, "err", err)
		}
	}
}

func (m *SessionManager) runOrphanInboxCheck(ctx context.Context) {
	sweep := time.NewTicker(orphanSweepEvery)
	purge := time.NewTicker(orphanPurgeEvery)
	defer sweep.Stop()
	defer purge.Stop()
	// ao subir: registra as inboxes que existem (sem esperar 12 h) e limpa vencidas
	m.sweepInboxes(ctx)
	m.purgeOrphans(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-sweep.C:
			m.sweepInboxes(ctx)
		case <-purge.C:
			m.purgeOrphans(ctx)
		}
	}
}

func (m *SessionManager) sweepInboxes(ctx context.Context) {
	m.mu.RLock()
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	m.mu.RUnlock()
	for _, s := range list {
		go m.checkInbox(ctx, s)
	}
}

// GET /api/orphans — sessões em espera (inbox excluída), com prazo de exclusão.
func (s *server) handleListOrphans(w http.ResponseWriter, r *http.Request) {
	m := s.sessions
	orphans, err := m.store.listOrphans(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	infos := map[string]SessionInfo{}
	for _, in := range m.infos() {
		infos[in.ID] = in
	}
	out := []orphanRow{}
	for id, since := range orphans {
		sess, ok := m.Get(id)
		if !ok {
			continue
		}
		cfg := sess.getChatwoot()
		in := infos[id]
		jid := in.JID
		if jid == "" {
			jid = in.LastJID
		}
		out = append(out, orphanRow{
			SessionID: id, Name: in.Name, Phone: phoneFromJIDString(jid),
			ChatwootURL: cfg.URL, AccountID: cfg.AccountID,
			OrphanSince: since, PurgeAt: since + orphanGrace.Milliseconds(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"orphans": out})
}

func phoneFromJIDString(jid string) string {
	for i, c := range jid {
		if c == ':' || c == '@' {
			return jid[:i]
		}
	}
	return jid
}
