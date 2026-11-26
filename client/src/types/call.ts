export type CallStatus = "starting" | "ringing" | "connected" | "ended";

export type CallSummary = {
  sessionId: string;
  callId: string;
  owner: string | null;
  direction: "outbound" | "inbound";
  peer: string;
  startedAt: number;
  status: CallStatus;
  held?: boolean;
};

export type IncomingPayload = {
  sessionId: string;
  callId: string;
  peer: string;
  /** Número y nombre de quien llama, resueltos por el servidor. */
  phone?: string;
  name?: string;
  /** Línea (cuenta) por la que entra la llamada. */
  lineName?: string;
  linePhone?: string;
  video: boolean;
  offeredAt: number;
};

export type TransferOfferPayload = { sessionId: string; callId: string; peer: string; from: string; offeredAt: number };
