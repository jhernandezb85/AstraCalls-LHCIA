import { useEffect, useState, type ReactNode } from "react";
import { Phone, Copy, Check, Eye, EyeOff, Loader2, Server, ArrowUpRight } from "lucide-react";
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
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { getSIPExt, setSIPExt, type SIPExtConfig } from "@/services/sessions";
import type { SessionInfo } from "@/types/session";

const SIP_PORT = 5060;

type Field = "user" | "pass" | "server" | "domain";
type Tab = "server" | "external";

const emptyExt: SIPExtConfig = {
  enabled: false,
  host: "",
  port: SIP_PORT,
  user: "",
  authUser: "",
  pass: "",
  dest: "",
  proxy: "",
  inboundMode: "originate",
  agentAutoAnswer: false,
};

const inboundModes: { value: NonNullable<SIPExtConfig["inboundMode"]>; label: string; hint: string }[] = [
  { value: "originate", label: "Marcar por WhatsApp", hint: "El PBX envía el número de destino y la llamada sale por WhatsApp (por defecto)." },
  { value: "agent", label: "Contestar con el agente de voz", hint: "Quien llama al anexo es atendido por el agente (evento incoming + WS de audio)." },
  { value: "auto", label: "Automático", hint: "Si llamó al propio anexo: agente. Si llamó a otro número: marca por WhatsApp." },
];

const StatusBadge = ({ status, error }: { status?: string; error?: string }) => {
  if (!status) return null;
  const map: Record<string, { label: string; cls: string }> = {
    registered: { label: "Registrado", cls: "bg-emerald-500/15 text-emerald-600 border-emerald-500/30" },
    registering: { label: "Registrando…", cls: "bg-amber-500/15 text-amber-600 border-amber-500/30" },
    failed: { label: "Falló", cls: "bg-red-500/15 text-red-600 border-red-500/30" },
  };
  const s = map[status] ?? { label: status, cls: "" };
  return (
    <Badge variant="outline" className={cn("font-normal", s.cls)} title={error || undefined}>
      {s.label}
    </Badge>
  );
};

