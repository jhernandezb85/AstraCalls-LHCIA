import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Bot, Unlink, Server, KeyRound, BookOpen, Clock } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { McpKeys } from "@/components/domain/session/McpDialog";
import { apiGet } from "@/lib/api";
import { EXTENSION_DOWNLOAD_URL } from "@/lib/passkey";
import { cn } from "@/lib/utils";
import { useSessions } from "@/stores/sessions";

type Tab = "mcp" | "orphans" | "system";

const TABS: { id: Tab; label: string; icon: ReactNode }[] = [
  { id: "mcp", label: "MCP", icon: <Bot className="h-4 w-4" /> },
  { id: "orphans", label: "Números desvinculados", icon: <Unlink className="h-4 w-4" /> },
  { id: "system", label: "Sistema", icon: <Server className="h-4 w-4" /> },
];

const TAB_KEY = "astracalls.settingsTab";

const loadTab = (): Tab => {
  try {
    const t = localStorage.getItem(TAB_KEY) as Tab | null;
    if (t && TABS.some((x) => x.id === t)) return t;
  } catch {
    /* sin almacenamiento */
  }
  return "mcp";
};

export const SettingsPage = () => {
  const [tab, setTab] = useState<Tab>(loadTab);
  useEffect(() => {
    try {
      localStorage.setItem(TAB_KEY, tab);
    } catch {
      /* sin almacenamiento */
    }
  }, [tab]);

  return (
    <div className="flex flex-col gap-6 md:flex-row">
      <nav className="flex shrink-0 gap-1 overflow-x-auto md:w-56 md:flex-col" aria-label="Secciones de configuración">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              "flex shrink-0 items-center gap-2 rounded-xl px-3 py-2 text-left text-sm transition-colors",
              tab === t.id ? "bg-accent font-medium text-accent-foreground" : "text-foreground/70 hover:bg-muted",
            )}
          >
            {t.icon}
            {t.label}
          </button>
        ))}
      </nav>
      <div className="min-w-0 flex-1">
        {tab === "mcp" && <McpTab />}
        {tab === "orphans" && <OrphansTab />}
        {tab === "system" && <SystemTab />}
      </div>
    </div>
  );
};

// ---------- MCP ----------
const McpTab = () => (
  <Card>
    <CardHeader>
      <CardTitle>Llave MCP de administrador</CardTitle>
      <CardDescription>
        Una sola conexión MCP para todas las cuentas (números) de este servidor: el agente elige el número en cada acción.
        Usa solo lectura para analizar conversaciones sin riesgo de enviar nada. Las llaves de un solo número se crean en
        el botón MCP de cada cuenta.
      </CardDescription>
    </CardHeader>
    <CardContent>
      <McpKeys />
    </CardContent>
  </Card>
);

// ---------- Números desvinculados ----------
type Orphan = {
  sessionId: string;
  name: string;
  phone: string;
  chatwootUrl: string;
  accountId: number;
  orphanSince: number;
  purgeAt: number;
};

const fmtDateTime = (ms: number) =>
  new Date(ms).toLocaleString("es-PE", { dateStyle: "short", timeStyle: "short" });

const fmtLeft = (ms: number) => {
  if (ms <= 0) return "en cualquier momento";
  const h = Math.floor(ms / 3_600_000);
  const m = Math.floor((ms % 3_600_000) / 60_000);
  return h > 0 ? `${h} h ${m} min` : `${m} min`;
};

const useNow = (every: number) => {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), every);
    return () => clearInterval(t);
  }, [every]);
  return now;
};

