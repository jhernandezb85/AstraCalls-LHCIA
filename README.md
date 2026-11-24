<div align="center">

# 📞 AstraCalls LHCIA

**Fork de [AstraCalls](https://github.com/AstraOnlineWeb/AstraCalls) con mejoras para Chatwoot.**
**A fork of [AstraCalls](https://github.com/AstraOnlineWeb/AstraCalls) with Chatwoot improvements.**

[![Imagen](https://img.shields.io/badge/imagen-ghcr.io%2Fjhernandezb85%2Fastracalls-2496ED?logo=docker&logoColor=white)](https://github.com/jhernandezb85/AstraCalls-LHCIA/pkgs/container/astracalls)
[![Base](https://img.shields.io/badge/base-AstraCalls%20v1.1.6-8A2BE2)](https://github.com/AstraOnlineWeb/AstraCalls)
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](./LICENSE)

[Español](#español) · [English](#english) · [Cambios / Changelog](./CHANGELOG-LHCIA.md) · [Documentación original / Original docs](./README.upstream.md)

</div>

---

## Español

### Qué es

AstraCalls conecta números de WhatsApp (como aparato vinculado) para llamadas de voz y video
desde el navegador, mensajes, webhooks e integración con Chatwoot. Este fork mantiene todo eso
y mejora la experiencia en Chatwoot para equipos de atención y ventas.

La documentación completa del proyecto original (API, configuración, llamadas, SIP, n8n, MCP)
sigue vigente y está en [README.upstream.md](./README.upstream.md) (en portugués).

### Qué agrega este fork

**Lo que escribes desde el celular**
- Aparece en Chatwoot como mensaje normal con el título "📱 Enviado desde el celular" (o como
  nota privada, según la opción de la bandeja).
- Lo publica un bot de la cuenta ("WhatsApp (celular)"), así Chatwoot no lo cuenta como
  respuesta humana. Con la opción por defecto, la conversación pasa a "abierta" y Captain deja
  de responder: una persona tomó el control.

**Checks y envíos fallidos**
- ✓✓ entregado y leído en los mensajes, como en WhatsApp.
- Si un mensaje no sale, queda marcado como fallido con el motivo en español.
- "Reenviar" en Chatwoot ahora sí lo reenvía por WhatsApp.
- Los envíos lentos (grupos grandes) ya no se cortan cuando Chatwoot deja de esperar.

**Mensajes eliminados**
- El cliente elimina un mensaje: se conserva con una nota "🗑️ El contacto eliminó este mensaje"
  (o se elimina también, según la opción).
- Tú eliminas para todos (en el celular o en Chatwoot): se elimina en ambos lados.

**Anuncios (Click to WhatsApp)**
- Nota privada con los datos del anuncio (título, descripción, enlace, ID, `ctwa_clid`).
- Línea corta del anuncio en el primer mensaje del cliente, para que Captain sepa de qué
  producto se habla.

**Otros**
- Textos que AstraCalls pone en Chatwoot traducidos al español.
- La relación mensaje WhatsApp ↔ Chatwoot se guarda en Postgres (sobrevive reinicios).

### Opciones nuevas (por bandeja)

Se configuran en `PUT /api/sessions/{id}/chatwoot` (o desde el
[provisioner](https://github.com/jhernandezb85/astracalls-provisioner)).

| Campo | Valores | Por defecto | Qué hace |
|---|---|---|---|
| `device_mode` | `open` · `keep` · `note` | `open` | Cómo entra lo escrito desde el celular: mensaje que abre la conversación, mensaje sin cambiar el estado, o nota privada |
| `device_title` | texto | `📱 Enviado desde el celular` | Título arriba de cada mensaje del celular |
| `device_bot_token` | token | — | Token del Agent Bot que publica los mensajes del celular (nunca se devuelve) |
| `read_receipts` | `true` · `false` | `true` | Checks de entregado y leído, y fallidos con motivo |
| `contact_delete` | `note` · `delete` | `note` | Qué hacer cuando el cliente elimina un mensaje |

### Instalación

Imagen (amd64): `ghcr.io/jhernandezb85/astracalls`

```yaml
services:
  astracalls:
    image: 'ghcr.io/jhernandezb85/astracalls:latest'
```

- `latest` apunta siempre a la última versión publicada.
- Cada versión también queda fija como `vX.Y.Z-lhcia.N` (ej. `v1.1.6-lhcia.10`).
- Volver atrás: poner en el compose una versión anterior, o la oficial
  `astraonline/astracalls:v1.1.6`.

El resto de la instalación (variables de entorno, Postgres, proxy) es igual al original.

### Versiones

Formato `vX.Y.Z-lhcia.N`: `vX.Y.Z` es la versión de AstraCalls en la que se basa y `N` la
revisión propia. Todos los cambios están en [CHANGELOG-LHCIA.md](./CHANGELOG-LHCIA.md).

### Créditos y licencia

Basado en [AstraCalls](https://github.com/AstraOnlineWeb/AstraCalls), que a su vez es un fork
de [WaCalls](https://github.com/JotaDev66/WaCalls) (MIT, aviso en
[LICENSE.WaCalls](./LICENSE.WaCalls)). Distribuido bajo **GNU AGPL-3.0**
([LICENSE](./LICENSE)): el código de este fork es público, como exige la licencia para uso en
red (SaaS).

---

## English

### What it is

AstraCalls links WhatsApp numbers (as a companion device) for browser voice and video calls,
messaging, webhooks and Chatwoot integration. This fork keeps all of that and improves the
Chatwoot experience for support and sales teams.

The original project's full documentation (API, configuration, calls, SIP, n8n, MCP) still
applies and is in [README.upstream.md](./README.upstream.md) (in Portuguese).

### What this fork adds

**Messages written on the phone**
- Show up in Chatwoot as regular messages titled "📱 Enviado desde el celular" (or as private
  notes, depending on the inbox option).
- Posted by an account bot ("WhatsApp (celular)"), so Chatwoot doesn't count them as a human
  reply. With the default option the conversation becomes "open" and Captain stops replying:
  a person took over.

**Ticks and failed sends**
- ✓✓ delivered and read on messages, like WhatsApp.
- A message that can't be sent is marked failed, with the reason (in Spanish).
- Chatwoot's "Retry" now really resends it through WhatsApp.
- Slow sends (large groups) are no longer cut off when Chatwoot stops waiting.

**Deleted messages**
- The customer deletes a message: it's kept with a "🗑️ El contacto eliminó este mensaje" note
  (or deleted too, depending on the option).
- You delete for everyone (on the phone or in Chatwoot): it's deleted on both sides.

**Ads (Click to WhatsApp)**
- Private note with the ad data (title, description, link, ID, `ctwa_clid`).
- A short ad line on the customer's first message, so Captain knows which product it's about.

**Other**
- Texts AstraCalls posts to Chatwoot are translated to Spanish.
- The WhatsApp ↔ Chatwoot message link is stored in Postgres (survives restarts).

### New options (per inbox)

Set through `PUT /api/sessions/{id}/chatwoot` (or the
[provisioner](https://github.com/jhernandezb85/astracalls-provisioner)).

| Field | Values | Default | What it does |
|---|---|---|---|
| `device_mode` | `open` · `keep` · `note` | `open` | How phone messages come in: message that opens the conversation, message without changing status, or private note |
| `device_title` | text | `📱 Enviado desde el celular` | Title above each phone message |
| `device_bot_token` | token | — | Agent Bot token used to post phone messages (never returned) |
| `read_receipts` | `true` · `false` | `true` | Delivered/read ticks and failed sends with reason |
| `contact_delete` | `note` · `delete` | `note` | What to do when the customer deletes a message |

### Install

Image (amd64): `ghcr.io/jhernandezb85/astracalls`

```yaml
services:
  astracalls:
    image: 'ghcr.io/jhernandezb85/astracalls:latest'
```

- `latest` always points to the latest published version.
- Each version is also pinned as `vX.Y.Z-lhcia.N` (e.g. `v1.1.6-lhcia.10`).
- Roll back: put a previous version in the compose, or the official
  `astraonline/astracalls:v1.1.6`.

The rest of the setup (environment variables, Postgres, proxy) is the same as the original.

### Versions

Format `vX.Y.Z-lhcia.N`: `vX.Y.Z` is the AstraCalls version it's based on and `N` our own
revision. Every change is listed in [CHANGELOG-LHCIA.md](./CHANGELOG-LHCIA.md).

### Credits and license

Based on [AstraCalls](https://github.com/AstraOnlineWeb/AstraCalls), itself a fork of
[WaCalls](https://github.com/JotaDev66/WaCalls) (MIT, notice in
[LICENSE.WaCalls](./LICENSE.WaCalls)). Distributed under **GNU AGPL-3.0**
([LICENSE](./LICENSE)): this fork's source code is public, as the license requires for network
(SaaS) use.
