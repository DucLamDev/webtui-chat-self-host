import type { CreateTicketInput, Ticket, UpdateTicketInput } from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom } from "./response-utils";

export function createTicketsClient(http: HttpClient) {
  return {
    async list(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/tickets`, {
        query: params
      });
      return collectionFrom<Ticket>(data, "tickets");
    },
    create(workspaceId: string, input: CreateTicketInput) {
      return http.post<Ticket>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/tickets`, input);
    },
    get(workspaceId: string, ticketId: string) {
      return http.get<Ticket>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/tickets/${encodeURIComponent(ticketId)}`
      );
    },
    update(workspaceId: string, ticketId: string, input: UpdateTicketInput) {
      return http.patch<Ticket>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/tickets/${encodeURIComponent(ticketId)}`,
        input
      );
    }
  };
}
