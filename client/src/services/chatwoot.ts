import { apiGet, apiPost, apiDelete } from "@/lib/api";

export type ChatwootConfig = {
  url: string;
  account_id: number;
  account_token?: string;
  inbox_id: number;
  inbox_identifier: string;
  /** Reflete no Chatwoot (como nota privada) o que for enviado pela API. */
  mirror_api?: boolean;
  /** Importa o histórico de conversas ao conectar a conta (HistorySync). */
  import_history?: boolean;
  /** Janela do histórico a importar, em dias (0 = padrão). */
  import_history_days?: number;
  /** Como entram as mensagens escritas no aparelho: open (padrão), keep ou note. */
  device_mode?: "open" | "keep" | "note";
  /** Título acima das mensagens do aparelho. */
  device_title?: string;
  /** Token do Agent Bot que posta as mensagens do aparelho (nunca devolvido). */
  device_bot_token?: string;
  /** Recibos entregue/lida no Chatwoot (padrão: ligado). */
  read_receipts?: boolean;
  contact_delete?: 'note' | 'delete';
  ad_attributes?: boolean;
};

export const getChatwoot = (sid: string) =>
  apiGet<{ enabled: boolean; chatwoot: ChatwootConfig }>(`/api/sessions/${sid}/chatwoot`);

export const setChatwoot = (sid: string, cfg: ChatwootConfig) =>
  apiPost(`/api/sessions/${sid}/chatwoot`, cfg);

export const deleteChatwoot = (sid: string) => apiDelete(`/api/sessions/${sid}/chatwoot`);
