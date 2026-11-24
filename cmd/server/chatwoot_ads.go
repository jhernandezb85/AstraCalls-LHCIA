package main

import (
	"fmt"
	"net/http"
	"strings"
)

// Dados do anúncio (Click to WhatsApp) nos atributos da CONVERSA do Chatwoot
// (ad_attributes): ficam estruturados para automações/n8n enviarem o evento ao
// Meta (CAPI) sem ler a nota privada. Os atributos precisam estar definidos na
// conta para aparecerem no painel (o provisioner os cria ao ligar a opção).

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
