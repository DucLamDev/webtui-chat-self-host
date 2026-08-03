/* global self, caches, Request, Response, URL */

const PUSH_DEDUP_CACHE = "webtui-push-dedup-v1";
const MAX_DEDUP_ENTRIES = 100;

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  event.waitUntil(handlePush(event));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(openNotificationTarget(event.notification.data));
});

async function handlePush(event) {
  const payload = readPayload(event.data);
  if (!payload || !payload.id || await wasAlreadyDelivered(payload.id)) {
    return;
  }
  const title = safeText(payload.title, 120) || "WebTui Chat";
  const body = safeText(payload.body, 240) || "Bạn có thông báo mới.";
  const type = safeToken(payload.type, 64) || "notification";
  const tag = safeText(payload.tag, 64) || `${type}-${payload.id}`;
  const target = safeTarget(payload.data?.url);

  await self.registration.showNotification(title, {
    body,
    data: {
      id: payload.id,
      type,
      url: target
    },
    icon: "/brand/vpsttt-logo.png",
    renotify: false,
    requireInteraction: type.startsWith("call_"),
    tag
  });
}

function readPayload(data) {
  if (!data) {
    return null;
  }
  try {
    const payload = data.json();
    if (!payload || typeof payload !== "object" || payload.version !== 1) {
      return null;
    }
    const id = safeToken(payload.id, 128);
    return id ? { ...payload, id } : null;
  } catch {
    return null;
  }
}

async function wasAlreadyDelivered(id) {
  try {
    const cache = await caches.open(PUSH_DEDUP_CACHE);
    const request = dedupRequest(id);
    if (await cache.match(request)) {
      return true;
    }
    await cache.put(request, new Response("1", { headers: { "cache-control": "max-age=604800" } }));
    const keys = await cache.keys();
    await Promise.all(keys.slice(0, Math.max(0, keys.length - MAX_DEDUP_ENTRIES)).map((key) => cache.delete(key)));
    return false;
  } catch {
    return false;
  }
}

function dedupRequest(id) {
  return new Request(`${self.location.origin}/__webtui_push_dedup__/${encodeURIComponent(id)}`);
}

async function openNotificationTarget(data) {
  const target = safeTarget(data?.url);
  const targetURL = new URL(target, self.location.origin).href;
  const windows = await self.clients.matchAll({ includeUncontrolled: true, type: "window" });
  const exact = windows.find((client) => client.url === targetURL);
  if (exact) {
    return exact.focus();
  }
  const existing = windows[0];
  if (existing) {
    if (typeof existing.navigate === "function") {
      await existing.navigate(targetURL);
    }
    return existing.focus();
  }
  return self.clients.openWindow(targetURL);
}

function safeTarget(value) {
  if (typeof value !== "string") {
    return "/";
  }
  try {
    const target = new URL(value, self.location.origin);
    if (target.origin !== self.location.origin || (target.pathname !== "/" && !target.pathname.startsWith("/chat/"))) {
      return "/";
    }
    target.hash = "";
    return `${target.pathname}${target.search}`;
  } catch {
    return "/";
  }
}

function safeText(value, maxLength) {
  if (typeof value !== "string") {
    return "";
  }
  const printable = Array.from(value, (character) => {
    const code = character.charCodeAt(0);
    return code < 32 || code === 127 ? " " : character;
  }).join("");
  return printable.replace(/\s+/g, " ").trim().slice(0, maxLength);
}

function safeToken(value, maxLength) {
  const token = safeText(value, maxLength);
  return /^[a-zA-Z0-9_.:-]+$/.test(token) ? token : "";
}
