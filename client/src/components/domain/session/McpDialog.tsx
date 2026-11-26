import { useCallback, useEffect, useState } from "react";
import { Bot, Loader2, Plus, Trash2, ArrowLeft, Copy, Check, KeyRound } from "lucide-react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  listMcpKeys,
  createMcpKey,
  deleteMcpKey,
  listAdminMcpKeys,
  createAdminMcpKey,
  deleteAdminMcpKey,
  type McpKey,
  type McpKeyCreated,
} from "@/services/mcp";

const fmtDate = (iso?: string) =>
  iso ? new Date(iso).toLocaleString("es-PE", { dateStyle: "short", timeStyle: "short" }) : "nunca";

// Campo de solo lectura con botón de copiar.
export const CopyField = ({ label, value, multiline }: { label: string; value: string; multiline?: boolean }) => {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error("No se pudo copiar");
    }
  };
  return (
    <div className="min-w-0 space-y-1">
      <Label>{label}</Label>
      <div className="flex min-w-0 items-start gap-2">
        {multiline ? (
          <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-md border bg-muted/50 p-2 text-xs">{value}</pre>
        ) : (
          <Input readOnly value={value} className="min-w-0 flex-1 font-mono text-xs" onFocus={(e) => e.target.select()} />
        )}
        <Button type="button" size="icon" variant="outline" className="shrink-0" title="Copiar" onClick={copy}>
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
};

type View = "list" | "new" | "created";

