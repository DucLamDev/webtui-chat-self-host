export function collectionFrom<TItem>(value: unknown, key: string): TItem[] {
  if (Array.isArray(value)) {
    return value as TItem[];
  }

  if (value && typeof value === "object") {
    const item = (value as Record<string, unknown>)[key];
    if (Array.isArray(item)) {
      return item as TItem[];
    }
  }

  return [];
}

export function itemFrom<TItem>(value: unknown, key: string): TItem | null {
  if (value && typeof value === "object") {
    const item = (value as Record<string, unknown>)[key];
    if (item && typeof item === "object") {
      return item as TItem;
    }
  }

  if (value && typeof value === "object") {
    return value as TItem;
  }

  return null;
}
