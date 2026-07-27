import type { Id, ISODateTime, JsonObject } from "./api";
import type { FileObject } from "./file";

export type ChannelType = "public" | "private" | "direct" | string;

export type Channel = {
  id: Id;
  workspace_id: Id;
  department_id?: Id | null;
  slug?: string;
  name: string;
  description?: string | null;
  type?: ChannelType;
  kind?: ChannelType;
  member_count?: number;
  unread_count?: number;
  is_favorite?: boolean;
  created_by?: Id | null;
  membership_status?: "none" | "invited" | "active" | "muted" | "left" | "removed" | string;
  is_member?: boolean;
  can_manage?: boolean;
  private_session_mode?: boolean;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type ChannelMember = {
  user_id: Id;
  display_name?: string;
  username?: string;
  email?: string;
  avatar_url?: string | null;
  status?: string;
  last_read_at?: ISODateTime | null;
  last_read_message_id?: Id | null;
  joined_at?: ISODateTime;
};

export type DirectConversation = {
  id: Id;
  workspace_id?: Id;
  channel_id?: Id;
  user?: ChannelMember;
  participants?: ChannelMember[];
  last_message?: Message | null;
  unread_count?: number;
  updated_at?: ISODateTime;
};

export type ContactUser = {
  id: Id;
  email?: string;
  username?: string;
  display_name?: string;
  avatar_url?: string | null;
  phone_number?: string | null;
  status?: string;
};

export type ContactRequestStatus = "pending" | "accepted" | "rejected" | "cancelled";

export type ContactRequestDirection = "incoming" | "outgoing";

export type ContactRequest = {
  id: Id;
  direction?: ContactRequestDirection;
  requester_id: Id;
  receiver_id: Id;
  status: ContactRequestStatus | string;
  user: ContactUser;
  requested_at?: ISODateTime;
  responded_at?: ISODateTime | null;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type MessageAuthor = {
  id: Id;
  display_name?: string;
  username?: string;
  email?: string;
  avatar_url?: string | null;
  status?: string;
};

export type MessageReaction = {
  emoji: string;
  count?: number;
  reacted_by_me?: boolean;
  user_ids?: Id[];
};

export type MessageAttachment = {
  id?: Id;
  file_id?: Id;
  file_name?: string;
  name?: string;
  original_name?: string;
  size?: number;
  byte_size?: number;
  size_bytes?: number;
  mime_type?: string;
  download_url?: string;
  url?: string;
  file?: FileObject;
};

export type Message = {
  id: Id;
  workspace_id?: Id;
  channel_id?: Id;
  parent_id?: Id | null;
  thread_root_id?: Id | null;
  author_id?: Id;
  sender_id?: Id | null;
  author?: MessageAuthor | null;
  user?: MessageAuthor | null;
  kind?: string;
  body: string;
  metadata?: JsonObject | null;
  mentions?: Id[];
  mentioned_user_ids?: Id[];
  reactions?: MessageReaction[];
  attachments?: MessageAttachment[];
  edited_at?: ISODateTime | null;
  deleted_at?: ISODateTime | null;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
  sent_at?: ISODateTime;
};

export type SendMessageInput = {
  parent_id?: Id;
  client_message_id?: string;
  kind?: string;
  body: string;
  metadata?: JsonObject;
  mentioned_user_ids?: Id[];
  silent?: boolean;
};

export type ScheduledMessage = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  sender_id: Id;
  parent_id?: Id | null;
  kind: string;
  body: string;
  metadata: JsonObject;
  scheduled_for: ISODateTime;
  status: "pending" | "processing" | "sent" | "failed" | "cancelled";
  sent_message_id?: Id | null;
  attempt_count: number;
  last_error?: string | null;
  client_message_id?: string | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type ScheduleMessageInput = SendMessageInput & {
  scheduled_for: ISODateTime;
};

export type MessageReminder = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  message_id: Id;
  user_id: Id;
  remind_at: ISODateTime;
  note?: string | null;
  status: "pending" | "processing" | "fired" | "failed" | "cancelled";
  notification_id?: Id | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type ThreadDetails = {
  workspace_id: Id;
  channel_id: Id;
  root_message_id: Id;
  title: string;
  description: string;
  status: "open" | "resolved";
  subscribed: boolean;
  reply_count: number;
  unread_count: number;
  last_reply_at?: ISODateTime | null;
  last_read_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type UpdateMessageInput = {
  body: string;
};

export type AddReactionInput = {
  emoji: string;
};

export type ForwardMessageInput = {
  target_channel_id: Id;
};

export type CreateChannelInput = {
  department_id?: Id;
  slug: string;
  name: string;
  description?: string;
  type: ChannelType;
};

export type UpdateChannelInput = Partial<CreateChannelInput>;

export type AddChannelMemberInput = {
  user_id: Id;
};

export type UpdateReadStateInput = {
  last_read_message_id?: Id;
};

export type CreateDirectConversationInput = {
  participant_ids: Id[];
  source_channel_id?: Id;
};

export type CollaborationRoomMode = "internal" | "public" | "webinar";
export type CollaborationMeetingProvider = "jitsi" | "webrtc";
export type CollaborationParticipantRole = "moderator" | "presenter" | "member" | "listener";

export type CollaborationSettings = {
  channel_id: Id;
  workspace_id: Id;
  channel_name: string;
  channel_type: ChannelType;
  room_mode: CollaborationRoomMode;
  meeting_provider: CollaborationMeetingProvider;
  meeting_base_url?: string;
  meeting_room_key?: string;
  public_access_enabled: boolean;
  public_token_prefix?: string | null;
  has_password: boolean;
  lobby_enabled: boolean;
  chat_locked: boolean;
  guest_microphone_enabled: boolean;
  guest_camera_enabled: boolean;
  default_participant_role: CollaborationParticipantRole;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type UpdateCollaborationSettingsInput = {
  room_mode: CollaborationRoomMode;
  meeting_provider: CollaborationMeetingProvider;
  lobby_enabled: boolean;
  chat_locked: boolean;
  guest_microphone_enabled: boolean;
  guest_camera_enabled: boolean;
  default_participant_role: CollaborationParticipantRole;
};

export type PublicConversationLink = CollaborationSettings & {
  token: string;
};

export type CreatePublicConversationLinkInput = {
  room_mode: "public" | "webinar";
  password?: string;
  lobby_enabled: boolean;
  chat_locked: boolean;
  guest_microphone_enabled: boolean;
  guest_camera_enabled: boolean;
};

export type PublicConversationRoom = {
  channel_id: Id;
  channel_name: string;
  room_mode: CollaborationRoomMode;
  meeting_provider: CollaborationMeetingProvider;
  meeting_base_url?: string;
  meeting_room_key?: string;
  has_password: boolean;
  lobby_enabled: boolean;
  chat_locked: boolean;
  guest_microphone_enabled: boolean;
  guest_camera_enabled: boolean;
};

export type GuestJoinRequest = {
  id: Id;
  channel_id: Id;
  display_name: string;
  status: "waiting" | "approved" | "rejected" | "expired";
  guest_access_token?: string;
  room?: PublicConversationRoom;
  expires_at: ISODateTime;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type CollaborationRole = {
  channel_id: Id;
  user_id: Id;
  display_name: string;
  username: string;
  avatar_url?: string | null;
  role: CollaborationParticipantRole;
  updated_at: ISODateTime;
};

export type CollaborationDocumentKind = "notes" | "whiteboard";

export type CollaborationDocument = {
  channel_id: Id;
  kind: CollaborationDocumentKind;
  content: JsonObject;
  version: number;
  updated_by?: Id | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type ChannelTaskStatus = "open" | "in_progress" | "done" | "cancelled";

export type ChannelTask = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  source_message_id?: Id | null;
  title: string;
  description?: string | null;
  status: ChannelTaskStatus;
  assignee_user_id?: Id | null;
  due_at?: ISODateTime | null;
  created_by?: Id | null;
  completed_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type CreateChannelTaskInput = {
  source_message_id?: Id;
  title: string;
  description?: string;
  assignee_user_id?: Id;
  due_at?: ISODateTime;
};

export type BreakoutRoom = {
  id: Id;
  channel_id: Id;
  name: string;
  room_key: string;
  assigned_user_ids: Id[];
  status: "prepared" | "active" | "closed";
  assignment_mode: "automatic" | "manual" | "self_select";
  allow_self_select: boolean;
  started_at?: ISODateTime | null;
  sequence: number;
  created_by?: Id | null;
  closed_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type BreakoutRoomSpec = {
  name: string;
  assigned_user_ids: Id[];
  sequence?: number;
};

export type SetupBreakoutRoomsInput = {
  assignment_mode: "automatic" | "manual" | "self_select";
  room_count?: number;
  allow_self_select?: boolean;
  rooms?: BreakoutRoomSpec[];
};

export type ChannelMeeting = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  title: string;
  description: string;
  starts_at: ISODateTime;
  ends_at?: ISODateTime | null;
  lobby_opens_at?: ISODateTime | null;
  status: "scheduled" | "active" | "ended" | "cancelled";
  room_policy: "keep" | "archive" | "delete";
  cleanup_after?: ISODateTime | null;
  created_by?: Id | null;
  started_at?: ISODateTime | null;
  ended_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type CreateChannelMeetingInput = {
  title: string;
  description?: string;
  starts_at: ISODateTime;
  ends_at?: ISODateTime;
  lobby_opens_at?: ISODateTime;
  room_policy?: "keep" | "archive" | "delete";
  cleanup_after?: ISODateTime;
};

export type ChannelVoiceRoom = {
  channel_id: Id;
  workspace_id: Id;
  status: "active" | "inactive";
  started_by?: Id | null;
  started_at?: ISODateTime | null;
  ended_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type SharedItem = {
  id: Id;
  kind: "file" | "pin" | "poll" | "task" | "recording" | string;
  title: string;
  subtitle?: string;
  url?: string;
  metadata: JsonObject;
  created_by?: Id | null;
  created_at: ISODateTime;
};

export type TalkHome = {
  upcoming_meetings: ChannelMeeting[];
  active_voice_rooms: ChannelVoiceRoom[];
  open_tasks: ChannelTask[];
  unread_mentions: number;
  pending_reminders: number;
  missed_calls: number;
};

export type RecordingPolicy = {
  channel_id: Id;
  workspace_id: Id;
  enabled: boolean;
  consent_required: boolean;
  retention_days: number;
  transcription_enabled: boolean;
  summary_enabled: boolean;
  provider: "jibri" | "external" | "disabled" | string;
  updated_by?: Id | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type ChannelRecording = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  meeting_id?: Id | null;
  status: "pending" | "recording" | "processing" | "ready" | "failed" | "deleted";
  provider: string;
  provider_recording_id?: string | null;
  participant_user_ids: Id[];
  mime_type?: string | null;
  byte_size?: number | null;
  checksum_sha256?: string | null;
  started_by?: Id | null;
  started_at?: ISODateTime | null;
  ended_at?: ISODateTime | null;
  expires_at?: ISODateTime | null;
  transcript_status: "disabled" | "pending" | "processing" | "ready" | "failed";
  transcript: JsonObject;
  summary_status: "disabled" | "pending" | "processing" | "ready" | "failed";
  summary: JsonObject;
  error?: string | null;
  consent_count: number;
  declined_count: number;
  participant_count: number;
  ready_to_start: boolean;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type TalkIntegration = {
  workspace_id: Id;
  ai_enabled: boolean;
  ai_provider: string;
  transcription_provider: string;
  federation_enabled: boolean;
  e2ee_calls_enabled: boolean;
  sip_enabled: boolean;
  bridge_enabled: boolean;
  config: JsonObject;
  updated_by?: Id | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type TalkSummary = {
  summary: string;
  decisions: string[];
  action_items: string[];
  model: string;
  message_count: number;
  generated_at: ISODateTime;
};

export type FederationInvite = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  remote_server: string;
  remote_user: string;
  direction: "outbound" | "inbound";
  status: "pending" | "accepted" | "declined" | "revoked" | "failed";
  protocol: "open_cloud_mesh" | "talk_federation" | string;
  payload: JsonObject;
  created_by?: Id | null;
  responded_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};