export const SIPDialog = ({ session }: { session: SessionInfo }) => {
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<Tab>("server");
  const [showPass, setShowPass] = useState(false);
  const [copied, setCopied] = useState<Field | null>(null);

  // Modelo 1 (servidor): credenciais que o PBX do cliente usa para registrar aqui.
  const sipUser = session.sip_user || "";
  const sipPass = session.sip_pass || "";
  const server =
    session.sip_url && session.sip_url !== "127.0.0.1:5060"
      ? session.sip_url
      : `${window.location.hostname}:${SIP_PORT}`;
  const domain = server.split(":")[0];

  // Modelo 2 (externo): esta sessão se registra num PBX do cliente.
  const [ext, setExt] = useState<SIPExtConfig>({ ...emptyExt });
  const [busy, setBusy] = useState(false);
  const [showExtPass, setShowExtPass] = useState(false);

  useEffect(() => {
    if (!open) return;
    getSIPExt(session.id)
      .then((r) => setExt({ ...emptyExt, ...r, port: r.port || SIP_PORT }))
      .catch(() => {});
  }, [open, session.id]);

  const copy = (field: Field, value: string) => {
    navigator.clipboard?.writeText(value);
    setCopied(field);
    setTimeout(() => setCopied(null), 1500);
  };

  const setE = <K extends keyof SIPExtConfig>(k: K, v: SIPExtConfig[K]) =>
    setExt((prev) => ({ ...prev, [k]: v }));

  const saveExt = async () => {
    if (ext.enabled && (!ext.host.trim() || !ext.user.trim())) {
      toast.error("Completa el host y el usuario del PBX");
      return;
    }
    setBusy(true);
    try {
      await setSIPExt(session.id, {
        enabled: ext.enabled,
        host: ext.host.trim(),
        port: Number(ext.port) || SIP_PORT,
        user: ext.user.trim(),
        authUser: (ext.authUser ?? "").trim(),
        pass: ext.pass,
        dest: ext.dest.trim(),
        proxy: (ext.proxy ?? "").trim(),
        inboundMode: ext.inboundMode ?? "originate",
        agentAutoAnswer: !!ext.agentAutoAnswer,
        agentRingTimeoutSeconds: ext.agentRingTimeoutSeconds,
      });
      toast.success(ext.enabled ? "Registro en el PBX guardado, conectando…" : "Registro en el PBX desactivado");
      setOpen(false);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const Row = ({ label, field, value }: { label: string; field: Field; value: string }) => (
    <div className="space-y-1">
      <Label>{label}</Label>
      <div className="flex gap-2">
        <Input readOnly value={value} className="font-mono text-xs" onFocus={(e) => e.target.select()} />
        <Button type="button" variant="outline" size="icon" onClick={() => copy(field, value)} aria-label={`Copiar ${label}`}>
          {copied === field ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );

  const TabButton = ({ id, icon, children }: { id: Tab; icon: ReactNode; children: ReactNode }) => (
    <button
      type="button"
      onClick={() => setTab(id)}
      className={cn(
        "flex flex-1 items-center justify-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors",
        tab === id ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
      )}
    >
      {icon}
      {children}
    </button>
  );

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant={session.sip_ext_status === "registered" ? "default" : "outline"} size="sm">
          <Phone className="h-4 w-4" />
          SIP
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Integración SIP / PBX</DialogTitle>
          <DialogDescription>
            Dos formas de integrar esta cuenta a un PBX/softphone. Elige una de las pestañas de abajo.
          </DialogDescription>
        </DialogHeader>

        <div className="flex gap-1 rounded-full bg-muted p-1">
          <TabButton id="server" icon={<Server className="h-3.5 w-3.5" />}>
            El PBX se registra aquí
          </TabButton>
          <TabButton id="external" icon={<ArrowUpRight className="h-3.5 w-3.5" />}>
            Registrar en un PBX
          </TabButton>
        </div>

        {tab === "server" ? (
          <div className="space-y-3">
            <p className="text-xs text-muted-foreground">
              El PBX/softphone del cliente <b>se registra en este servidor</b> con las credenciales de abajo.
              Las llamadas marcadas por la troncal salen por el WhatsApp de esta cuenta.
            </p>
            <Row label="Servidor SIP (host:puerto)" field="server" value={server} />
            <Row label="Dominio / Realm" field="domain" value={domain} />
            <Row label="Usuario SIP" field="user" value={sipUser} />

            <div className="space-y-1">
              <Label>Contraseña SIP</Label>
              <div className="flex gap-2">
                <Input
                  readOnly
                  type={showPass ? "text" : "password"}
                  value={sipPass}
                  className="font-mono text-xs"
                  onFocus={(e) => e.target.select()}
                />
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  onClick={() => setShowPass((v) => !v)}
                  aria-label={showPass ? "Ocultar contraseña" : "Mostrar contraseña"}
                >
                  {showPass ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="icon"
                  onClick={() => copy("pass", sipPass)}
                  aria-label="Copiar contraseña"
                >
                  {copied === "pass" ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>

            <div className="rounded-md border border-border bg-muted/40 p-3 text-xs text-muted-foreground">
              <p className="font-medium text-foreground">Cómo configurarlo</p>
              <ul className="mt-1 list-disc space-y-0.5 pl-4">
                <li>Transporte: <b>UDP</b> (puerto {SIP_PORT}), codec <b>G.711 u-law (PCMU)</b>.</li>
                <li>En FreePBX: crea un <b>Trunk PJSIP</b> con Username = Usuario SIP y SIP Server = el host de arriba.</li>
                <li>Marca el número en formato internacional (ej.: 51987654321).</li>
                <li>Mantén estas credenciales en reserva: permiten hacer llamadas por esta cuenta.</li>
              </ul>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <div>
                <p className="text-sm font-medium">Registrar en un PBX externo</p>
                <p className="text-xs text-muted-foreground">
                  AstraCalls pasa a ser un <b>anexo registrado</b> en tu Asterisk/FreePBX.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <StatusBadge status={ext.status} error={ext.error} />
                <Switch checked={ext.enabled} onCheckedChange={(v) => setE("enabled", v)} />
              </div>
            </div>

            {ext.error && ext.status === "failed" ? (
              <p className="text-xs text-red-600">{ext.error}</p>
            ) : null}

            <div className="grid grid-cols-[1fr_6rem] gap-2">
              <div className="space-y-1">
                <Label>Host del PBX</Label>
                <Input
                  value={ext.host}
                  onChange={(e) => setE("host", e.target.value)}
                  placeholder="pbx.tuempresa.com"
                  disabled={!ext.enabled}
                />
              </div>
              <div className="space-y-1">
                <Label>Puerto</Label>
                <Input
                  value={String(ext.port)}
                  onChange={(e) => setE("port", Number(e.target.value.replace(/\D/g, "")) || 0)}
                  inputMode="numeric"
                  placeholder="5060"
                  disabled={!ext.enabled}
                />
              </div>
            </div>

            <div className="space-y-1">
              <Label>Proxy (outbound proxy), opcional</Label>
              <Input
                value={ext.proxy ?? ""}
                onChange={(e) => setE("proxy", e.target.value)}
                placeholder="ej.: sip.proveedor.com:5060 (vacío para usar el host)"
                disabled={!ext.enabled}
              />
              <p className="text-[0.7rem] text-muted-foreground">
                A dónde enviar el registro y las llamadas cuando el <b>dominio</b> (host de arriba) es
                distinto del <b>servidor</b>; es común en FreePBX y troncales alojadas.
              </p>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>Usuario / Anexo</Label>
                <Input
                  value={ext.user}
                  onChange={(e) => setE("user", e.target.value)}
                  autoComplete="off"
                  placeholder="1001"
                  disabled={!ext.enabled}
                />
              </div>
              <div className="space-y-1">
                <Label>Contraseña</Label>
                <div className="flex gap-2">
                  <Input
                    value={ext.pass}
                    onChange={(e) => setE("pass", e.target.value)}
                    type={showExtPass ? "text" : "password"}
                    autoComplete="new-password"
                    placeholder="contraseña del anexo"
                    disabled={!ext.enabled}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => setShowExtPass((v) => !v)}
                    aria-label={showExtPass ? "Ocultar contraseña" : "Mostrar contraseña"}
                  >
                    {showExtPass ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </div>
            </div>

            <div className="space-y-1">
              <Label>Usuario de autenticación (Auth ID), opcional</Label>
              <Input
                value={ext.authUser ?? ""}
                onChange={(e) => setE("authUser", e.target.value)}
                autoComplete="off"
                placeholder="vacío para usar el anexo"
                disabled={!ext.enabled}
              />
              <p className="text-[0.7rem] text-muted-foreground">
                Complétalo cuando el PBX autentica con un ID distinto del anexo; en <b>3CX</b> es el
                <b> Authentication ID</b> del anexo (la contraseña es la <b>Authentication Password</b>).
              </p>
            </div>

            <div className="space-y-1">
              <Label>Anexo/destino para llamadas entrantes (opcional)</Label>
              <Input
                value={ext.dest}
                onChange={(e) => setE("dest", e.target.value)}
                placeholder="ej.: 600 (cola/anexo que suena cuando entra una llamada de WhatsApp)"
                disabled={!ext.enabled}
              />
            </div>

            <div className="space-y-1">
              <Label htmlFor="sip-inbound-mode">Llamada que entra al anexo</Label>
              <select
                id="sip-inbound-mode"
                value={ext.inboundMode ?? "originate"}
                onChange={(e) => setE("inboundMode", e.target.value as SIPExtConfig["inboundMode"])}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              >
                {inboundModes.map((m) => (
                  <option key={m.value} value={m.value}>
                    {m.label}
                  </option>
                ))}
              </select>
              <p className="text-[0.7rem] text-muted-foreground">
                {inboundModes.find((m) => m.value === (ext.inboundMode ?? "originate"))?.hint}
              </p>
            </div>

            {(ext.inboundMode ?? "originate") !== "originate" ? (
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm">Contestar al instante</p>
                  <p className="text-[0.7rem] text-muted-foreground">
                    Responde 200 OK sin esperar el POST /accept del puente del agente.
                  </p>
                </div>
                <Switch checked={!!ext.agentAutoAnswer} onCheckedChange={(v) => setE("agentAutoAnswer", v)} />
              </div>
            ) : null}

            <div className="rounded-md border border-border bg-muted/40 p-3 text-xs text-muted-foreground">
              <p className="font-medium text-foreground">Cómo funciona</p>
              <ul className="mt-1 list-disc space-y-0.5 pl-4">
                <li>Crea un <b>anexo</b> en tu PBX y pon aquí su usuario y contraseña.</li>
                <li>
                  <b>PBX → WhatsApp:</b> en el dialplan, envía la llamada a este anexo con el número de
                  destino en formato internacional (ej.: 51987654321).
                </li>
                <li>
                  <b>WhatsApp → PBX:</b> las llamadas entrantes suenan en el anexo o cola del campo de arriba.
                </li>
                <li>Codec <b>G.711 (PCMU o PCMA)</b>, transporte UDP.</li>
              </ul>
            </div>
          </div>
        )}

        {tab === "external" ? (
          <DialogFooter>
            <Button disabled={busy} onClick={() => void saveExt()}>
              {busy ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : null}
              Guardar
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
};
