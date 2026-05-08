# Diagnóstico STAB-10: cache stale em simu.fcxlabs.com/login

**Data:** 2026-05-08
**Sintoma reproduzido?** parcial (evidência coletada via curl; inspeção DevTools em browser infectado ainda pendente de confirmação de usuário real — ver passo 3)

---

## Headers servidos em prod

```
=== HEAD / ===
HTTP/1.1 200 OK
Server: nginx/1.29.2
Date: Fri, 08 May 2026 17:43:06 GMT
Content-Type: text/html
Content-Length: 473
Cache-Control: no-cache, no-store, must-revalidate
Pragma: no-cache
Expires: 0
Last-Modified: Thu, 07 May 2026 18:42:01 GMT
ETag: "69fcdcf9-1d9"
Strict-Transport-Security: max-age=63072000; includeSubDomains
X-Content-Type-Options: nosniff

=== GET /index.html (first 30 lines) ===
<!doctype html>
<html lang="pt-BR">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/png" href="/logo.png" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>FBTax Cloud — Simulador RT</title>
    <script type="module" crossorigin src="/assets/index-D_NfZdEx.js"></script>
    <link rel="stylesheet" crossorigin href="/assets/index-CLTmyb36.css">
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
```

**Observações:**
- Title: "FBTax Cloud — Simulador RT" → prod está servindo FB_APU04 corretamente
- Cache-Control: `no-cache, no-store, must-revalidate` em `/index.html` e `/` → nginx não é a fonte do problema
- Sem header `Age:` ou `Via:` → Traefik/Coolify não está adicionando cache de borda
- Sem header `X-Cache:` → sem CDN (Cloudflare) na frente

### SW-style paths retornam 200 mas é o index.html (fallback)

```
--- /sw.js ---     → HTTP 200, Content-Type: text/html, 473 bytes = index.html (try_files fallback)
--- /service-worker.js ---    → HTTP 200, Content-Type: text/html, 473 bytes = index.html
--- /firebase-messaging-sw.js --- → HTTP 200, Content-Type: text/html, 473 bytes = index.html
--- /workbox-sw.js ---        → HTTP 200, Content-Type: text/html, 473 bytes = index.html
```

**Todos os paths de SW retornam status 200 porque o nginx usa `try_files $uri $uri/ /index.html`** — nenhum desses arquivos existe de fato no servidor. O nginx está servindo index.html como fallback. Isso descarta a hipótese de que o SW antigo ainda estaria fisicamente no servidor sendo servido.

---

## Artefatos legados no repo

```bash
# frontend/public: nenhum arquivo .js ou .html presente
# frontend/dist: sem sw.js, service-worker.js, ou firebase-messaging-sw.js
# frontend/src: zero referências a navigator.serviceWorker
# Nenhuma referência a workbox, vite-plugin-pwa, ou registerServiceWorker
# Nenhuma referência a "FC Bots", "fc-bots", "fcbots"
```

**Conclusão:** O codebase do FB_APU04 está limpo — não há vestígios de service worker próprio nem de FC Bots no código-fonte.

---

## Inspeção DevTools (resultado do passo 3)

> **Nota:** Este passo requer um navegador com histórico real do domínio simu.fcxlabs.com para ser executado. Os passos 1 e 2 (curl + análise de artefatos) fornecem evidência suficiente para o diagnóstico.

**Análise baseada nos dados coletados:**

O cenário é o seguinte:
- O nginx do FB_APU04 serve `index.html` com headers `no-cache, no-store` — ou seja, o servidor está correto
- Não há SW no servidor servindo HTML antigo do FC Bots
- FC Bots era o app anterior nesse domínio e tipicamente usa Firebase + workbox (gerado por CRA ou similar)
- Quando FC Bots foi substituído, o SW do FC Bots já estava registrado nos browsers dos usuários no scope `/` de simu.fcxlabs.com

**Mecanismo do bug:**
1. Usuário visitou simu.fcxlabs.com quando FC Bots estava lá → SW do FC Bots registrou-se no scope `/`
2. FB_APU04 foi deploiado no mesmo domínio (sem SW próprio)
3. Na próxima visita do usuário ao simu.fcxlabs.com/login, o SW do FC Bots ainda está ativo no browser
4. O SW intercept o `fetch('/')` e responde com o cache do FC Bots (HTML antigo) — **o servidor nunca é consultado**
5. `Ctrl+Shift+R` (hard reload) bypassa o SW e funciona porque força request direto ao servidor
6. Uma sessão "normal" (F5 ou fechar/abrir aba) não bypassa o SW

**Confirmação indireta:** O fato de `Ctrl+Shift+R` resolver o problema (conforme descrição do STAB-10) é o sintoma clássico de SW órfão interceptando requests.

---

## Causa raiz

**Service Worker órfão do FC Bots registrado no browser dos usuários no escopo `/` de simu.fcxlabs.com.**

Quando o FB_APU04 substituiu o FC Bots no mesmo domínio, os browsers de usuários que visitaram o FC Bots mantiveram o SW antigo registrado e ativo. Este SW intercepta requests de navegação (`fetch('/')`) e responde com o HTML cacheado do FC Bots, fazendo o usuário ver o app antigo. O hard reload (`Ctrl+Shift+R`) bypassa o SW e permite ao browser buscar diretamente do servidor, que já serve o FB_APU04 corretamente.

**Evidências:**
1. Headers do servidor confirmam que nginx serve FB_APU04 com `no-store` — servidor não é a causa
2. Nenhum arquivo SW existe no codebase ou nos paths `/sw.js`, `/service-worker.js` etc (retornam index.html via fallback)
3. O comportamento de "Ctrl+Shift+R resolve, F5 não resolve" é diagnóstico de SW ativo interceptando navegação
4. FB_APU04 não registra SW próprio (sem vite-plugin-pwa, sem workbox, sem navigator.serviceWorker no código)

---

## Plano de fix

- [x] **option A: unregister-sw.js** — script JS vanilla no `<head>` do index.html que:
  - Desregistra todos os SWs registrados no origin via `navigator.serviceWorker.getRegistrations()`
  - Limpa todo o Cache Storage via `caches.keys()` + `caches.delete()`
  - Idempotente: no-op se não há SW/cache
  - Executado síncrono no parser antes do bundle React (sem defer/async)
- [x] Adicionar `<script src="/unregister-sw.js"></script>` no `<head>` do `index.html` ANTES do bundle
- [x] Adicionar `location = /unregister-sw.js` no nginx com `Cache-Control: no-store` (garantir que o próprio cleanup script nunca seja cacheado)

Option B (Clear-Site-Data) descartada: risco de apagar LocalStorage que pode conter estado de sessão, e não tem suporte em Safari ≤14.

---

## Riscos

- **Se FB_APU04 adotar PWA/SW próprio no futuro:** `unregister-sw.js` desregistrará o SW legítimo a cada visita, quebrando o PWA. O script DEVE ser removido ANTES de registrar qualquer SW próprio.
- **Usuários sem histórico de FC Bots:** cleanup é no-op; nenhum impacto.
- **Browsers muito antigos sem `serviceWorker` API:** `try/catch` e verificação `'serviceWorker' in navigator` cobrem o fallback silencioso.
- **Cache Storage limpo continha dados de navegação:** o FB_APU04 não usa Cache Storage para nada — auth vive em React state + httpOnly cookie. Limpar não desloga o usuário.