// Gestión de llaves MCP. Sin sid = llaves de administrador (todas las cuentas).
export const McpKeys = ({ sid, onCount }: { sid?: string; onCount?: (n: number) => void }) => {
  const admin = !sid;
  const [busy, setBusy] = useState(false);
  const [keys, setKeys] = useState<McpKey[]>([]);
  const [max, setMax] = useState(20);
  const [view, setView] = useState<View>("list");
  const [name, setName] = useState("");
  const [readOnly, setReadOnly] = useState(false);
  const [created, setCreated] = useState<McpKeyCreated | null>(null);

  const load = useCallback(
    () =>
      (admin ? listAdminMcpKeys() : listMcpKeys(sid!))
        .then((r) => {
          setKeys(r.keys ?? []);
          setMax(r.max || 20);
          onCount?.((r.keys ?? []).length);
        })
        .catch(() => {}),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [sid, admin],
  );

  useEffect(() => {
    load();
  }, [load]);

  const startNew = () => {
    setName("");
    setReadOnly(false);
    setView("new");
  };

  const create = async () => {
    setBusy(true);
    try {
      const body = { name: name.trim(), readOnly };
      const r = admin ? await createAdminMcpKey(body) : await createMcpKey(sid!, body);
      setCreated(r);
      setView("created");
      await load();
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (k: McpKey) => {
    if (!window.confirm(`¿Revocar la llave ${k.name || k.prefix}? Quien la esté usando pierde el acceso de inmediato.`)) return;
    setBusy(true);
    try {
      await (admin ? deleteAdminMcpKey(k.id) : deleteMcpKey(sid!, k.id));
      toast.success("Llave revocada");
      await load();
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  if (view === "new") {
    return (
      <div className="space-y-3">
        <p className="text-sm font-semibold">{admin ? "Nueva llave de administrador" : "Nueva llave MCP"}</p>
        <div className="space-y-1">
          <Label>Nombre</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="ej.: Claude de ventas, n8n" />
        </div>
        <label className="flex cursor-pointer items-start justify-between gap-4 rounded-lg border p-3">
          <span className="space-y-0.5">
            <span className="block text-sm font-medium">Solo lectura</span>
            <span className="block text-xs text-muted-foreground">
              El agente puede leer conversaciones, buscar mensajes y consultar contactos y grupos, pero no envía ni cambia nada.
            </span>
          </span>
          <Switch checked={readOnly} onCheckedChange={setReadOnly} aria-label="Solo lectura" />
        </label>
        <p className="text-xs text-muted-foreground">
          {admin
            ? "La llave da acceso a TODOS los números de este servidor, incluida la API completa (api_request). Guárdala como la clave maestra."
            : "La llave solo da acceso a este número. No permite vincular, desconectar ni tocar webhooks, Chatwoot o SIP."}
        </p>
        <div className="flex flex-wrap justify-between gap-2 pt-1">
          <Button variant="outline" disabled={busy} onClick={() => setView("list")}>
            <ArrowLeft className="h-4 w-4" />
            Volver
          </Button>
          <Button disabled={busy} onClick={create}>
            {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Crear llave
          </Button>
        </div>
      </div>
    );
  }

  if (view === "created" && created) {
    return (
      <div className="min-w-0 space-y-4">
        <p className="text-sm font-semibold">Llave creada</p>
        <p className="flex items-start gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
          <KeyRound className="mt-0.5 h-4 w-4 shrink-0" />
          Cópiala ahora: la llave no se volverá a mostrar. Si la pierdes, revócala y crea otra.
        </p>
        <CopyField label="URL completa (claude.ai, ChatGPT y clientes que solo piden la URL)" value={created.url} />
        <CopyField label="Dirección MCP" value={created.endpoint} />
        <CopyField label="Llave (Authorization: Bearer)" value={created.secret} />
        <CopyField
          label="Claude Code"
          multiline
          value={`claude mcp add --transport http astracalls ${created.endpoint} \\\n  --header "Authorization: Bearer ${created.secret}"`}
        />
        <CopyField
          label="Claude Desktop, Cursor, VS Code (mcpServers)"
          multiline
          value={JSON.stringify(
            {
              mcpServers: {
                astracalls: {
                  type: "http",
                  url: created.endpoint,
                  headers: { Authorization: `Bearer ${created.secret}` },
                },
              },
            },
            null,
            2,
          )}
        />
        <p className="text-xs text-muted-foreground">
          En n8n: herramienta <strong>MCP Client</strong> del agente de IA, con la dirección de arriba y autenticación Bearer.
        </p>
        <div className="flex justify-end">
          <Button onClick={() => setView("list")}>Listo</Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {keys.length === 0 && (
        <p className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
          {admin
            ? "No hay llaves de administrador. Crea una para conectar un agente de IA a todos los números."
            : "Esta sesión no tiene llaves MCP. Crea una para conectar un agente de IA a este número."}
        </p>
      )}
      {keys.map((k) => (
        <div key={k.id} className="flex items-start justify-between gap-2 rounded-lg border p-3">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{k.name || "Llave MCP"}</span>
              <Badge variant={k.readOnly ? "muted" : "secondary"}>{k.readOnly ? "Solo lectura" : "Acceso completo"}</Badge>
            </div>
            <p className="font-mono text-xs text-muted-foreground">{k.prefix}…</p>
            <p className="text-xs text-muted-foreground">
              Creada {fmtDate(k.createdAt)} · último uso {fmtDate(k.lastUsedAt)}
            </p>
          </div>
          <Button size="icon" variant="ghost" disabled={busy} title="Revocar" onClick={() => remove(k)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ))}
      <div className="flex items-center justify-between gap-2 pt-1">
        <span className="text-xs text-muted-foreground">
          {keys.length}/{max}
        </span>
        <Button disabled={busy || keys.length >= max} onClick={startNew}>
          <Plus className="h-4 w-4" />
          Nueva llave
        </Button>
      </div>
    </div>
  );
};

// Botón + diálogo MCP de una cuenta.
export const McpDialog = ({ sid }: { sid: string }) => {
  const [open, setOpen] = useState(false);
  const [count, setCount] = useState(0);

  useEffect(() => {
    listMcpKeys(sid)
      .then((r) => setCount((r.keys ?? []).length))
      .catch(() => {});
  }, [sid]);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant={count ? "default" : "outline"} size="sm" title="MCP (agentes de IA)">
          <Bot className="h-4 w-4" />
          <span className="hidden sm:inline">MCP{count > 1 ? ` (${count})` : ""}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto [&>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>MCP — agentes de IA</DialogTitle>
          <DialogDescription>
            Conecta Claude, ChatGPT, Cursor, n8n y otros agentes de IA a este número por MCP: envían mensajes, leen
            conversaciones, hacen llamadas y gestionan grupos.
          </DialogDescription>
        </DialogHeader>
        {open && <McpKeys sid={sid} onCount={setCount} />}
      </DialogContent>
    </Dialog>
  );
};
