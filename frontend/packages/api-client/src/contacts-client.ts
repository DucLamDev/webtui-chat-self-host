import type { ContactRequest } from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export type SendContactRequestInput = {
  user_id: string;
};

export function createContactsClient(http: HttpClient) {
  return {
    async list() {
      const data = await http.get<unknown>("/api/v1/contacts");
      return collectionFrom<ContactRequest>(data, "contacts");
    },
    async requests(params: QueryParams = {}) {
      const data = await http.get<unknown>("/api/v1/contact-requests", { query: params });
      return collectionFrom<ContactRequest>(data, "contact_requests");
    },
    async sendRequest(input: SendContactRequestInput) {
      const data = await http.post<unknown>("/api/v1/contact-requests", input);
      return itemFrom<ContactRequest>(data, "contact_request");
    },
    async acceptRequest(requestId: string) {
      const data = await http.post<unknown>(`/api/v1/contact-requests/${encodeURIComponent(requestId)}/accept`, {});
      return itemFrom<ContactRequest>(data, "contact_request");
    },
    async rejectRequest(requestId: string) {
      const data = await http.post<unknown>(`/api/v1/contact-requests/${encodeURIComponent(requestId)}/reject`, {});
      return itemFrom<ContactRequest>(data, "contact_request");
    },
    cancelRequest(requestId: string) {
      return http.delete<void>(`/api/v1/contact-requests/${encodeURIComponent(requestId)}`);
    }
  };
}
