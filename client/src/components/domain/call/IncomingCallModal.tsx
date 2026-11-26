import { useEffect } from "react";
import { Phone, PhoneIncoming, PhoneOff } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useCalls } from "@/stores/calls";
import { useSessions } from "@/stores/sessions";
import { phoneFromJid } from "@/utils/format";
import { useDevices } from "@/stores/devices";
import { useAcceptCall } from "@/hooks/useAcceptCall";
import { useRejectCall } from "@/hooks/useRejectCall";

type RingHandle = { stop: () => void };

const startRingLoop = (): RingHandle | null => {
  const AC = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AC) return null;
  let ctx: AudioContext;
  try {
    ctx = new AC();
  } catch {
    return null;
  }
  let cancelled = false;
  const playToneAt = (when: number, durationSec: number, freq: number, gainVal = 0.18) => {
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.type = "sine";
    osc.frequency.value = freq;
    const t = ctx.currentTime + when;
    gain.gain.setValueAtTime(0, t);
    gain.gain.linearRampToValueAtTime(gainVal, t + 0.02);
    gain.gain.linearRampToValueAtTime(gainVal, t + durationSec - 0.02);
    gain.gain.linearRampToValueAtTime(0, t + durationSec);
    osc.connect(gain).connect(ctx.destination);
    osc.start(t);
    osc.stop(t + durationSec + 0.05);
  };
  const scheduleCycle = () => {
    if (cancelled) return;
    playToneAt(0, 1.0, 440);
    playToneAt(0, 1.0, 480);
    setTimeout(scheduleCycle, 3000);
  };
  scheduleCycle();
  return {
    stop: () => {
      cancelled = true;
      void ctx.close().catch(() => {});
    },
  };
};

export const IncomingCallModal = () => {
  const incoming = useCalls((s) => s.incoming);
  const micId = useDevices((s) => s.micId);
  const accept = useAcceptCall(micId);
  const reject = useRejectCall();
  const busy = accept.isPending || reject.isPending;
  const session = useSessions((s) => s.sessions.find((x) => x.id === incoming?.sessionId));

  // Quien llama: nombre · número (el servidor ya los resuelve; peer crudo solo como respaldo).
  const callerPhone = incoming?.phone ? `+${incoming.phone}` : incoming?.peer ?? "";
  const caller = incoming?.name ? `${incoming.name} · ${callerPhone}` : callerPhone;
  // Línea por la que entra: del evento o, si falta, de la lista de cuentas.
  const lineName = incoming?.lineName || session?.name || "";
  const lineNum = incoming?.linePhone
    ? `+${incoming.linePhone}`
    : phoneFromJid(session?.jid ?? "") || phoneFromJid(session?.lastJid ?? "");
  const line = [lineName, lineNum].filter(Boolean).join(" · ");

  useEffect(() => {
    if (!incoming) return;
    const ring = startRingLoop();
    return () => ring?.stop();
  }, [incoming]);

  return (
    <Dialog open={!!incoming}>
      <DialogContent
        showCloseButton={false}
        onEscapeKeyDown={(e) => e.preventDefault()}
        onPointerDownOutside={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
        className="sm:max-w-sm"
      >
        <DialogHeader className="items-center text-center">
          <div className="mb-2 flex h-14 w-14 items-center justify-center rounded-full bg-primary/10 text-primary">
            <PhoneIncoming className="h-7 w-7" />
          </div>
          <DialogTitle>{incoming?.video ? "Videollamada entrante" : "Llamada entrante"}</DialogTitle>
          <DialogDescription className="truncate">{caller}</DialogDescription>
          {line && (
            <p className="mt-1 rounded-full bg-primary/10 px-3 py-1 text-xs text-primary">
              Entra por: <span className="font-medium">{line}</span>
            </p>
          )}
        </DialogHeader>
        <div className="mt-2 flex items-center justify-center gap-6">
          <Button
            variant="destructive"
            size="icon"
            className="h-14 w-14 rounded-full"
            disabled={busy}
            onClick={() => incoming && reject.mutate({ sid: incoming.sessionId, callId: incoming.callId })}
            aria-label="Rechazar"
          >
            <PhoneOff className="h-6 w-6" />
          </Button>
          <Button
            size="icon"
            className="h-14 w-14 rounded-full"
            disabled={busy}
            onClick={() => incoming && accept.mutate({ sid: incoming.sessionId, callId: incoming.callId, video: !!incoming.video })}
            aria-label="Contestar"
          >
            <Phone className="h-6 w-6" />
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
};
