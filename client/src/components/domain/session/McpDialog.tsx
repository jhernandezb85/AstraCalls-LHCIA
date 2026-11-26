import { useCallback, useEffect, useState } from "react";
import { Bot, Loader2, Plus, Trash2, ArrowLeft, Copy, Check, KeyRound, Settings } from "lucide-react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
  DialogFooter,
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
  iso ? new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" }) : "nunca";

// Campo somente leitura com botão de copiar.
const CopyField = ({ label, value, multiline }: { label: string; value: string; multiline?: boolean }) => {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error("Não foi possível copiar");
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

// Sem sid = Configurações gerais: chaves MCP de administrador (todas as sessões).
export const McpDialog = ({ sid, className }: { sid?: string; className?: string }) => {
  const admin = !sid;
  const [open, setOpen] = useState(false);
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
        })
        .catch(() => {}),
    [sid, admin],
  );

  useEffect(() => {
    load();
  }, [load, open]);

  useEffect(() => {
    if (!open) {
      setView("list");
      setCreated(null);
    }
  }, [open]);

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
    if (!window.confirm(`Revogar a chave ${k.name || k.prefix}? Quem estiver usando perde o acesso na hora.`)) return;
    setBusy(true);
    try {
      await (admin ? deleteAdminMcpKey(k.id) : deleteMcpKey(sid!, k.id));
      toast.success("Chave revogada");
      await load();
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const list = (
    <div className="space-y-3">
      {admin && (
        <div className="space-y-1">
          <p className="text-sm font-semibold">Chave MCP de administrador</p>
          <p className="text-xs text-muted-foreground">
            Uma só conexão MCP para todas as contas (números) deste servidor: o agente escolhe o número em cada ação.
            Use somente leitura para analisar conversas sem risco de enviar nada.
          </p>
        </div>
      )}
      {keys.length === 0 && (
        <p className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
          {admin
            ? "Nenhuma chave de administrador. Crie uma para conectar um agente de IA a todos os números."
            : "Nenhuma chave MCP nesta sessão. Crie uma para conectar um agente de IA a este número."}
        </p>
      )}
      {keys.map((k) => (
        <div key={k.id} className="flex items-start justify-between gap-2 rounded-lg border p-3">
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{k.name || "Chave MCP"}</span>
              <Badge variant={k.readOnly ? "muted" : "secondary"}>{k.readOnly ? "Somente leitura" : "Acesso completo"}</Badge>
            </div>
            <p className="font-mono text-xs text-muted-foreground">{k.prefix}…</p>
            <p className="text-xs text-muted-foreground">
              Criada {fmtDate(k.createdAt)} · último uso {fmtDate(k.lastUsedAt)}
            </p>
          </div>
          <Button size="icon" variant="ghost" disabled={busy} title="Revogar" onClick={() => remove(k)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ))}
    </div>
  );

  const form = (
    <div className="space-y-3">
      <div className="space-y-1">
        <Label>Nome</Label>
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="ex.: Claude do comercial, n8n" />
      </div>
      <label className="flex cursor-pointer items-start justify-between gap-4 rounded-lg border p-3">
        <span className="space-y-0.5">
          <span className="block text-sm font-medium">Somente leitura</span>
          <span className="block text-xs text-muted-foreground">
            O agente pode ler conversas, buscar mensagens e consultar contatos e grupos, mas não envia nem altera nada.
          </span>
        </span>
        <Switch checked={readOnly} onCheckedChange={setReadOnly} aria-label="Somente leitura" />
      </label>
      <p className="text-xs text-muted-foreground">
        {admin
          ? "A chave dá acesso a TODOS os números deste servidor, inclusive à API completa (api_request). Guarde-a como a chave-mestra."
          : "A chave só dá acesso a este número. Não permite parear, desconectar nem mexer em webhooks, Chatwoot ou SIP."}
      </p>
    </div>
  );

  const createdView = created && (
    <div className="min-w-0 space-y-4">
      <p className="flex items-start gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
        <KeyRound className="mt-0.5 h-4 w-4 shrink-0" />
        Copie agora: a chave não será mostrada de novo. Se perder, revogue e crie outra.
      </p>
      <CopyField label="URL completa (claude.ai, ChatGPT e clientes que só pedem a URL)" value={created.url} />
      <CopyField label="Endereço MCP" value={created.endpoint} />
      <CopyField label="Chave (Authorization: Bearer)" value={created.secret} />
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
        No n8n: ferramenta <strong>MCP Client</strong> do agente de IA, com o endereço acima e autenticação Bearer.
      </p>
    </div>
  );

  const title =
    view === "list"
      ? admin
        ? "Configurações gerais"
        : "MCP — agentes de IA"
      : view === "new"
        ? admin
          ? "Nova chave de administrador"
          : "Nova chave MCP"
        : "Chave criada";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {admin ? (
          <button
            className={
              className ??
              "flex w-full items-center gap-2 rounded-xl px-3 py-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            }
            title="Configurações gerais do AstraCalls"
          >
            <Settings className="h-3.5 w-3.5 shrink-0" />
            Configurações
          </button>
        ) : (
          <Button variant={keys.length ? "default" : "outline"} size="sm" title="MCP (agentes de IA)">
            <Bot className="h-4 w-4" />
            <span className="hidden sm:inline">MCP{keys.length > 1 ? ` (${keys.length})` : ""}</span>
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto [&>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {admin
              ? "Opções que valem para todo o AstraCalls."
              : "Conecte Claude, ChatGPT, Cursor, n8n e outros agentes de IA a este número pelo MCP: eles enviam mensagens, leem conversas, fazem ligações e gerenciam grupos."}
          </DialogDescription>
        </DialogHeader>

        {view === "list" ? list : view === "new" ? form : createdView}

        <DialogFooter className="gap-2 sm:justify-between">
          {view === "list" ? (
            <>
              <span className="self-center text-xs text-muted-foreground">
                {keys.length}/{max}
              </span>
              <Button disabled={busy || keys.length >= max} onClick={startNew}>
                <Plus className="h-4 w-4" />
                Nova chave
              </Button>
            </>
          ) : view === "new" ? (
            <>
              <Button variant="outline" disabled={busy} onClick={() => setView("list")}>
                <ArrowLeft className="h-4 w-4" />
                Voltar
              </Button>
              <Button disabled={busy} onClick={create}>
                {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                Criar chave
              </Button>
            </>
          ) : (
            <Button className="sm:ml-auto" onClick={() => setView("list")}>
              Concluir
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
