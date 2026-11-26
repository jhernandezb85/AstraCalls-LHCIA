import { useCallback, useEffect, useState } from "react";
import { Check, ClipboardList, Copy, Loader2, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { listNativeForms, type NativeFormInfo } from "@/services/sessions";

// Rótulos dos campos padrão dos modelos do app (nome interno → texto).
const FIELD_LABELS: Record<string, string> = {
  full_name: "Nombre",
  first_name: "Nombre",
  last_name: "Apellido",
  phone_number: "Teléfono",
  email: "Correo",
  address: "Dirección",
  city: "Ciudad",
  zip_code: "Código postal",
};

const typeLabel = (t?: string) =>
  t === "template" ? "Plantilla" : t?.startsWith("ai_form") ? "Generado por IA" : t || "Formulario";

// Campos do formulário: os *_visible ligados do modelo, os custom_fields do
// formulário de IA, ou os nomes vistos nas respostas.
const fieldsOf = (f: NativeFormInfo): string[] => {
  const d = f.data ?? {};
  const custom = Array.isArray(d.custom_fields)
    ? (d.custom_fields as { label?: string }[]).map((c) => c.label ?? "").filter(Boolean)
    : [];
  if (custom.length) return custom;
  const visible = Object.entries(d)
    .filter(([k, v]) => k.endsWith("_visible") && v === true)
    .map(([k]) => FIELD_LABELS[k.replace(/_visible$/, "")] ?? k.replace(/_visible$/, ""));
  if (visible.length) return visible;
  return (f.fields ?? []).map((n) => FIELD_LABELS[n] ?? n);
};

const fmtDate = (ms?: number) =>
  ms ? new Date(ms).toLocaleString("es-PE", { dateStyle: "short", timeStyle: "short" }) : "—";

// Corpo pronto para POST /api/sessions/{sid}/messages/native-form.
const sendExample = (f: NativeFormInfo) =>
  JSON.stringify(
    {
      to: "51987654321",
      flowId: f.flowId,
      ...(f.formType && f.formType !== "template" ? { formType: f.formType } : {}),
      body: f.body || "Comparte tus datos.",
      cta: f.cta || "Llenar formulario",
      ...(f.screen ? { screen: f.screen } : {}),
      ...(f.data ? { data: f.data } : {}),
    },
    null,
    2,
  );

const CopyButton = ({ text, label, title }: { text: string; label: string; title: string }) => {
  const [done, setDone] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(text);
      setDone(true);
      setTimeout(() => setDone(false), 1500);
    } catch {
      toast.error("No se pudo copiar");
    }
  };
  return (
    <Button variant="outline" size="sm" onClick={() => void copy()} title={title}>
      {done ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
      {label}
    </Button>
  );
};

export const FormsDialog = ({ sid }: { sid: string }) => {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [forms, setForms] = useState<NativeFormInfo[] | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(
    async (quiet = false) => {
      if (!quiet) setLoading(true);
      try {
        const r = await listNativeForms(sid);
        setForms(r.forms ?? []);
        setError("");
      } catch (e) {
        setError((e as Error).message);
      } finally {
        if (!quiet) setLoading(false);
      }
    },
    [sid],
  );

  // Abre carregando e atualiza a cada 10 s: o formulário enviado pelo celular
  // aparece sozinho enquanto o pop-up está aberto.
  useEffect(() => {
    if (!open) return;
    void load();
    const t = setInterval(() => void load(true), 10000);
    return () => clearInterval(t);
  }, [open, load]);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" title="Formularios nativos de WhatsApp Business">
          <ClipboardList className="h-4 w-4" />
          <span className="hidden sm:inline">Formularios</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Formularios de WhatsApp Business</DialogTitle>
          <DialogDescription>
            Formularios creados en la app (Herramientas para la empresa → Formularios) que esta cuenta ya envió. Copia el
            ID y úsalo en <code className="font-mono">POST /messages/native-form</code>. Un formulario nuevo aparece
            aquí después de enviarlo una vez desde el celular.
          </DialogDescription>
        </DialogHeader>

        <div className="flex justify-end">
          <Button variant="ghost" size="sm" disabled={loading} onClick={() => void load()}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            Actualizar
          </Button>
        </div>

        {error && <p className="text-sm text-destructive">Error al cargar: {error}</p>}

        {forms === null && loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          </div>
        ) : forms && forms.length === 0 ? (
          <div className="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground">
            Todavía no hay formularios. En el celular de esta cuenta, crea un formulario en Herramientas para la empresa →
            Formularios y envíalo a cualquier contacto. Aparece aquí en unos segundos.
          </div>
        ) : (
          <div className="space-y-3">
            {(forms ?? []).map((f) => {
              const fields = fieldsOf(f);
              return (
                <div key={f.flowId} className="space-y-2 rounded-lg border p-3">
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate font-medium">{f.body || f.flowName || "Formulario"}</div>
                      <div className="text-xs text-muted-foreground">
                        Botón: {f.cta || "Llenar formulario"}
                        {f.flowName ? ` · ${f.flowName}` : ""}
                      </div>
                    </div>
                    <Badge variant="secondary">{typeLabel(f.formType)}</Badge>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-xs font-medium text-muted-foreground">ID:</span>
                    <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-sm">{f.flowId}</code>
                    <CopyButton text={f.flowId} label="Copiar ID" title="Copiar el flowId" />
                    <CopyButton
                      text={sendExample(f)}
                      label="Copiar envío"
                      title="Copiar el cuerpo listo para POST /messages/native-form"
                    />
                  </div>

                  {fields.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {fields.map((name) => (
                        <Badge key={name} variant="muted">
                          {name}
                        </Badge>
                      ))}
                    </div>
                  )}

                  <div className="text-xs text-muted-foreground">
                    Enviado {f.sentCount}× · último {fmtDate(f.lastSentAt)} · {f.responseCount}{" "}
                    {f.responseCount === 1 ? "respuesta" : "respuestas"}
                    {f.lastResponseAt ? ` · última ${fmtDate(f.lastResponseAt)}` : ""}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};
