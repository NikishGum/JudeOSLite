const CACHE = __CACHE_NAME__;
const FILES = __STATIC_FILES__;
const SHELL = __SHELL_PATH__;
const allowed = new Set(FILES);

self.addEventListener('install', (event) => {
  // A failed/partial install must never replace the working version.
  event.waitUntil((async () => {
    try {
      const cache = await caches.open(CACHE);
      await cache.addAll(FILES.map((path) => new Request(path, { cache: 'reload', credentials: 'omit' })));
    } catch (error) {
      await caches.delete(CACHE);
      throw error;
    }
  })());
});
self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    for (const name of await caches.keys()) {
      if (name.startsWith('pride-shell-') && name !== CACHE) await caches.delete(name);
    }
    await self.clients.claim();
  })());
});
self.addEventListener('message', (event) => {
  if (event.data?.type === 'ACTIVATE_UPDATE') event.waitUntil(self.skipWaiting());
});
self.addEventListener('fetch', (event) => {
  const request = event.request;
  const url = new URL(request.url);
  if (request.method !== 'GET' || url.origin !== self.location.origin || url.pathname === '/api' || url.pathname.startsWith('/api/')) return;
  if (allowed.has(url.pathname) && !url.search) {
    event.respondWith((async () => (await (await caches.open(CACHE)).match(url.pathname)) ?? fetch(request))());
  } else if (request.mode === 'navigate' && /^\/(?:login|home|sessions|participants|groups|journals|statistics|staff|clubs|history|clubs\/[^/]+(?:\/(?:home|sessions|participants|groups|journals|statistics|staff|clubs|history))?)?$/.test(url.pathname)) {
    // Navigations are never stored. The fallback is the public build-time HTML.
    event.respondWith(fetch(new Request(request, { cache: 'no-store' })).catch(async () => {
      return (await (await caches.open(CACHE)).match(SHELL)) ?? Response.error();
    }));
  }
});
