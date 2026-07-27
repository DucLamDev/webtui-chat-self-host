import type {
  AddReactionInput,
  ApiEnvelope,
  CursorMeta,
  ForwardMessageInput,
  Message,
  MessageReminder,
  ScheduledMessage,
  ScheduleMessageInput,
  SendMessageInput,
  ThreadDetails,
  UpdateMessageInput
} from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export type MessagePage = {
  messages: Message[];
  meta: CursorMeta;
};

export function createMessagesClient(http: HttpClient) {
  return {
    async list(workspaceId: string, channelId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages`,
        { query: params }
      );
      return collectionFrom<Message>(data, "messages");
    },
    async listPage(workspaceId: string, channelId: string, params: QueryParams = {}): Promise<MessagePage> {
      const envelope = await http.get<ApiEnvelope<{ messages: Message[] }, CursorMeta>>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages`,
        { query: params, unwrap: false }
      );

      return {
        messages: envelope.data?.messages ?? [],
        meta: envelope.meta ?? {}
      };
    },
    send(workspaceId: string, channelId: string, input: SendMessageInput) {
      return http.post<Message>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages`,
        input
      );
    },
    async scheduled(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/scheduled`,
        { query: params }
      );
      return collectionFrom<ScheduledMessage>(data, "scheduled_messages");
    },
    async schedule(workspaceId: string, channelId: string, input: ScheduleMessageInput) {
      const data = await http.post<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/scheduled`,
        input,
        { query: { channel_id: channelId } }
      );
      return itemFrom<ScheduledMessage>(data, "scheduled_message") ?? (data as ScheduledMessage);
    },
    cancelScheduled(workspaceId: string, scheduledMessageId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/scheduled/${encodeURIComponent(scheduledMessageId)}`
      );
    },
    async reminders(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/reminders`,
        { query: params }
      );
      return collectionFrom<MessageReminder>(data, "reminders");
    },
    async createReminder(
      workspaceId: string,
      channelId: string,
      messageId: string,
      input: { remind_at: string; note?: string }
    ) {
      const data = await http.post<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/reminders`,
        input
      );
      return itemFrom<MessageReminder>(data, "reminder") ?? (data as MessageReminder);
    },
    cancelReminder(workspaceId: string, reminderId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/reminders/${encodeURIComponent(reminderId)}`
      );
    },
    async get(workspaceId: string, channelId: string, messageId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}`
      );
      return itemFrom<Message>(data, "message");
    },
    update(workspaceId: string, channelId: string, messageId: string, input: UpdateMessageInput) {
      return http.patch<Message>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}`,
        input
      );
    },
    delete(workspaceId: string, channelId: string, messageId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}`
      );
    },
    forward(workspaceId: string, channelId: string, messageId: string, input: ForwardMessageInput) {
      return http.post<Message>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/forward`,
        input
      );
    },
    async pins(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/pins`
      );
      return collectionFrom<Message>(data, "messages");
    },
    pin(workspaceId: string, channelId: string, messageId: string) {
      return http.post<Message>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/pin`,
        {}
      );
    },
    unpin(workspaceId: string, channelId: string, messageId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/pin`
      );
    },
    async thread(workspaceId: string, channelId: string, messageId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/thread`
      );
      return collectionFrom<Message>(data, "messages");
    },
    async threadDetails(workspaceId: string, channelId: string, messageId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/thread/details`
      );
      return itemFrom<ThreadDetails>(data, "thread") ?? (data as ThreadDetails);
    },
    async updateThreadDetails(
      workspaceId: string,
      channelId: string,
      messageId: string,
      input: { title: string; description: string; status: "open" | "resolved" }
    ) {
      const data = await http.put<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/thread/details`,
        input
      );
      return itemFrom<ThreadDetails>(data, "thread") ?? (data as ThreadDetails);
    },
    async setThreadSubscription(
      workspaceId: string,
      channelId: string,
      messageId: string,
      subscribed: boolean
    ) {
      const data = await http.put<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/thread/subscription`,
        { subscribed }
      );
      return itemFrom<ThreadDetails>(data, "thread") ?? (data as ThreadDetails);
    },
    async markThreadRead(
      workspaceId: string,
      channelId: string,
      messageId: string,
      lastReadMessageId?: string
    ) {
      const data = await http.put<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/thread/read`,
        { last_read_message_id: lastReadMessageId ?? "" }
      );
      return itemFrom<ThreadDetails>(data, "thread") ?? (data as ThreadDetails);
    },
    async threads(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/threads`,
        { query: params }
      );
      return collectionFrom<ThreadDetails>(data, "threads");
    },
    async threadPage(
      workspaceId: string,
      channelId: string,
      messageId: string,
      params: QueryParams = {}
    ): Promise<MessagePage> {
      const envelope = await http.get<ApiEnvelope<{ messages: Message[] }, CursorMeta>>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/thread`,
        { query: params, unwrap: false }
      );

      return {
        messages: envelope.data?.messages ?? [],
        meta: envelope.meta ?? {}
      };
    },
    addReaction(workspaceId: string, channelId: string, messageId: string, input: AddReactionInput) {
      return http.post<Message>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/reactions`,
        input
      );
    },
    removeReaction(workspaceId: string, channelId: string, messageId: string, emoji: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/reactions/${encodeURIComponent(emoji)}`
      );
    },
    async search(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/search`, {
        query: params
      });
      return collectionFrom<Message>(data, "messages");
    },
    async searchPage(workspaceId: string, params: QueryParams = {}): Promise<MessagePage> {
      const envelope = await http.get<ApiEnvelope<{ messages: Message[] }, CursorMeta>>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/messages/search`,
        {
          query: params,
          unwrap: false
        }
      );

      return {
        messages: envelope.data?.messages ?? [],
        meta: envelope.meta ?? {}
      };
    }
  };
}
