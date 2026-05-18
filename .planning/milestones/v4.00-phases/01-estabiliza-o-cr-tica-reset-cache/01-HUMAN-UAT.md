---
status: partial
phase: 01-estabiliza-o-cr-tica-reset-cache
source: [01-VERIFICATION.md]
started: 2026-05-08T15:12:00Z
updated: 2026-05-08T15:12:00Z
---

## Current Test

[aguardando teste humano]

## Tests

### 1. Limpeza de SW órfão do FC Bots em browser real

expected: FB_APU04 carrega corretamente na primeira visita pós-deploy (F5 simples, sem Ctrl+Shift+R), em browser com SW do FC Bots registrado no origin simu.fcxlabs.com

Como testar:
1. Em Chrome/Edge, registrar SW dummy via console do DevTools em simu.fcxlabs.com:
   ```js
   navigator.serviceWorker.register('data:application/javascript,self.addEventListener("install",e=>self.skipWaiting());self.addEventListener("activate",e=>e.waitUntil(self.clients.claim()))');
   ```
2. Fazer deploy do build com o fix (ou testar em staging)
3. Fechar aba, reabrir simu.fcxlabs.com/login com F5 simples
4. Verificar em DevTools > Network: `/unregister-sw.js` carregado com `cache-control: no-cache, no-store`
5. Verificar em DevTools > Application > Service Workers: lista vazia após o carregamento

result: [pending]

---

### 2. Screenshot DevTools de browser com SW do FC Bots

expected: Sequência antes/depois — SW do FC Bots visível em DevTools antes do cleanup, lista vazia após carregar simu.fcxlabs.com/login com o fix deployado

result: [pending]

---

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
