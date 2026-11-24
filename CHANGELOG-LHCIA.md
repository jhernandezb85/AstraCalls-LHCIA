# Cambios del fork LHCIA · LHCIA fork changelog

[Español](#español) · [English](#english)

---

## Español

Versiones de la imagen `ghcr.io/jhernandezb85/astracalls`. Formato `vX.Y.Z-lhcia.N`:
`vX.Y.Z` es la versión oficial de AstraCalls en la que se basa y `N` la revisión propia.
`latest` apunta a la última versión publicada. Para volver atrás, poner en el compose la
versión anterior (o la oficial `astraonline/astracalls:vX.Y.Z`).

### v1.1.6-lhcia.12

- Chatwoot: los datos del anuncio en los atributos de la conversación pasan a ser una opción de
  la CUENTA, no de cada bandeja: AstraCalls los llena en cualquier bandeja cuando la cuenta
  tiene definido el atributo `ctwa_clid` (lo crea el provisioner desde Integraciones →
  "Meta · API de conversiones"). Se quita la opción por bandeja `ad_attributes` de la `.11`.

### v1.1.6-lhcia.11

- Chatwoot: opción `ad_attributes` (apagada por defecto). Con ella, cuando un cliente llega
  por un anuncio, los datos se guardan en los atributos de la conversación: `ctwa_clid`,
  `ad_id`, `ad_title`, `ad_source`, `ad_url`. Sirven para la API de conversiones de Meta
  (CAPI) y para automatizaciones, sin leer la nota privada. Se suman a los atributos que ya
  tenga la conversación (el anuncio más reciente reemplaza al anterior).

### v1.1.6-lhcia.10

- Chatwoot: el primer mensaje de un contacto que llega por un anuncio (CTWA) lleva arriba una
  línea corta "📢 Desde el anuncio: título — descripción", como la tarjeta que muestra
  WhatsApp. Captain no lee notas privadas; esta línea sí, y así sabe de qué producto se habla.
  La nota privada completa (enlace, ID, ctwa_clid) sigue igual.

### v1.1.6-lhcia.9

- Eliminar en WhatsApp desde Chatwoot: si el mensaje no tiene guardado el ID de WhatsApp en
  Chatwoot (la función que lo guarda no existe en todo Chatwoot), se usa la relación guardada
  por AstraCalls en Postgres. El log dice qué mensaje se elimina.

### v1.1.6-lhcia.8

- Chatwoot: los envíos lentos (grupos grandes) ya no se cortan a los 5 segundos. Antes, cuando
  Chatwoot dejaba de esperar, AstraCalls abortaba el envío a medias ("context canceled") y el
  mensaje no salía. Ahora termina el envío y la marca vuelve a "enviado".
- Eliminar en WhatsApp desde Chatwoot: un reintento si WhatsApp no responde a tiempo.

### v1.1.6-lhcia.7

- Chatwoot: mensajes eliminados ("eliminar para todos").
  - El cliente elimina en WhatsApp: según `contact_delete`, `note` (por defecto) conserva el
    mensaje y agrega una nota privada citándolo; `delete` lo elimina también en Chatwoot.
  - Tú eliminas en el celular: se elimina también en Chatwoot.
  - Eliminas un mensaje enviado en Chatwoot: se elimina para todos en WhatsApp. Pasados
    ~2 días y medio (límite de WhatsApp) o si falla, queda una nota avisando.
- Los mensajes recibidos también se guardan en `chatwoot_msg_refs` (para poder eliminarlos).

### v1.1.6-lhcia.6

- Chatwoot: en el aviso de anuncio (CTWA), la descripción se muestra como texto normal; las
  líneas con sangría ya no aparecen como bloque de código con barra de desplazamiento.

### v1.1.6-lhcia.5

- CI: cada versión publicada también se etiqueta `latest` (producción usa `:latest`; las
  versiones `vX.Y.Z-lhcia.N` quedan como histórico para volver atrás). Mismo código que `.4`.

### v1.1.6-lhcia.4

- Textos que AstraCalls pone en Chatwoot traducidos al español: grabación de llamada, avisos
  de desconexión/restricción/límite, anuncio (CTWA), grupos, encuestas, eventos, contactos,
  pedidos, "ver una sola vez", editado. Título por defecto "📱 Enviado desde el celular".
- Motivo de envío fallido en Chatwoot en español y entendible (el error técnico queda en el log).

### v1.1.6-lhcia.3

- Chatwoot: el ✓✓ "entregado" ya no se pierde cuando WhatsApp lo confirma antes de que el
  mensaje termine de publicarse en Chatwoot (contacto en línea): el recibo se guarda unos
  minutos y se aplica al publicarse.

### v1.1.6-lhcia.2

- Chatwoot: con `device_mode=open`, la conversación la abre el bot del celular (el aviso dice
  "reabierta por WhatsApp (celular)" y no por el dueño del token); si el bot no tiene acceso,
  se usa el token de la cuenta.

### v1.1.6-lhcia.1

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

---

## English

Image versions of `ghcr.io/jhernandezb85/astracalls`. Format `vX.Y.Z-lhcia.N`: `vX.Y.Z` is
the official AstraCalls version it's based on and `N` our own revision. `latest` points to the
latest published version. To roll back, put the previous version in the compose (or the
official `astraonline/astracalls:vX.Y.Z`).

### v1.1.6-lhcia.12

- Chatwoot: storing the ad data in the conversation attributes is now an ACCOUNT option, not a
  per-inbox one: AstraCalls fills them in any inbox when the account has the `ctwa_clid`
  attribute defined (the provisioner creates it from Integrations → "Meta · API de
  conversiones"). The per-inbox `ad_attributes` option from `.11` is removed.

### v1.1.6-lhcia.11

- Chatwoot: `ad_attributes` option (off by default). When on, if a customer comes from an ad,
  the data is stored in the conversation's custom attributes: `ctwa_clid`, `ad_id`,
  `ad_title`, `ad_source`, `ad_url`. Meant for Meta's Conversions API (CAPI) and automations,
  without parsing the private note. Merged with the conversation's existing attributes (the
  latest ad replaces the previous one).

### v1.1.6-lhcia.10

- Chatwoot: the first message from a contact who came from an ad (CTWA) gets a short line on
  top, "📢 Desde el anuncio: title — description", like the ad card WhatsApp shows. Captain
  doesn't read private notes; it does read this line, so it knows which product it's about.
  The full private note (link, ID, ctwa_clid) is unchanged.

### v1.1.6-lhcia.9

- Deleting in WhatsApp from Chatwoot: if Chatwoot doesn't have the WhatsApp ID stored on the
  message (the endpoint that stores it doesn't exist in every Chatwoot), the link AstraCalls
  keeps in Postgres is used. The log says which message is being deleted.

### v1.1.6-lhcia.8

- Chatwoot: slow sends (large groups) are no longer cut off after 5 seconds. Before, when
  Chatwoot stopped waiting, AstraCalls aborted the send halfway ("context canceled") and the
  message never went out. Now the send finishes and the status goes back to "sent".
- Deleting in WhatsApp from Chatwoot: one retry if WhatsApp doesn't answer in time.

### v1.1.6-lhcia.7

- Chatwoot: deleted messages ("delete for everyone").
  - The customer deletes on WhatsApp: per `contact_delete`, `note` (default) keeps the message
    and adds a private note quoting it; `delete` deletes it in Chatwoot too.
  - You delete on the phone: it's deleted in Chatwoot too.
  - You delete a sent message in Chatwoot: it's deleted for everyone on WhatsApp. After ~2.5
    days (WhatsApp's limit) or on failure, a note says so.
- Incoming messages are also stored in `chatwoot_msg_refs` (so they can be deleted).

### v1.1.6-lhcia.6

- Chatwoot: in the ad (CTWA) note, the description shows as plain text; indented lines no
  longer render as a scrolling code block.

### v1.1.6-lhcia.5

- CI: every published version is also tagged `latest` (production uses `:latest`; the
  `vX.Y.Z-lhcia.N` versions stay as history for rollbacks). Same code as `.4`.

### v1.1.6-lhcia.4

- Texts AstraCalls posts to Chatwoot translated to Spanish: call recording, disconnect /
  restriction / limit alerts, ad (CTWA), groups, polls, events, contacts, orders, "view once",
  edited. Default title "📱 Enviado desde el celular".
- Failed-send reason in Chatwoot in plain Spanish (the technical error stays in the log).

### v1.1.6-lhcia.3

- Chatwoot: the ✓✓ "delivered" tick is no longer lost when WhatsApp confirms before the
  message finishes posting to Chatwoot (contact online): the receipt is held for a few
  minutes and applied once posted.

### v1.1.6-lhcia.2

- Chatwoot: with `device_mode=open`, the phone bot opens the conversation (the notice says
  "reopened by WhatsApp (celular)", not by the token owner); if the bot has no access, the
  account token is used.

### v1.1.6-lhcia.1

Base: AstraCalls v1.1.6.

- Chatwoot: phone messages come in per `device_mode`: `open` (default) as an outgoing message
  and the conversation becomes open; `keep` as a message without changing status; `note` as a
  private note. Configurable title (`device_title`).
- Chatwoot: phone messages can be posted by an account Agent Bot (`device_bot_token`), which
  doesn't count as a human reply.
- Chatwoot: WhatsApp receipts (delivered/read) as message status, on by default
  (`read_receipts`); a failed send is marked "failed" with the reason.
- Chatwoot: "Retry" on a failed message now really resends it through WhatsApp.
- Chatwoot: a slow send Chatwoot marked as failed goes back to "sent".
- WhatsApp ↔ Chatwoot message link stored in Postgres (`chatwoot_msg_refs`).
- CI: amd64 image on GHCR (`.github/workflows/image-lhcia.yml`).
