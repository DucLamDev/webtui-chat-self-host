type MediaCacheEntry = {
  lastUsedAt: number;
  promise: Promise<string>;
  url?: string;
};

const mediaCache = new Map<string, MediaCacheEntry>();
const maxEntries = 120;
const minEvictionAgeMs = 10 * 60_000;

export function getCachedMediaUrl(key: string): string | undefined {
  const entry = mediaCache.get(key);
  if (entry?.url) {
    entry.lastUsedAt = Date.now();
  }
  return entry?.url;
}

export function resolveCachedMediaUrl(key: string, loader: () => Promise<Blob>): Promise<string> {
  const existing = mediaCache.get(key);
  if (existing) {
    existing.lastUsedAt = Date.now();
    return existing.promise;
  }

  const entry: MediaCacheEntry = {
    lastUsedAt: Date.now(),
    promise: Promise.resolve("")
  };
  entry.promise = loader()
    .then((blob) => {
      entry.url = URL.createObjectURL(blob);
      evictExpiredMedia();
      return entry.url;
    })
    .catch((error) => {
      mediaCache.delete(key);
      throw error;
    });
  mediaCache.set(key, entry);
  return entry.promise;
}

export function clearMediaObjectUrlCache() {
  mediaCache.forEach((entry) => {
    if (entry.url) {
      URL.revokeObjectURL(entry.url);
    }
  });
  mediaCache.clear();
}

function evictExpiredMedia() {
  if (mediaCache.size <= maxEntries) {
    return;
  }

  const threshold = Date.now() - minEvictionAgeMs;
  const candidates = [...mediaCache.entries()]
    .filter(([, entry]) => entry.lastUsedAt < threshold)
    .sort((left, right) => left[1].lastUsedAt - right[1].lastUsedAt);

  for (const [key, entry] of candidates) {
    if (mediaCache.size <= maxEntries) {
      break;
    }
    if (entry.url) {
      URL.revokeObjectURL(entry.url);
    }
    mediaCache.delete(key);
  }
}
