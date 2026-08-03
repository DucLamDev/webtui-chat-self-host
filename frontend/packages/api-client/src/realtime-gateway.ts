export type RealtimeGatewayOptions = {
  accessToken: string;
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

      if (options.workspaceId) {
        url.searchParams.set("workspace_id", options.workspaceId);
      }

      // Browser WebSockets cannot set Authorization. Carry the JWT in the
      // subprotocol request header so it never appears in URLs or access logs.
      return new WebSocket(url.toString(), [
        `webtui.jwt.${options.accessToken}`,
        ...(options.protocols ?? [])
      ]);
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