const OrphansTab = () => {
  const now = useNow(30_000);
  const { data, isLoading, isError } = useQuery({
    queryKey: ["orphans"],
    queryFn: () => apiGet<{ orphans: Orphan[] }>("/api/orphans"),
    refetchInterval: 60_000,
  });
  const rows = [...(data?.orphans ?? [])].sort((a, b) => a.purgeAt - b.purgeAt);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Números desvinculados</CardTitle>
        <CardDescription>
          Cuando se borra la bandeja de Chatwoot de una cuenta, AstraCalls desvincula el número de WhatsApp (el
          dispositivo vinculado se cierra) y guarda la cuenta 24 horas. Pasado ese plazo se borra junto con su historial
          y ya no es recuperable.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {isLoading && <p className="text-sm text-muted-foreground">Cargando…</p>}
        {isError && <p className="text-sm text-destructive">No se pudo cargar la lista.</p>}
        {!isLoading && !isError && rows.length === 0 && (
          <p className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
            No hay números desvinculados.
          </p>
        )}
        {rows.map((o) => {
          const left = o.purgeAt - now;
          return (
            <div key={o.sessionId} className="flex flex-wrap items-start justify-between gap-3 rounded-lg border p-3">
              <div className="min-w-0 space-y-1">
                <p className="text-sm font-medium">{o.name || o.sessionId}</p>
                {o.phone && <p className="font-mono text-xs text-muted-foreground">+{o.phone}</p>}
                <p className="truncate text-xs text-muted-foreground">
                  {o.chatwootUrl ? `${o.chatwootUrl} · cuenta ${o.accountId}` : "Sin Chatwoot"}
                </p>
                <p className="text-xs text-muted-foreground">Desvinculado el {fmtDateTime(o.orphanSince)}</p>
              </div>
              <Badge variant={left < 3 * 3_600_000 ? "destructive" : "secondary"} className="gap-1">
                <Clock className="h-3 w-3" />
                Se borra en {fmtLeft(left)}
              </Badge>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
};

// ---------- Sistema ----------
type ServerConfig = {
  version?: string;
  commit?: string;
  startedAt?: number;
  maxCallsPerSession?: number;
  defaultTransport?: string;
  groupCalls?: boolean;
  publicIp?: string;
  publicIpSource?: string;
};

const fmtUptime = (ms: number) => {
  const d = Math.floor(ms / 86_400_000);
  const h = Math.floor((ms % 86_400_000) / 3_600_000);
  const m = Math.floor((ms % 3_600_000) / 60_000);
  if (d > 0) return `${d} d ${h} h`;
  if (h > 0) return `${h} h ${m} min`;
  return `${m} min`;
};

const Row = ({ label, children }: { label: string; children: ReactNode }) => (
  <div className="flex flex-wrap items-baseline justify-between gap-2 border-b py-2 last:border-0">
    <span className="text-sm text-muted-foreground">{label}</span>
    <span className="min-w-0 break-all text-right text-sm">{children}</span>
  </div>
);

const SystemTab = () => {
  const now = useNow(60_000);
  const sessions = useSessions((s) => s.sessions);
  const { data, isError } = useQuery({
    queryKey: ["config-system"],
    queryFn: () => apiGet<ServerConfig>("/api/config"),
    refetchInterval: 60_000,
  });
  const connected = sessions.filter((s) => s.state === "open").length;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Servidor</CardTitle>
        </CardHeader>
        <CardContent>
          <Row label="Estado">
            {isError ? <Badge variant="destructive">Sin respuesta</Badge> : <Badge variant="success">En línea</Badge>}
          </Row>
          <Row label="Versión">
            <span className="font-mono">{data?.version || "—"}</span>
          </Row>
          <Row label="Commit">
            <span className="font-mono">{data?.commit ? data.commit.slice(0, 7) : "—"}</span>
          </Row>
          <Row label="Encendido hace">{data?.startedAt ? fmtUptime(now - data.startedAt) : "—"}</Row>
          <Row label="Cuentas">
            {connected} conectadas de {sessions.length}
          </Row>
          <Row label="Llamadas a la vez por cuenta">{data?.maxCallsPerSession ?? "—"}</Row>
          <Row label="Transporte de audio por defecto">{data?.defaultTransport || "WebRTC"}</Row>
          <Row label="Llamadas grupales">{data?.groupCalls ? "Activadas" : "Desactivadas"}</Row>
          {data?.publicIp && (
            <Row label="IP pública">
              <span className="font-mono">{data.publicIp}</span>
              {data.publicIpSource && <span className="text-muted-foreground"> ({data.publicIpSource})</span>}
            </Row>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Recursos</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button variant="outline" asChild>
            <a href="/api-docs.html" target="_blank" rel="noopener noreferrer">
              <BookOpen className="h-4 w-4" />
              Documentación de la API
            </a>
          </Button>
          <Button variant="outline" asChild>
            <a href={EXTENSION_DOWNLOAD_URL} download title="Solo hace falta para conectar cuentas protegidas con passkey">
              <KeyRound className="h-4 w-4" />
              Extensión Passkey (.zip)
            </a>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
};
