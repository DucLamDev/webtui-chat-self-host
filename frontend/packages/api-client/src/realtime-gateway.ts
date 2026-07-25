export type RealtimeGatewayOptions = {
  accessToken: string;
  authMode?: "query" | "subprotocol";
  baseUrl: string;
  protocols?: string[];
  workspaceId?: string;
};

export type RealtimeEventHandler = (event: MessageEvent<string>) => void;

export type RealtimeServerEvent<TPayload = unknown> = {
  payload?: TPayload;
  room?: string;
  timestamp?: string;
  type: string;
  user_id?: string;
};

export type RealtimeCommand = {
  payload?: unknown;
  room: string;
  type: "join" | "leave" | string;
};

export function createRealtimeGateway(defaultBaseUrl: string) {
  return {
    connect(options: Omit<RealtimeGatewayOptions, "baseUrl">) {
      const url = new URL(defaultBaseUrl);
      const authMode = options.authMode ?? "query";

      if (options.workspaceId) {
        url.searchParams.set("workspace_id", options.workspaceId);
      }

      if (authMode === "query") {
        url.searchParams.set("access_token", options.accessToken);
        return new WebSocket(url.toString(), options.protocols);
      }

      return new WebSocket(url.toString(), ["webtui.jwt", options.accessToken, ...(options.protocols ?? [])]);
    },
    subscribe(socket: WebSocket, handler: RealtimeEventHandler) {
      socket.addEventListener("message", handler);

      return () => socket.removeEventListener("message", handler);
    },
    send(socket: WebSocket, command: RealtimeCommand) {
      if (socket.readyState !== WebSocket.OPEN) {
        return false;
      }

      socket.send(JSON.stringify(command));
      return true;
    },
    join(socket: WebSocket, room: string) {
      return this.send(socket, { room, type: "join" });
    },
    leave(socket: WebSocket, room: string) {
      return this.send(socket, { room, type: "leave" });
    }
  };
}
