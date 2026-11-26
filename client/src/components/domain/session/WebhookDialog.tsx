import { useCallback, useEffect, useState } from "react";
import { Webhook, Loader2, Plus, Pencil, Trash2, Send, ArrowLeft, RotateCcw } from "lucide-react";
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
  listWebhooks,
  createWebhook,
  updateWebhook,
  deleteWebhook,
  testWebhook,
  reenableWebhook,
  type WebhookInput,
  type WebhookItem,
} from "@/services/webhook";

// Tipos de evento que o webhook da sessão entrega (nenhum marcado = todos).
const EVENTS: { id: string; label: string }[] = [
  { id: "message", label: "Mensajes" },
  { id: "receipt", label: "Entrega/lectura" },
  { id: "reaction", label: "Reacciones" },
  { id: "deleted", label: "Eliminados" },
  { id: "poll_vote", label: "Votos de encuesta" },
  { id: "event_response", label: "Respuestas de evento" },
  { id: "form_response", label: "Formularios" },
  { id: "group_participants", label: "Participantes de grupo" },
  { id: "view_once", label: "Ver una vez" },
  { id: "session_status", label: "Estado de la cuenta" },
  { id: "broadcast", label: "Envíos masivos" },
  { id: "recording", label: "Grabaciones" },
  { id: "restriction", label: "Restricciones" },
  { id: "call_peer_mute", label: "Silencio en la llamada" },
  { id: "call_hand_raise", label: "Mano levantada" },
];

const eventLabel = (id: string) => EVENTS.find((e) => e.id === id)?.label ?? id;

type Draft = { name: string; url: string; secret: string; events: string[]; ignoreGroups: boolean };
const emptyDraft: Draft = { name: "", url: "", secret: "", events: [], ignoreGroups: false };

