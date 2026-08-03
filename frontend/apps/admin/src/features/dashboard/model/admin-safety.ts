export function isAdminUserBlockedStatus(status: unknown): boolean {
  const normalized = String(status ?? "").toLowerCase();
  return normalized === "blocked" || normalized === "disabled" || normalized === "locked";
}

export function escapeCsvCell(value: number | string): string {
  const text = String(value);
  const firstMeaningfulCharacter = Array.from(text).find((character) => character.charCodeAt(0) > 0x20);
  const safeText = firstMeaningfulCharacter && "=+-@".includes(firstMeaningfulCharacter) ? `'${text}` : text;
  return `"${safeText.replaceAll('"', '""')}"`;
}

export function confirmDestructiveAction(message: string): boolean {
  return typeof window !== "undefined" && window.confirm(message);
}
