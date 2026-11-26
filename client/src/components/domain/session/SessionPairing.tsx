import { useEffect, useRef, useState } from "react";
import { Loader2, ShieldCheck, KeyRound, QrCode, Smartphone } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useSessions, setPairingCode } from "@/stores/sessions";
import { submitPasskeyAssertion, pairSessionCode } from "@/services/sessions";
import {
  isExtensionInstalled,
  runPasskeyAssertion,
  EXTENSION_DOWNLOAD_URL,
} from "@/lib/passkey";
import type { SessionInfo } from "@/types/session";

const PasskeyStep = ({ session }: { session: SessionInfo }) => {
  const publicKey = useSessions((s) => s.passkeys[session.id]);
  const [hasExt, setHasExt] = useState<boolean | null>(null);
  const [status, setStatus] = useState<"idle" | "running" | "error">("idle");
  const [error, setError] = useState("");
  const startedFor = useRef<string | null>(null);

  useEffect(() => {
    void isExtensionInstalled().then(setHasExt);
  }, []);

  const authorize = async () => {
    if (!publicKey) return;
    setStatus("running");
    setError("");
    try {
      const assertion = await runPasskeyAssertion(publicKey);
      await submitPasskeyAssertion(session.id, assertion);
      // sucesso final chega via auth-state (state="open")
    } catch (e) {
      setStatus("error");
      setError(e instanceof Error ? e.message : String(e));
      return;
    }
    setStatus("idle");
  };

  // dispara automaticamente uma vez quando a extensão está presente e há desafio
  useEffect(() => {
    if (hasExt && publicKey && startedFor.current !== session.id && status === "idle") {
      startedFor.current = session.id;
      void authorize();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasExt, publicKey, session.id]);

  return (
    <div className="flex flex-col items-center gap-4 text-center">
      <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground">
        <KeyRound className="h-7 w-7" />
      </div>
      <div>
        <p className="font-semibold">Esta cuenta usa passkey</p>
        <p className="text-sm text-muted-foreground">
          WhatsApp pide una confirmación de seguridad del dueño de la cuenta para conectar.
        </p>
      </div>

      {hasExt === false ? (
        <div className="space-y-3">
          <Badge variant="destructive">No se encontró la extensión AstraCalls Passkey</Badge>
          <p className="text-sm text-muted-foreground">
            Instala la extensión en el navegador del dueño de la cuenta para autorizar la conexión:
          </p>
          <ol className="mx-auto max-w-xs space-y-1 text-left text-sm text-muted-foreground">
            <li>1. Descarga y descomprime el archivo de abajo</li>
            <li>
              2. Abre <code className="rounded bg-muted px-1">chrome://extensions</code> y activa el
              <b> Modo de desarrollador</b>
            </li>
            <li>3. Haz clic en <b>Cargar descomprimida</b> y elige la carpeta</li>
            <li>4. Vuelve aquí y reactiva la conexión</li>
          </ol>
          <Button asChild>
            <a href={EXTENSION_DOWNLOAD_URL} download>
              Descargar extensión (.zip)
            </a>
          </Button>
        </div>
      ) : status === "running" ? (
        <Badge variant="muted" className="gap-1.5">
          <Loader2 className="h-3 w-3 animate-spin" /> Esperando la confirmación en la pestaña de WhatsApp…
        </Badge>
      ) : status === "error" ? (
        <div className="space-y-3">
          <Badge variant="destructive">Error: {error}</Badge>
          <Button onClick={() => void authorize()} className="gap-1.5">
            <ShieldCheck className="h-4 w-4" /> Intentar de nuevo
          </Button>
        </div>
      ) : (
        <Button onClick={() => void authorize()} className="gap-1.5">
          <ShieldCheck className="h-4 w-4" /> Confirmar con passkey
        </Button>
      )}
    </div>
  );
};

const CodeStep = ({ session }: { session: SessionInfo }) => {
  const code = useSessions((s) => s.codes[session.id]);
  const [phone, setPhone] = useState("");
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [error, setError] = useState("");

  const digits = phone.replace(/\D/g, "");
  const canSubmit = digits.length >= 10 && status !== "loading";

  const generate = async () => {
    if (!canSubmit) return;
    setStatus("loading");
    setError("");
    try {
      const { code: c } = await pairSessionCode(session.id, digits);
      setPairingCode(session.id, c);
      setStatus("idle");
    } catch (e) {
      setStatus("error");
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const pretty = code ? (code.length === 8 ? `${code.slice(0, 4)}-${code.slice(4)}` : code) : "";

  return (
    <div className="flex w-full flex-col items-center gap-4 text-center">
      {code ? (
        <>
          <p className="text-sm text-muted-foreground">
            En el WhatsApp del número, abre <b>Dispositivos vinculados → Vincular un dispositivo →
            Vincular con el número de teléfono</b> y escribe:
          </p>
          <div className="rounded-xl border bg-muted/50 px-6 py-4">
            <span className="font-mono text-3xl font-bold tracking-[0.25em]">{pretty}</span>
          </div>
          <Badge variant="muted" className="gap-1.5">
            <Loader2 className="h-3 w-3 animate-spin" /> Esperando la confirmación en el celular…
          </Badge>
          <Button variant="ghost" size="sm" onClick={() => void generate()} disabled={status === "loading"}>
            Generar un código nuevo
          </Button>
        </>
      ) : (
        <>
          <p className="text-sm text-muted-foreground">
            Escribe el número con código de país (ej.: <code className="rounded bg-muted px-1">51987654321</code>)
            para recibir un código de vinculación.
          </p>
          <Input
            type="tel"
            inputMode="numeric"
            placeholder="51987654321"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void generate();
            }}
            className="text-center"
          />
          {status === "error" && <Badge variant="destructive">Error: {error}</Badge>}
          <Button onClick={() => void generate()} disabled={!canSubmit} className="w-full gap-1.5">
            {status === "loading" ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Generando…
              </>
            ) : (
              <>
                <Smartphone className="h-4 w-4" /> Generar código
              </>
            )}
          </Button>
        </>
      )}
    </div>
  );
};

