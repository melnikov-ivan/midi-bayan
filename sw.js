const CACHE = 'midi-bayan-pwa-v2';

const PRECACHE = [
  './index.html',
  './api.js',
  './ble.js',
  './styles.css',
  './manifest.json',
  './icon.svg',
  './bayan.jpg'
];

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(PRECACHE))
  );
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

// Сначала сеть, при успехе — обновляем кэш; офлайн — из кэша (последняя версия)
self.addEventListener('fetch', (e) => {
  if (e.request.mode !== 'navigate') return;
  e.respondWith(
    fetch(e.request)
      .then((response) => {
        const clone = response.clone();
        caches.open(CACHE).then((cache) => cache.put(e.request, clone));
        return response;
      })
      .catch(() =>
        caches.match(e.request).then((r) => r || caches.match('./index.html'))
      )
  );
});
