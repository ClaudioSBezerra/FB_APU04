/*
 * unregister-sw.js
 * STAB-10: limpa service workers órfãos e Cache Storage da origem.
 *
 * Contexto: simu.fcxlabs.com hospedou anteriormente FC Bots, que registrou um
 * service worker. FB_APU04 substituiu FC Bots no mesmo domínio sem SW próprio.
 * Sem este cleanup, o SW antigo intercepta fetch('/') e devolve HTML/cache
 * do FC Bots, fazendo a primeira visita parecer com o app errado.
 *
 * Este script é IDEMPOTENTE — em loads subsequentes (após o cleanup) é no-op.
 * É carregado SÍNCRONO no <head> antes do bundle React, mas suas chamadas a
 * navigator.serviceWorker e caches.* são assíncronas e não bloqueiam render.
 *
 * IMPORTANTE: se o FB_APU04 adotar PWA/Service Worker próprio no futuro,
 * REMOVER este script ANTES de registrar o novo SW (caso contrário ele será
 * desregistrado a cada visita).
 */
(function () {
  try {
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.getRegistrations().then(function (regs) {
        for (var i = 0; i < regs.length; i++) {
          try { regs[i].unregister(); } catch (e) { /* ignore */ }
        }
      }).catch(function () { /* ignore */ });
    }
  } catch (e) { /* ignore */ }
  try {
    if (typeof caches !== 'undefined' && caches && typeof caches.keys === 'function') {
      caches.keys().then(function (keys) {
        for (var i = 0; i < keys.length; i++) {
          try { caches.delete(keys[i]); } catch (e) { /* ignore */ }
        }
      }).catch(function () { /* ignore */ });
    }
  } catch (e) { /* ignore */ }
})();
