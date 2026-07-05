// Insucar Service Worker — push notifications + offline cache + PWA
var CACHE = 'insucar-v2';
var OFFLINE_URL = '/app';

self.addEventListener('install', function(e) {
  e.waitUntil(caches.open(CACHE).then(function(cache) {
    return cache.addAll([OFFLINE_URL, '/', '/login', '/register']);
  }));
  self.skipWaiting();
});

self.addEventListener('activate', function(e) {
  e.waitUntil(caches.keys().then(function(keys) {
    return Promise.all(keys.filter(function(k) { return k !== CACHE; }).map(function(k) { return caches.delete(k); }));
  }));
  self.clients.claim();
});

self.addEventListener('fetch', function(e) {
  if (e.request.method !== 'GET') return;
  e.respondWith(caches.match(e.request).then(function(cached) {
    return cached || fetch(e.request).then(function(resp) {
      if (resp.ok) { var clone = resp.clone(); caches.open(CACHE).then(function(c) { c.put(e.request, clone); }); }
      return resp;
    }).catch(function() { return cached || caches.match(OFFLINE_URL); });
  }));
});

self.addEventListener('push', function(event) {
  var data = event.data ? event.data.json() : {};
  var title = data.title || 'Insucar';
  var options = {
    body: data.body || 'Update on your assistance request',
    icon: '/favicon.ico', badge: '/favicon.ico',
    vibrate: [200, 100, 200],
    data: { url: data.url || '/app' },
    requireInteraction: true
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', function(e) {
  e.notification.close();
  e.waitUntil(clients.openWindow(e.notification.data.url || '/app'));
});
