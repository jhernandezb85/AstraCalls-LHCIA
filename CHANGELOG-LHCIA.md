# Cambios del fork LHCIA

Versiones de la imagen `ghcr.io/jhernandezb85/astracalls`. Formato `vX.Y.Z-lhcia.N`:
`vX.Y.Z` es la versión oficial de AstraCalls en la que se basa y `N` la revisión propia.
Para volver atrás, poner en el compose la versión anterior (o la oficial `astraonline/astracalls:vX.Y.Z`).

## v1.1.6-lhcia.5

- CI: cada versión publicada también se etiqueta `latest` (producción usa `:latest`; las
  versiones `vX.Y.Z-lhcia.N` quedan como histórico para volver atrás). Mismo código que `.4`.

## v1.1.6-lhcia.4

- Textos que AstraCalls pone en Chatwoot traducidos al español: grabación de llamada, avisos
  de desconexión/restricción/límite, anuncio (CTWA), grupos, encuestas, eventos, contactos,
  pedidos, "ver una sola vez", editado. Título por defecto "📱 Enviado desde el celular".
- Motivo de envío fallido en Chatwoot en español y entendible (el error técnico queda en el log).

## v1.1.6-lhcia.3

- Chatwoot: el ✓✓ "entregado" ya no se pierde cuando WhatsApp lo confirma antes de que el
  mensaje termine de publicarse en Chatwoot (contacto en línea): el recibo se guarda unos
  minutos y se aplica al publicarse.

## v1.1.6-lhcia.2

- Chatwoot: con `device_mode=open`, la conversación la abre el bot del celular (el aviso dice
  "reabierta por WhatsApp (celular)" y no por el dueño del token); si el bot no tiene acceso,
  se usa el token de la cuenta.

## v1.1.6-lhcia.1

Base: AstraCalls v1.1.6.

- Chatwoot: lo escrito desde el celular entra según `device_mode`: `open` (por defecto) como
  mensaje saliente y la conversación pasa a abierta; `keep` como mensaje sin cambiar el estado;
  `note` como nota privada. Título configurable (`device_title`).
- Chatwoot: los mensajes del celular pueden publicarse con un Agent Bot de la cuenta
  (`device_bot_token`), que no cuenta como respuesta humana.
- Chatwoot: recibos de WhatsApp (entregado/leído) en el estado de los mensajes, encendidos por
  defecto (`read_receipts`); un envío fallido queda como "falló" con el motivo.
- Chatwoot: "Reenviar" en un mensaje fallido ahora lo reenvía de verdad por WhatsApp.
- Chatwoot: un envío lento que Chatwoot marcó como fallido vuelve a "enviado".
- Relación mensaje WhatsApp ↔ Chatwoot guardada en Postgres (`chatwoot_msg_refs`).
- CI: imagen amd64 en GHCR (`.github/workflows/image-lhcia.yml`).
