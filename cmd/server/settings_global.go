package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Configurações globais do AstraCalls (painel → "Configurações").
//
// Por enquanto: as chaves MCP de ADMINISTRADOR, que dão acesso MCP a TODAS as
// sessões sem usar a chave-mestra (WACALLS_API_KEY). Ficam na mesma tabela das
// chaves de sessão, com session_id = adminMCPSession; só o hash é salvo e a chave
// aparece uma vez. Podem ser somente leitura (ex.: analisar conversas).

const (
	adminMCPSession = "*"
	adminMCPMax     = 5
)

// GET /api/settings/mcp-keys
func (s *server) handleListAdminMCPKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.sessions.store.listMCPKeys(r.Context(), adminMCPSession)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys, "max": adminMCPMax, "endpoint": mcpEndpoints(r)})
}

// POST /api/settings/mcp-keys {name, readOnly}
func (s *server) handleCreateAdminMCPKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		ReadOnly bool   `json:"readOnly"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	name := strings.TrimSpace(body.Name)
	if len([]rune(name)) > mcpKeyNameMaxLen {
		name = string([]rune(name)[:mcpKeyNameMaxLen])
	}
	existing, err := s.sessions.store.listMCPKeys(r.Context(), adminMCPSession)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if len(existing) >= adminMCPMax {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "limite de chaves de administrador atingido"})
		return
	}
	k, key, err := s.sessions.store.createMCPKey(r.Context(), adminMCPSession, name, body.ReadOnly)
	if errors.Is(err, errMCPKeyLimit) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ep := mcpEndpoints(r)
	writeJSON(w, http.StatusCreated, map[string]any{"key": k, "secret": key, "endpoint": ep, "url": ep + "/" + key})
}

// DELETE /api/settings/mcp-keys/{kid}
func (s *server) handleDeleteAdminMCPKey(w http.ResponseWriter, r *http.Request) {
	ok, err := s.sessions.store.deleteMCPKey(r.Context(), adminMCPSession, r.PathValue("kid"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "chave não encontrada"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (st *sessionStore) createAdminMCPKey(ctx context.Context, name string, readOnly bool) (mcpKey, string, error) {
	key, hash := newMCPKey()
	k := mcpKey{ID: genSIPCredential(8), SessionID: adminMCPSession, Name: name, Prefix: key[:len(mcpKeyPrefix)+6], ReadOnly: readOnly}
	err := st.db.QueryRowContext(ctx, `INSERT INTO mcp_admin_keys (id, name, key_hash, prefix, read_only)
		VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		k.ID, name, hash, k.Prefix, readOnly).Scan(&k.CreatedAt)
	return k, key, err
}
