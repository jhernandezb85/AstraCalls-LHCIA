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

type adAttrsState struct {
	on bool
	at time.Time
}

var adAttrsCache sync.Map // "<url>|<conta>" -> adAttrsState

// adAttributesDefined diz se a conta tem os atributos do anúncio definidos
// (cache: 10 min se ligado, 2 min se desligado; em erro, desligado e tenta de novo).
func (c ChatwootConfig) adAttributesDefined() bool {
	key := fmt.Sprintf("%s|%d", c.URL, c.AccountID)
	if v, ok := adAttrsCache.Load(key); ok {
		st := v.(adAttrsState)
		ttl := adAttrsCacheTTL
		if !st.on {
			ttl = 2 * time.Minute // recém-ligado na conta: percebe logo
		}
		if time.Since(st.at) < ttl {
			return st.on
		}
	}
	res, code, err := c.reqList(http.MethodGet, "/custom_attribute_definitions?attribute_model=conversation_attribute")
	if err != nil || code != http.StatusOK {
		return false
	}
	on := false
	for _, it := range res {
		if asStr(asMap(it)["attribute_key"]) == "ctwa_clid" {
			on = true
			break
		}
	}
	adAttrsCache.Store(key, adAttrsState{on: on, at: time.Now()})
	return on
}

// Dados do anúncio (Click to WhatsApp) nos atributos da CONVERSA do Chatwoot:
// ficam estruturados para automações/n8n enviarem o evento ao Meta (CAPI) sem ler
// a nota privada. É uma opção da CONTA: vale quando a conta tem o atributo
// ctwa_clid definido (o provisioner o cria em Integrações); sem ele, nada é gravado.

// adAttributes extrai do referral os atributos gravados na conversa.
func adAttributes(ref map[string]any) map[string]any {
	out := map[string]any{}
	set := func(key string, srcs ...string) {
		for _, src := range srcs {
			if v := strings.TrimSpace(asStr(ref[src])); v != "" {
				out[key] = v
				return
			}
		}
	}
	set("ctwa_clid", "ctwaClid")
	set("ad_id", "sourceId")
	set("ad_title", "title")
	set("ad_source", "utmSource", "sourceApp", "conversionSource")
	set("ad_url", "sourceUrl")
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
