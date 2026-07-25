import type {
  AddChannelMemberInput,
  Channel,
  ChannelMember,
  CreateChannelInput,
  CreateDirectConversationInput,
  DirectConversation,
  UpdateChannelInput,
  UpdateMemberStatusInput,
  UpdateReadStateInput
} from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export function createChannelsClient(http: HttpClient) {
  return {
    async list(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels`);
      return collectionFrom<Channel>(data, "channels");
    },
    create(workspaceId: string, input: CreateChannelInput) {
      return http.post<Channel>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels`, input);
    },
    async get(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}`
      );
      return itemFrom<Channel>(data, "channel");
    },
    update(workspaceId: string, channelId: string, input: UpdateChannelInput) {
      return http.patch<Channel>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}`,
        input
      );
    },
    archive(workspaceId: string, channelId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}`
      );
    },
    async members(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/members`
      );
      return collectionFrom<ChannelMember>(data, "members");
    },
    addMember(workspaceId: string, channelId: string, input: AddChannelMemberInput) {
      return http.post<ChannelMember>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/members`,
        input
      );
    },
    requestJoin(workspaceId: string, channelId: string) {
      return http.post<ChannelMember>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/join-requests`,
        {}
      );
    },
    async joinRequests(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/join-requests`
      );
      return collectionFrom<ChannelMember>(data, "join_requests");
    },
    approveJoinRequest(workspaceId: string, channelId: string, userId: string) {
      return http.post<ChannelMember>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/join-requests/${encodeURIComponent(userId)}/approve`,
        {}
      );
    },
    rejectJoinRequest(workspaceId: string, channelId: string, userId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/join-requests/${encodeURIComponent(userId)}`
      );
    },
    updateMemberStatus(workspaceId: string, channelId: string, userId: string, input: UpdateMemberStatusInput) {
      return http.patch<ChannelMember>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/members/${encodeURIComponent(userId)}`,
        input
      );
    },
    updateReadState(workspaceId: string, channelId: string, input: UpdateReadStateInput) {
      return http.put<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/read-state`,
        input
      );
    },
    openPrivateSession(workspaceId: string, channelId: string) {
      return http.post<Channel>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/private-session`,
        {}
      );
    },
    async directConversations(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/direct-conversations`);
      return collectionFrom<DirectConversation>(data, "direct_conversations");
    },
    createDirectConversation(workspaceId: string, input: CreateDirectConversationInput) {
      return http.post<DirectConversation>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/direct-conversations`,
        input
      );
    }
  };
}
