import { getPlatformServices } from "@webtui/chat-core";

export type ParsedChatRoute = {
  kind?: "channel" | "dm";
  messageView?: ChatMessageView;
  sectionRef?: string;
  targetRef?: string;
  workspaceRef: string;
};

export type ChatMessageView = "channels" | "conversations";

type ChatRouteSearchParams = {
  get: (name: string) => string | null;
};

export function parseChatRoute(pathname: string, searchParams?: ChatRouteSearchParams): ParsedChatRoute | null {
  const segments = pathname.split("/").filter(Boolean).map(safeDecode);
  if (segments[0] !== "chat" || !segments[1]) {
    return null;
  }
  const desktopQueryRoute = parseDesktopQueryRoute(segments, searchParams);
  if (desktopQueryRoute) {
    return desktopQueryRoute;
  }
  const kind = segments[2] === "channel" || segments[2] === "dm" ? segments[2] : undefined;
  const messageView = parseMessageView(searchParams);
  return {
    workspaceRef: segments[1],
    ...(kind ? { kind, targetRef: segments[3] } : {}),
    ...(messageView ? { messageView } : {}),
    ...(!kind && segments[2] ? { sectionRef: segments[2] } : {})
  };
}

export function buildWorkspaceSectionRoute(workspaceRef: string, sectionRef: string) {
  if (shouldUseDesktopQueryRoute()) {
    const params = new URLSearchParams({ section: sectionRef, workspace: workspaceRef });
    return `/chat/desktop?${params.toString()}`;
  }
  return `${buildChatRoute(workspaceRef)}/${encodeURIComponent(sectionRef)}`;
}

export function buildChatRoute(
  workspaceRef: string,
  kind?: "channel" | "dm",
  targetRef?: string,
  messageView?: ChatMessageView
) {
  if (shouldUseDesktopQueryRoute()) {
    const params = new URLSearchParams({ workspace: workspaceRef });
    if (kind && targetRef) {
      params.set("kind", kind);
      params.set("target", targetRef);
    }
    if (messageView) {
      params.set("view", messageView);
    }
    return `/chat/desktop?${params.toString()}`;
  }
  const base = `/chat/${encodeURIComponent(workspaceRef)}`;
  const route = kind && targetRef ? `${base}/${kind}/${encodeURIComponent(targetRef)}` : base;
  return messageView ? `${route}?view=${encodeURIComponent(messageView)}` : route;
}

export function directRouteRef(name: string, channelId: string) {
  const readableName = routeSlug(name) || "hoi-thoai";
  return `${readableName}--${channelId.slice(0, 8)}`;
}

export function directIdPrefix(reference: string) {
  const marker = reference.lastIndexOf("--");
  return marker >= 0 ? reference.slice(marker + 2) : reference;
}

function routeSlug(value: string) {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[đĐ]/g, "d")
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48);
}

function safeDecode(value: string) {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function parseDesktopQueryRoute(
  segments: string[],
  searchParams?: ChatRouteSearchParams
): ParsedChatRoute | null {
  if (segments[1] !== "desktop" || !searchParams) {
    return null;
  }
  const workspaceRef = searchParams.get("workspace");
  if (!workspaceRef) {
    return null;
  }
  const kindValue = searchParams.get("kind");
  const kind = kindValue === "channel" || kindValue === "dm" ? kindValue : undefined;
  const targetRef = kind ? searchParams.get("target") || undefined : undefined;
  const sectionRef = !kind ? searchParams.get("section") || undefined : undefined;
  const messageView = parseMessageView(searchParams);
  return {
    workspaceRef,
    ...(kind ? { kind, targetRef } : {}),
    ...(sectionRef ? { sectionRef } : {}),
    ...(messageView ? { messageView } : {})
  };
}

function parseMessageView(searchParams?: ChatRouteSearchParams): ChatMessageView | undefined {
  const value = searchParams?.get("view");
  return value === "channels" || value === "conversations" ? value : undefined;
}

function shouldUseDesktopQueryRoute() {
  return getPlatformServices().lifecycle.isDesktop;
}
