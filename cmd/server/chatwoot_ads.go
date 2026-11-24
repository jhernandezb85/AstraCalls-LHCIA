package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const adAttrsCacheTTL = 10 * time.Minute

// Dados do anúncio (Click to WhatsApp) nos atributos da CONVERSA do Chatwoot:
// ficam estruturados para automações/n8n enviarem o evento ao Meta (CAPI) sem ler
// a nota privada. É uma opção da CONTA: vale quando a conta tem o atributo
// ctwa_clid definido (o provisioner o cria em Integrações). Só são gravados os
// atributos que a conta tiver: ctwa_clid e meta_ad_id (os da CAPI) e, se o
// administrador quis, anuncio_titulo, anuncio_origen e anuncio_enlace.

type adAttrsState struct {
	keys map[string]bool
	at   time.Time
}

var adAttrsCache sync.Map // "<url>|<conta>" -> adAttrsState

// definedAdKeys devolve os atributos de conversa definidos na conta (cache: 10
// min se a opção está ligada, 2 min se não; em erro, vazio e tenta de novo).
func (c ChatwootConfig) definedAdKeys() map[string]bool {
	key := fmt.Sprintf("%s|%d", c.URL, c.AccountID)
	if v, ok := adAttrsCache.Load(key); ok {
		st := v.(adAttrsState)
		ttl := adAttrsCacheTTL
		if !st.keys["ctwa_clid"] {
			ttl = 2 * time.Minute // recém-ligado na conta: percebe logo
		}
		if time.Since(st.at) < ttl {
			return st.keys
		}
	}
	res, code, err := c.reqList(http.MethodGet, "/custom_attribute_definitions?attribute_model=conversation_attribute")
	if err != nil || code != http.StatusOK {
		return nil
	}
	keys := map[string]bool{}
	for _, it := range res {
		if k := asStr(asMap(it)["attribute_key"]); k != "" {
			keys[k] = true
		}
	}
	adAttrsCache.Store(key, adAttrsState{keys: keys, at: time.Now()})
	return keys
}

// accountAdAttributes devolve os atributos do anúncio a gravar na conversa, só
// os que a conta tiver definidos; nada se a opção estiver desligada.
func (c ChatwootConfig) accountAdAttributes(ref map[string]any) map[string]any {
	keys := c.definedAdKeys()
	if !keys["ctwa_clid"] {
		return nil
	}
	out := map[string]any{}
	for k, v := range adAttributes(ref) {
		if keys[k] {
			out[k] = v
		}
	}
	return out
}

// adAttributes extrai do referral todos os atributos possíveis do anúncio.
func adAttributes(ref map[string]any) map[string]any {
	out := map[string]any{}
	if !referralIsAd(ref) { // link wa.me, busca etc.: não é anúncio, não vira atributo
		return out
	}
	set := func(key string, srcs ...string) {
		for _, src := range srcs {
			if v := strings.TrimSpace(asStr(ref[src])); v != "" {
				out[key] = v
				return
			}
		}
	}
	set("ctwa_clid", "ctwaClid")
	set("meta_ad_id", "sourceId")
	set("anuncio_titulo", "title")
	set("anuncio_origen", "utmSource", "sourceApp", "conversionSource")
	set("anuncio_enlace", "sourceUrl")
	return out
}

// mergeConversationAttrs grava attrs nos atributos da conversa SEM apagar os
// outros (o endpoint do Chatwoot substitui o conjunto inteiro).
func (c ChatwootConfig) mergeConversationAttrs(convID int, attrs map[string]any) error {
	if convID == 0 || len(attrs) == 0 {
		return nil
	}
	res, code, err := c.req(http.MethodGet, fmt.Sprintf("/conversations/%d", convID), nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("get conversation http %d", code)
	}
	merged := map[string]any{}
	for k, v := range asMap(res["custom_attributes"]) {
		merged[k] = v
	}
	for k, v := range attrs {
		merged[k] = v
	}
	_, code, err = c.req(http.MethodPost, fmt.Sprintf("/conversations/%d/custom_attributes", convID), map[string]any{"custom_attributes": merged})
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("custom_attributes http %d", code)
	}
	return nil
}

// reqList é o req para respostas que são uma lista JSON (ex.: definições de
// atributos). Sem corpo.
func (c ChatwootConfig) reqList(method, path string) ([]any, int, error) {
	req, err := http.NewRequest(method, c.base()+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("api_access_token", c.AccountToken)
	resp, err := cwHTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var out []any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode, nil
}
