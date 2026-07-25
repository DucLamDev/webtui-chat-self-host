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
};