export const SessionPairing = ({ session }: { session: SessionInfo }) => {
  const qr = useSessions((s) => s.qrs[session.id]);
  const hasCode = useSessions((s) => !!s.codes[session.id]);
  const isPasskey = session.state === "passkey_request";
  const [mode, setMode] = useState<"qr" | "code">(
    session.state === "pairing_code" || hasCode ? "code" : "qr",
  );

  return (
    <div className="flex min-h-[55vh] items-center justify-center">
      <Card className="w-full max-w-md">
        <CardHeader className="items-center text-center">
          <CardTitle>Conectar {session.name}</CardTitle>
          {!isPasskey && mode === "qr" && (
            <CardDescription>
              Abre WhatsApp → Dispositivos vinculados → Vincular un dispositivo y escanea.
            </CardDescription>
          )}
        </CardHeader>
        <CardContent className="flex flex-col items-center gap-4">
          {isPasskey ? (
            <PasskeyStep session={session} />
          ) : (
            <>
              {/* Seletor de método: QR ou código por número */}
              <div className="flex w-full rounded-lg border p-1">
                <button
                  type="button"
                  onClick={() => setMode("qr")}
                  className={`flex flex-1 items-center justify-center gap-1.5 rounded-md py-1.5 text-sm font-medium transition ${
                    mode === "qr" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <QrCode className="h-4 w-4" /> Código QR
                </button>
                <button
                  type="button"
                  onClick={() => setMode("code")}
                  className={`flex flex-1 items-center justify-center gap-1.5 rounded-md py-1.5 text-sm font-medium transition ${
                    mode === "code" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  <Smartphone className="h-4 w-4" /> Código
                </button>
              </div>

              {mode === "code" ? (
                <CodeStep session={session} />
              ) : qr ? (
                <div className="rounded-lg border bg-white p-3">
                  <QRCodeSVG value={qr} size={232} marginSize={1} />
                </div>
              ) : session.state === "logged_out" ? (
                <Badge variant="destructive">Desconectado: usa Reactivar arriba para generar un QR</Badge>
              ) : (
                <>
                  <Skeleton className="h-[258px] w-[258px] rounded-lg" />
                  <Badge variant="muted" className="gap-1.5">
                    <Loader2 className="h-3 w-3 animate-spin" /> Esperando el QR…
                  </Badge>
                </>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
};
