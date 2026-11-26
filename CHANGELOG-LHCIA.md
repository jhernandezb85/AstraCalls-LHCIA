# Cambios del fork LHCIA · LHCIA fork changelog

[Español](#español) · [English](#english)

---

## Español

Versiones de la imagen `ghcr.io/jhernandezb85/astracalls`. Formato `vX.Y.Z-lhcia.N`:
`vX.Y.Z` es la versión oficial de AstraCalls en la que se basa y `N` la revisión propia.
`latest` apunta a la última versión publicada. Para volver atrás, poner en el compose la
versión anterior (o la oficial `astraonline/astracalls:vX.Y.Z`).

### v1.1.7-lhcia.13

- **Diagnóstico de llamadas, más detalle:** las líneas `diag llamada: señal` traen todos los atributos y la señal completa (salvo el offer), para distinguir "colgó el cliente" de "contestada en el celular". Nueva línea `diag llamada: sync` con el tipo de cada sincronización que manda el celular (una por tipo y minuto).

### v1.1.7-lhcia.12

- **Diagnóstico de llamadas hechas o contestadas en el celular:** el log registra (líneas `diag llamada:`) cada señal de llamada que recibe el dispositivo vinculado y el registro de llamadas que sincroniza el celular (`CallLogAction` y `CallLogMessage`). Solo registra; no cambia el comportamiento. Sirve para corregir que la llamada siga sonando en el CRM cuando se contesta o se corta en el celular, y para dejar en Chatwoot las llamadas hechas desde el celular.

### v1.1.7-lhcia.11

- **Llamada entrante: por qué número entra.** La tarjeta de Chatwoot y el aviso del panel muestran "Entra por: <cuenta> · +<número>", para saber en cuál de las líneas está sonando. El evento `incoming` trae `lineName` y `linePhone`.
- El aviso de llamada entrante del panel ahora está en español y muestra el nombre y el número de quien llama (antes podía salir el LID crudo).
- Configuración: la pestaña activa ya no muestra el borde de foco al hacer clic.

### v1.1.7-lhcia.10

- **Página de Configuración:** el botón Configuración de la barra lateral abre una página con pestañas: **MCP** (llaves de administrador), **Números desvinculados** (cuentas cuya bandeja de Chatwoot se borró, con la cuenta atrás de 24 h antes del borrado) y **Sistema** (versión, commit, tiempo encendido, cuentas conectadas, opciones del servidor, documentación de la API y extensión Passkey).
- **Script de Chatwoot en español:** el botón y la tarjeta de llamada que ven los asesores (llamar, contestar, rechazar, silenciar, video, llamadas grupales y avisos). La búsqueda de respaldo del lugar del botón también reconoce Chatwoot en español e inglés.
- `/api/config` devuelve `startedAt` (inicio del proceso).

### v1.1.7-lhcia.9

- **Panel en español (lo más usado):** barra lateral, encabezado de la sesión, vinculación por QR o código, tarjetas de llamada (individual y grupal), marcador, historial, inicio de sesión, ventana MCP y Configuración general. Pendiente: los diálogos técnicos (SIP, Webhook, Proxy, Chatwoot, Formularios, Opciones).

### v1.1.7-lhcia.8

- **MCP en español:** las instrucciones, los nombres y descripciones de las herramientas, sus parámetros y los mensajes de error que lee la IA.

### v1.1.7-lhcia.7

- **Resolver en Chatwoot archiva también los chats donde lo último fue una respuesta del CRM.** WhatsApp exige indicar el último mensaje del chat al archivar, y las respuestas enviadas desde Chatwoot no se guardaban en el historial de AstraCalls. Ahora se guardan, y el archivado va dirigido al identificador interno (LID) del chat. Además, esas respuestas aparecen en el historial que lee el MCP.

### v1.1.7-lhcia.6

- Registros (logs) en cada paso del espejo archivar ↔ resolver, para diagnosticar cuando no se refleja.

### v1.1.7-lhcia.5

- **Configuración global en el panel** (botón **Configurações** en la barra lateral). Primera opción: **llaves MCP de administrador**, que dan una sola conexión MCP a todos los números del servidor (con opción de solo lectura). Se crean con un clic, se muestran una vez y se guardan cifradas. Reemplaza la variable `WACALLS_MCP_ADMIN_KEY` de la lhcia.4, que ya no existe.
- **Bandeja eliminada: nuevo comportamiento** (reemplaza el de la lhcia.4). AstraCalls lo detecta en el momento en que falla una entrega a Chatwoot, y además revisa cada 12 h. Con un 404 confirmado unos minutos después, y solo si antes vio esa bandeja existir: **desvincula el número** de WhatsApp y deja la sesión **24 horas en espera** (configuración, llaves e historial). Si en ese tiempo se liga a una bandeja nueva, sale de la espera. Pasadas 24 h, la sesión se borra por completo. La espera se guarda en la base de datos y sobrevive a reinicios.
- `GET /api/orphans`: lista las sesiones en espera con su cuenta de Chatwoot y la hora de borrado (lo usará el provisioner para ofrecer "Recuperar").

### v1.1.7-lhcia.4

- **Archivar en WhatsApp ↔ resolver en Chatwoot (espejo).** Opción por bandeja (`archive_sync`: desactivado por defecto, `resolved` o `pending`). Archivar el chat en WhatsApp resuelve la conversación (o la deja pendiente); desarchivar la reabre. Resolver en Chatwoot archiva el chat en WhatsApp y reabrir lo desarchiva. Los cambios que hace el propio AstraCalls (importación, abrir al responder, el espejo) no rebotan al otro lado, y la sincronización completa al vincular no resuelve conversaciones en masa.
- **Bandeja eliminada en Chatwoot → sesión eliminada.** Cada 10 minutos AstraCalls revisa que la bandeja de cada sesión exista. Tras 3 respuestas seguidas de "no existe" (404) desvincula el número y borra la sesión. Solo aplica a bandejas que vio existir desde que arrancó; caídas o errores de Chatwoot nunca borran nada.
- **Clave de administrador solo para MCP** (`WACALLS_MCP_ADMIN_KEY`, mínimo 24 caracteres): da acceso MCP a todas las sesiones sin usar la clave maestra, y se puede cambiar sin tocar el provisioner.

### v1.1.7-lhcia.3

- **Responder desde el CRM abre la conversación pendiente.** Cuando un atendente humano responde en una conversación pendiente, pasa a abierta y Captain deja de responder. Las respuestas de Captain, bots o automatizaciones no la abren.

### v1.1.7-lhcia.2

- **Estado de las conversaciones nuevas** (`conversation_status` en la config de Chatwoot de
  cada número): `open` (por defecto, como antes) o `pending`. Con `pending` la conversación
  nueva queda pendiente y el **Captain** (o un bot) la atiende primero; antes AstraCalls la
  creaba siempre abierta y el Captain nunca respondía a clientes nuevos.
- **Importación de historial**: las conversaciones que crea la importación terminan
  **resueltas** (no llenan la bandeja de abiertas y el Captain no responde mensajes viejos);
  se reabren solas cuando el contacto vuelve a escribir. Si el contacto ya tenía una
  conversación activa, se usa esa sin cambiarle el estado.
- **La importación espera la agenda**: al vincular, WhatsApp manda el historial antes que los
  contactos. Ahora AstraCalls guarda el historial y espera la agenda (máximo 3 minutos) antes
  de pasarlo a Chatwoot, así los contactos nacen con su nombre. El equipo recibe avisos en la
  conversación "AstraCalls (sistema)": "⏳ La importación ya comenzó…" y, al terminar,
  "✅ Importación terminada: N conversaciones y M mensajes". Esa conversación de sistema
  siempre entra abierta (el Captain no le responde).
- **Nombres de contacto**: al vincular, WhatsApp manda el historial antes que la agenda y los
  contactos importados quedaban con el número como nombre. Ahora, cuando llega la agenda (o un
  mensaje con el nombre del perfil), AstraCalls corrige el nombre en Chatwoot. Solo cambia
  nombres que son únicamente el número; nunca uno escrito a mano.
- **Leído al responder**: cuando se responde desde Chatwoot, los mensajes de ese contacto se
  marcan como leídos en WhatsApp (el cliente ve los tiques azules). Si se responde desde el
  celular, el propio celular ya los marca. Con "leer automáticamente" no hace falta.
- **Nota por cada llamada**: toda llamada deja una nota privada en la conversación con la
  dirección, el resultado y la duración ("📞 Llamada entrante · contestada · 1:23",
  "· perdida", "· rechazada", "· sin respuesta"). La grabación, cuando la hay, sigue llegando
  aparte con el audio.

### v1.1.7-lhcia.1

- Base actualizada a la **v1.1.7** oficial de AstraCalls (ver su `CHANGELOG.md`): nombre de
  grupos y canales en caché (menos consultas a WhatsApp, menos riesgo de restricción),
  "siempre en línea" apagado por defecto para que el celular siga recibiendo notificaciones
  (botón **Opciones** por número), whatsmeow actualizado, e info de contacto con
  `isBusiness`/`verifiedName`.
- Origen del contacto: solo se trata como anuncio lo que trae `ctwa_clid` o `sourceType`
  "ad". Un enlace wa.me o una búsqueda muestran su origen ("Origen: enlace de WhatsApp", etc.)
  sin la línea "Desde el anuncio" en el mensaje y sin llenar los atributos del anuncio (CAPI).
- Chatwoot: si la búsqueda de contactos de Chatwoot falla con error 500 (pasa cuando un
  contacto conserva un vínculo con una bandeja ya borrada), AstraCalls la repite sin los
  vínculos (`include_contact_inboxes=false`) y encuentra el contacto. Antes intentaba crearlo,
  Chatwoot respondía 422 ("ya existe") y el mensaje terminaba descartado sin llegar a Chatwoot.
  El fallo de la búsqueda ahora queda en el log con su código.

### v1.1.6-lhcia.14

- Panel: el nombre de cada número se puede cambiar (lápiz junto al nombre en la lista; Enter
  guarda, Esc cancela). API: `PATCH /api/sessions/{id}` con `{"name": "..."}`. El nombre nuevo
  también se usa en los avisos que AstraCalls pone en Chatwoot. Una sesión creada por AstraChat
  que todavía no tiene Chatwoot (nombre `inboxN_accM`) no se puede renombrar hasta conectarlo.

### v1.1.6-lhcia.13

- Atributos del anuncio con los nombres de la cuenta: `ctwa_clid` y `meta_ad_id` (los que usa
  CAPI) y, si la cuenta los tiene creados, `anuncio_titulo`, `anuncio_origen` y
  `anuncio_enlace`. Solo se llenan los atributos que existan en la cuenta.

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

### v1.1.7-lhcia.1

- Base updated to the official AstraCalls **v1.1.7** (see its `CHANGELOG.md`): cached group
  and channel names (fewer WhatsApp queries, lower restriction risk), "always online" off by
  default so the phone keeps getting notifications (per-number **Options** button), whatsmeow
  update, and contact info with `isBusiness`/`verifiedName`.
- Contact origin: only `ctwa_clid` or `sourceType` "ad" counts as an ad. A wa.me link or a
  search shows its origin ("Origen: enlace de WhatsApp", etc.) without the "Desde el anuncio"
  line on the message and without filling the ad attributes (CAPI).
- Chatwoot: when Chatwoot's contact search fails with a 500 (happens when a contact keeps a
  link to an already deleted inbox), AstraCalls retries it without the links
  (`include_contact_inboxes=false`) and finds the contact. Before, it tried to create it,
  Chatwoot answered 422 ("already taken") and the message ended up discarded without reaching
  Chatwoot. Search failures are now logged with their status code.

### v1.1.6-lhcia.14

- Panel: each number's name can be changed (pencil next to the name in the list; Enter saves,
  Esc cancels). API: `PATCH /api/sessions/{id}` with `{"name": "..."}`. The new name is also
  used in the notices AstraCalls posts to Chatwoot. A session created by AstraChat that has no
  Chatwoot yet (name `inboxN_accM`) can't be renamed until it's connected.

### v1.1.6-lhcia.13

- Ad attributes use the account's names: `ctwa_clid` and `meta_ad_id` (the ones CAPI uses)
  and, if the account has them, `anuncio_titulo`, `anuncio_origen` and `anuncio_enlace`. Only
  attributes that exist in the account are filled.

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
