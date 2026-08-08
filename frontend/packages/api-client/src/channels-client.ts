import type {
  AddChannelMemberInput,
  BreakoutRoom,
  Channel,
  ChannelMeeting,
  ChannelRecording,
  ChannelTask,
  ChannelTaskStatus,
  ChannelVoiceRoom,
  ChannelMember,
  CollaborationDocument,
  CollaborationDocumentKind,
  CollaborationParticipantRole,
  CollaborationRole,
  CollaborationSettings,
  CreateChannelTaskInput,
  CreateChannelMeetingInput,
  CreateChannelInput,
  CreatePublicConversationLinkInput,
  CreateDirectConversationInput,
  DirectConversation,
  GuestJoinRequest,
  FederationInvite,
  JsonObject,
  PublicConversationLink,
  PublicConversationRoom,
  RecordingPolicy,
  SetupBreakoutRoomsInput,
  SharedItem,
  TalkHome,
  TalkIntegration,
  TalkSummary,
  UpdateCollaborationSettingsInput,
  UpdateChannelInput,
  UpdateMemberStatusInput,
  UpdateReadStateInput
} from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export function createChannelsClient(http: HttpClient) {
  const collaborationPath = (workspaceId: string, channelId: string) =>
    `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/collaboration`;

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
    },
    async collaborationSettings(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(collaborationPath(workspaceId, channelId));
      return itemFrom<CollaborationSettings>(data, "settings") ?? (data as CollaborationSettings);
    },
    async updateCollaborationSettings(
      workspaceId: string,
      channelId: string,
      input: UpdateCollaborationSettingsInput
    ) {
      const data = await http.put<unknown>(collaborationPath(workspaceId, channelId), input);
      return itemFrom<CollaborationSettings>(data, "settings") ?? (data as CollaborationSettings);
    },
    async promoteConversation(workspaceId: string, channelId: string, name: string) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/promote`, { name });
      return itemFrom<CollaborationSettings>(data, "settings") ?? (data as CollaborationSettings);
    },
    async createPublicLink(
      workspaceId: string,
      channelId: string,
      input: CreatePublicConversationLinkInput
    ) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/public-link`, input);
      return itemFrom<PublicConversationLink>(data, "link") ?? (data as PublicConversationLink);
    },
    async disablePublicLink(workspaceId: string, channelId: string) {
      const data = await http.delete<unknown>(`${collaborationPath(workspaceId, channelId)}/public-link`);
      return itemFrom<CollaborationSettings>(data, "settings") ?? (data as CollaborationSettings);
    },
    async publicRoom(publicToken: string) {
      const data = await http.get<unknown>(
        `/api/v1/public/conversations/${encodeURIComponent(publicToken)}`,
        { auth: false }
      );
      return itemFrom<PublicConversationRoom>(data, "room") ?? (data as PublicConversationRoom);
    },
    async joinPublicRoom(publicToken: string, input: {
      display_name: string;
      password?: string;
      privacy_accepted: true;
      privacy_version: string;
      terms_accepted: true;
      terms_version: string;
    }) {
      const data = await http.post<unknown>(
        `/api/v1/public/conversations/${encodeURIComponent(publicToken)}/join`,
        input,
        { auth: false }
      );
      return itemFrom<GuestJoinRequest>(data, "guest") ?? (data as GuestJoinRequest);
    },
    async publicJoinStatus(publicToken: string, requestId: string, accessToken: string) {
      const data = await http.get<unknown>(
        `/api/v1/public/conversations/${encodeURIComponent(publicToken)}/join/${encodeURIComponent(requestId)}`,
        { auth: false, query: { access_token: accessToken } }
      );
      return itemFrom<GuestJoinRequest>(data, "guest") ?? (data as GuestJoinRequest);
    },
    async guestRequests(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/guests`);
      return collectionFrom<GuestJoinRequest>(data, "guests");
    },
    async moderateGuest(
      workspaceId: string,
      channelId: string,
      requestId: string,
      action: "approve" | "reject"
    ) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/guests/${encodeURIComponent(requestId)}/${action}`,
        {}
      );
      return itemFrom<GuestJoinRequest>(data, "guest") ?? (data as GuestJoinRequest);
    },
    async collaborationRoles(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/roles`);
      return collectionFrom<CollaborationRole>(data, "roles");
    },
    async updateCollaborationRole(
      workspaceId: string,
      channelId: string,
      userId: string,
      role: CollaborationParticipantRole
    ) {
      const data = await http.patch<unknown>(
        `${collaborationPath(workspaceId, channelId)}/roles/${encodeURIComponent(userId)}`,
        { role }
      );
      return itemFrom<CollaborationRole>(data, "role") ?? (data as CollaborationRole);
    },
    async collaborationDocument(
      workspaceId: string,
      channelId: string,
      kind: CollaborationDocumentKind
    ) {
      const data = await http.get<unknown>(
        `${collaborationPath(workspaceId, channelId)}/documents/${encodeURIComponent(kind)}`
      );
      return itemFrom<CollaborationDocument>(data, "document") ?? (data as CollaborationDocument);
    },
    async updateCollaborationDocument(
      workspaceId: string,
      channelId: string,
      kind: CollaborationDocumentKind,
      content: JsonObject,
      expectedVersion: number
    ) {
      const data = await http.put<unknown>(
        `${collaborationPath(workspaceId, channelId)}/documents/${encodeURIComponent(kind)}`,
        { content, expected_version: expectedVersion }
      );
      return itemFrom<CollaborationDocument>(data, "document") ?? (data as CollaborationDocument);
    },
    async channelTasks(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/tasks`);
      return collectionFrom<ChannelTask>(data, "tasks");
    },
    async createChannelTask(workspaceId: string, channelId: string, input: CreateChannelTaskInput) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/tasks`, input);
      return itemFrom<ChannelTask>(data, "task") ?? (data as ChannelTask);
    },
    async updateChannelTask(
      workspaceId: string,
      channelId: string,
      taskId: string,
      input: { status: ChannelTaskStatus; assignee_user_id?: string | null; due_at?: string | null }
    ) {
      const data = await http.patch<unknown>(
        `${collaborationPath(workspaceId, channelId)}/tasks/${encodeURIComponent(taskId)}`,
        input
      );
      return itemFrom<ChannelTask>(data, "task") ?? (data as ChannelTask);
    },
    async breakoutRooms(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/breakouts`);
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async createBreakoutRoom(
      workspaceId: string,
      channelId: string,
      input: { name: string; assigned_user_ids: string[] }
    ) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/breakouts`, input);
      return itemFrom<BreakoutRoom>(data, "breakout_room") ?? (data as BreakoutRoom);
    },
    async closeBreakoutRoom(workspaceId: string, channelId: string, roomId: string) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/breakouts/${encodeURIComponent(roomId)}/close`,
        {}
      );
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async returnBreakoutRooms(workspaceId: string, channelId: string) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/breakouts/return`, {});
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async setupBreakoutRooms(workspaceId: string, channelId: string, input: SetupBreakoutRoomsInput) {
      const data = await http.put<unknown>(`${collaborationPath(workspaceId, channelId)}/breakouts/setup`, input);
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async startBreakoutRooms(workspaceId: string, channelId: string) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/breakouts/start`, {});
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async joinBreakoutRoom(workspaceId: string, channelId: string, roomId: string) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/breakouts/${encodeURIComponent(roomId)}/join`,
        {}
      );
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async updateBreakoutAssignments(
      workspaceId: string,
      channelId: string,
      roomId: string,
      assignedUserIds: string[]
    ) {
      const data = await http.put<unknown>(
        `${collaborationPath(workspaceId, channelId)}/breakouts/${encodeURIComponent(roomId)}/assignments`,
        { assigned_user_ids: assignedUserIds }
      );
      return collectionFrom<BreakoutRoom>(data, "breakout_rooms");
    },
    async broadcastToBreakouts(workspaceId: string, channelId: string, body: string) {
      return http.post<{ id: string }>(
        `${collaborationPath(workspaceId, channelId)}/breakouts/broadcast`,
        { body }
      );
    },
    async meetings(workspaceId: string, channelId: string, params: { from?: string; to?: string } = {}) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/meetings`, {
        query: params
      });
      return collectionFrom<ChannelMeeting>(data, "meetings");
    },
    async createMeeting(
      workspaceId: string,
      channelId: string,
      input: CreateChannelMeetingInput
    ) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/meetings`, input);
      return itemFrom<ChannelMeeting>(data, "meeting") ?? (data as ChannelMeeting);
    },
    async transitionMeeting(
      workspaceId: string,
      channelId: string,
      meetingId: string,
      action: "start" | "end" | "cancel"
    ) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/meetings/${encodeURIComponent(meetingId)}/${action}`,
        {}
      );
      return itemFrom<ChannelMeeting>(data, "meeting") ?? (data as ChannelMeeting);
    },
    async voiceRoom(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/voice-room`);
      return itemFrom<ChannelVoiceRoom>(data, "voice_room") ?? (data as ChannelVoiceRoom);
    },
    async startVoiceRoom(workspaceId: string, channelId: string) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/voice-room/start`, {});
      return itemFrom<ChannelVoiceRoom>(data, "voice_room") ?? (data as ChannelVoiceRoom);
    },
    async stopVoiceRoom(workspaceId: string, channelId: string) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/voice-room/stop`, {});
      return itemFrom<ChannelVoiceRoom>(data, "voice_room") ?? (data as ChannelVoiceRoom);
    },
    async sharedItems(
      workspaceId: string,
      channelId: string,
      params: { kind?: string; limit?: number } = {}
    ) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/shared-items`, {
        query: params
      });
      return collectionFrom<SharedItem>(data, "items");
    },
    async summarizeChannel(
      workspaceId: string,
      channelId: string,
      input: { since?: string; language?: string } = {}
    ) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/ai/summary`,
        input
      );
      return itemFrom<TalkSummary>(data, "summary") ?? (data as TalkSummary);
    },
    async recordingPolicy(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/recording-policy`);
      return itemFrom<RecordingPolicy>(data, "recording_policy") ?? (data as RecordingPolicy);
    },
    async updateRecordingPolicy(
      workspaceId: string,
      channelId: string,
      input: Omit<RecordingPolicy, "channel_id" | "workspace_id" | "updated_by" | "created_at" | "updated_at">
    ) {
      const data = await http.put<unknown>(`${collaborationPath(workspaceId, channelId)}/recording-policy`, input);
      return itemFrom<RecordingPolicy>(data, "recording_policy") ?? (data as RecordingPolicy);
    },
    async recordings(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/recordings`);
      return collectionFrom<ChannelRecording>(data, "recordings");
    },
    async startRecording(
      workspaceId: string,
      channelId: string,
      input: { meeting_id?: string; participant_user_ids?: string[] }
    ) {
      const data = await http.post<unknown>(`${collaborationPath(workspaceId, channelId)}/recordings`, input);
      return itemFrom<ChannelRecording>(data, "recording") ?? (data as ChannelRecording);
    },
    async setRecordingConsent(
      workspaceId: string,
      channelId: string,
      recordingId: string,
      consented: boolean
    ) {
      const data = await http.put<unknown>(
        `${collaborationPath(workspaceId, channelId)}/recordings/${encodeURIComponent(recordingId)}/consent`,
        { consented }
      );
      return itemFrom<ChannelRecording>(data, "recording") ?? (data as ChannelRecording);
    },
    async stopRecording(workspaceId: string, channelId: string, recordingId: string) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/recordings/${encodeURIComponent(recordingId)}/stop`,
        {}
      );
      return itemFrom<ChannelRecording>(data, "recording") ?? (data as ChannelRecording);
    },
    async talkHome(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/talk/home`);
      return itemFrom<TalkHome>(data, "home") ?? (data as TalkHome);
    },
    async talkIntegration(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/talk/integrations`);
      return itemFrom<TalkIntegration>(data, "integration") ?? (data as TalkIntegration);
    },
    async updateTalkIntegration(
      workspaceId: string,
      input: Omit<TalkIntegration, "workspace_id" | "updated_by" | "created_at" | "updated_at">
    ) {
      const data = await http.put<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/talk/integrations`,
        input
      );
      return itemFrom<TalkIntegration>(data, "integration") ?? (data as TalkIntegration);
    },
    async federationInvites(workspaceId: string, channelId: string) {
      const data = await http.get<unknown>(`${collaborationPath(workspaceId, channelId)}/federation-invites`);
      return collectionFrom<FederationInvite>(data, "invites");
    },
    async createFederationInvite(
      workspaceId: string,
      channelId: string,
      input: { remote_server: string; remote_user: string; protocol?: string; payload?: JsonObject }
    ) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/federation-invites`,
        input
      );
      return itemFrom<FederationInvite>(data, "invite") ?? (data as FederationInvite);
    },
    async transitionFederationInvite(
      workspaceId: string,
      channelId: string,
      inviteId: string,
      status: "accepted" | "declined" | "revoked" | "failed"
    ) {
      const data = await http.post<unknown>(
        `${collaborationPath(workspaceId, channelId)}/federation-invites/${encodeURIComponent(inviteId)}/${status}`,
        {}
      );
      return itemFrom<FederationInvite>(data, "invite") ?? (data as FederationInvite);
    }
  };
}
