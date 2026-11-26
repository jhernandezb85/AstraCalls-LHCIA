import { useEffect, useState } from "react";
import { Settings2, Loader2 } from "lucide-react";
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
import { Switch } from "@/components/ui/switch";
import { getOptions, setOptions, type SessionOptions } from "@/services/sessions";

const empty: SessionOptions = { read_messages: false, always_online: false };

export const OptionsDialog = ({ sid }: { sid: string }) => {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [opts, setOpts] = useState<SessionOptions>({ ...empty });

  useEffect(() => {
    if (!open) return;
    getOptions(sid)
      .then((o) => setOpts({ ...empty, ...o }))
      .catch(() => {});
  }, [open, sid]);

  const set = (k: keyof SessionOptions) => (v: boolean) => setOpts((o) => ({ ...o, [k]: v }));

  const save = async () => {
    setBusy(true);
    try {
      await setOptions(sid, opts);
      toast.success("Opciones guardadas");
      setOpen(false);
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" title="Opciones de la cuenta">
          <Settings2 className="h-4 w-4" />
          <span className="hidden sm:inline">Opciones</span>
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Opciones de la cuenta</DialogTitle>
          <DialogDescription>Cómo se ve esta cuenta para los contactos y en el celular.</DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <label className="flex cursor-pointer items-start justify-between gap-4 rounded-lg border p-3">
            <span className="space-y-0.5">
              <span className="block text-sm font-medium">Marcar mensajes como leídos automáticamente</span>
              <span className="block text-xs text-muted-foreground">
                Marca como leído (doble check azul) todo mensaje recibido. Así el celular no muestra el
                mensaje como no leído.
              </span>
            </span>
            <Switch
              checked={opts.read_messages}
              onCheckedChange={set("read_messages")}
              aria-label="Marcar mensajes como leídos automáticamente"
            />
          </label>

          <label className="flex cursor-pointer items-start justify-between gap-4 rounded-lg border p-3">
            <span className="space-y-0.5">
              <span className="block text-sm font-medium">Siempre en línea</span>
              <span className="block text-xs text-muted-foreground">
                Mantiene la cuenta como "en línea". Atención: mientras esté en línea, WhatsApp{" "}
                <b>no notifica al celular</b>. Apagado, la cuenta sale en línea solo un instante al
                conectar, lo justo para que WhatsApp no desvincule el dispositivo.
              </span>
            </span>
            <Switch
              checked={opts.always_online}
              onCheckedChange={set("always_online")}
              aria-label="Siempre en línea"
            />
          </label>
        </div>

        <DialogFooter>
          <Button disabled={busy} onClick={() => void save()}>
            {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Guardar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