export const WebhookDialog = ({ sid }: { sid: string }) => {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [hooks, setHooks] = useState<WebhookItem[]>([]);
  const [max, setMax] = useState(10);
  // null = lista; "new" = criando; id = editando
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>({ ...emptyDraft });

  const load = useCallback(
    () =>
      listWebhooks(sid)
        .then((r) => {
          setHooks(r.webhooks ?? []);
          setMax(r.max || 10);
        })
        .catch(() => {}),
    [sid],
  );

  // carrega já na montagem p/ o botão refletir se há webhook configurado
  useEffect(() => {
    load();
  }, [load, open]);

  useEffect(() => {
    if (!open) setEditing(null);
  }, [open]);

  const current = hooks.find((h) => h.id === editing);

  const startNew = () => {
    setDraft({ ...emptyDraft });
    setEditing("new");
  };

  const startEdit = (h: WebhookItem) => {
    setDraft({ name: h.name, url: h.url, secret: "", events: h.events ?? [], ignoreGroups: h.ignoreGroups });
    setEditing(h.id);
  };

  const toggleEvent = (id: string) =>
    setDraft((d) => ({
      ...d,
      events: d.events.includes(id) ? d.events.filter((e) => e !== id) : [...d.events, id],
    }));

  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    setBusy(true);
    try {
      await fn();
      if (ok) toast.success(ok);
      await load();
      return true;
    } catch (e) {
      toast.error((e as Error).message);
      return false;
    } finally {
      setBusy(false);
    }
  };

  const save = async () => {
    const url = draft.url.trim();
    if (!/^https?:\/\//i.test(url)) {
      toast.error("Escribe una URL http(s) válida");
      return;
    }
    const body: WebhookInput = { name: draft.name.trim(), url, events: draft.events, ignoreGroups: draft.ignoreGroups };
    if (draft.secret.trim()) body.secret = draft.secret.trim();
    const done = await run(
      () => (editing === "new" ? createWebhook(sid, body) : updateWebhook(sid, editing!, body)),
      "Webhook guardado",
    );
    if (done) setEditing(null);
  };

  const remove = (h: WebhookItem) => {
    if (!window.confirm(`¿Eliminar el webhook ${h.name || h.url}?`)) return;
    run(() => deleteWebhook(sid, h.id), "Webhook eliminado").then((ok) => ok && setEditing(null));
  };

  const clearSecret = (h: WebhookItem) =>
    run(
      () =>
        updateWebhook(sid, h.id, {
          name: draft.name.trim(),
          url: draft.url.trim(),
          secret: "",
          events: draft.events,
          ignoreGroups: draft.ignoreGroups,
        }),
      "Secret quitado",
    );

  const test = async (h: WebhookItem) => {
    setBusy(true);
    try {
      const r = await testWebhook(sid, h.id);
      if (r.ok) toast.success(`Prueba entregada (HTTP ${r.status})`);
      else toast.error(r.error ? `Falló: ${r.error}` : `Falló: HTTP ${r.status}`);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const stateBadge = (h: WebhookItem) => {
    const st = h.status?.state ?? "closed";
    if (st === "closed") return <Badge variant="success">Activo</Badge>;
    return (
      <Badge variant="destructive" title={h.status?.disabledReason}>
        {st === "open" ? "Pausado (fallas)" : "Desactivado (fallas)"}
      </Badge>
    );
  };

  const list = (
    <div className="space-y-3">
      {hooks.length === 0 && (
        <p className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
          Esta cuenta no tiene webhooks.
        </p>
      )}
      {hooks.map((h) => (
        <div key={h.id} className="space-y-2 rounded-lg border p-3">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 space-y-0.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">{h.name || (h.id === "default" ? "Principal" : "Webhook")}</span>
                {stateBadge(h)}
              </div>
              <p className="break-all text-xs text-muted-foreground">{h.url}</p>
            </div>
            <div className="flex shrink-0 gap-1">
              <Button size="icon" variant="ghost" disabled={busy} title="Enviar prueba" onClick={() => test(h)}>
                <Send className="h-4 w-4" />
              </Button>
              <Button size="icon" variant="ghost" disabled={busy} title="Editar" onClick={() => startEdit(h)}>
                <Pencil className="h-4 w-4" />
              </Button>
              <Button size="icon" variant="ghost" disabled={busy} title="Eliminar" onClick={() => remove(h)}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <div className="flex flex-wrap gap-1">
            <Badge variant="muted">
              {h.events?.length ? h.events.map(eventLabel).join(", ") : "Todos los eventos"}
            </Badge>
            {h.ignoreGroups && <Badge variant="muted">Ignora grupos</Badge>}
            {h.hasSecret && <Badge variant="muted">Firmado</Badge>}
          </div>
          {h.status && h.status.state !== "closed" && (
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => run(() => reenableWebhook(sid, h.id), "Webhook reactivado")}
            >
              <RotateCcw className="h-4 w-4" />
              Reactivar
            </Button>
          )}
        </div>
      ))}
    </div>
  );

  const form = (
    <div className="space-y-3">
      <div className="space-y-1">
        <Label>Nombre (opcional)</Label>
        <Input
          value={draft.name}
          onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))}
          placeholder="ej.: n8n, CRM"
        />
      </div>
      <div className="space-y-1">
        <Label>URL</Label>
        <Input
          value={draft.url}
          onChange={(e) => setDraft((d) => ({ ...d, url: e.target.value }))}
          placeholder="https://mi-sistema/webhook/astracalls"
        />
      </div>
      <div className="space-y-1">
        <Label>Secret (opcional)</Label>
        <div className="flex gap-2">
          <Input
            value={draft.secret}
            onChange={(e) => setDraft((d) => ({ ...d, secret: e.target.value }))}
            type="password"
            placeholder={current?.hasSecret ? "configurado: déjalo vacío para mantenerlo" : "firma con X-Webhook-Signature"}
          />
          {current?.hasSecret && (
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => clearSecret(current)}>
              Quitar
            </Button>
          )}
        </div>
      </div>

      <label className="flex cursor-pointer items-start justify-between gap-4 rounded-lg border p-3">
        <span className="space-y-0.5">
          <span className="block text-sm font-medium">Ignorar grupos</span>
          <span className="block text-xs text-muted-foreground">
            No envía ningún evento de grupo (mensajes, confirmaciones, reacciones, participantes…). Solo conversaciones
            individuales.
          </span>
        </span>
        <Switch
          checked={draft.ignoreGroups}
          onCheckedChange={(v) => setDraft((d) => ({ ...d, ignoreGroups: v }))}
          aria-label="Ignorar grupos"
        />
      </label>

      <div className="space-y-2">
        <Label>Eventos</Label>
        <p className="text-xs text-muted-foreground">
          {draft.events.length === 0 ? "Ninguno marcado = envía todos." : `${draft.events.length} seleccionado(s).`}
        </p>
        <div className="flex flex-wrap gap-2">
          {EVENTS.map((e) => {
            const on = draft.events.includes(e.id);
            return (
              <Button
                key={e.id}
                type="button"
                size="sm"
                variant={on ? "default" : "outline"}
                className="h-7 px-2 text-xs"
                title={e.id}
                aria-pressed={on}
                onClick={() => toggleEvent(e.id)}
              >
                {e.label}
              </Button>
            );
          })}
        </div>
      </div>
    </div>
  );

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant={hooks.length ? "default" : "outline"} size="sm" title="Webhooks">
          <Webhook className="h-4 w-4" />
          <span className="hidden sm:inline">Webhook{hooks.length > 1 ? `s (${hooks.length})` : ""}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {editing === null ? "Webhooks de la cuenta" : editing === "new" ? "Nuevo webhook" : "Editar webhook"}
          </DialogTitle>
          <DialogDescription>
            Cada webhook recibe los eventos de esta cuenta por POST (JSON) con el formato{" "}
            <code>{"{ session, event, timestamp, data }"}</code>, con sus propios filtros.
          </DialogDescription>
        </DialogHeader>

        {editing === null ? list : form}

        <DialogFooter className="gap-2 sm:justify-between">
          {editing === null ? (
            <>
              <span className="self-center text-xs text-muted-foreground">
                {hooks.length}/{max}
              </span>
              <Button disabled={busy || hooks.length >= max} onClick={startNew}>
                <Plus className="h-4 w-4" />
                Agregar webhook
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" disabled={busy} onClick={() => setEditing(null)}>
                <ArrowLeft className="h-4 w-4" />
                Volver
              </Button>
              <Button disabled={busy} onClick={save}>
                {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                Guardar
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
