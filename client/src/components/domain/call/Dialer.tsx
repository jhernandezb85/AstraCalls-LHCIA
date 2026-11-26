import { useState } from "react";
import { Phone, Video } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { DeviceSelector } from "@/components/form/DeviceSelector";
import { useStartCall } from "@/hooks/useStartCall";
import { useDevices } from "@/stores/devices";

export const Dialer = ({ sid }: { sid: string }) => {
  const [phone, setPhone] = useState("");
  const [video, setVideo] = useState(false);
  const micId = useDevices((s) => s.micId);
  const startCall = useStartCall(sid, micId);

  const submit = () => {
    if (!phone.trim() || startCall.isPending) return;
    startCall.mutate({ phone: phone.trim(), record: false, video }, { onSuccess: () => setPhone("") });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Marcador</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <DeviceSelector />
        <div className="flex flex-wrap items-center gap-2">
          <Input
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") submit();
            }}
            placeholder="+51 987 654 321"
            inputMode="tel"
            className="min-w-0 flex-1"
          />
          <Button
            type="button"
            variant={video ? "default" : "outline"}
            size="sm"
            onClick={() => setVideo((v) => !v)}
            aria-pressed={video}
          >
            <Video className="h-4 w-4" />
            Video
          </Button>
          <Button onClick={submit} disabled={startCall.isPending || !phone.trim()}>
            <Phone className="h-4 w-4" />
            {startCall.isPending ? "Llamando…" : "Llamar"}
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Puedes hacer varias llamadas a la vez: marca otro número y aparece una llamada nueva abajo.
        </p>
      </CardContent>
    </Card>
  );
};
