export type PresenceStatus = "online" | "offline" | "busy";

export type ChannelTone = "purple" | "green" | "orange" | "red" | "violet" | "slate";

export type ChannelFilter = "all" | "unread" | "favorite";

export type DetailTab = "pinned" | "media" | "files";

export type ChatUser = {
  id: string;
  name: string;
  status: PresenceStatus;
  avatarUrl?: string;
  email?: string;
  phoneNumber?: string;
  username?: string;
};

export type ChatMetric = {
  label: string;
  value: string;
  tone: "blue" | "green" | "orange" | "purple";
};

export type MessageReplyPreview = {
  authorName: string;
  body: string;
  messageId: string;
};

export type ChatMessage = {
  id: string;
  author: ChatUser;
  sentAt: string;
  body: string;
  attachments?: MessageAttachmentItem[];
  canDelete?: boolean;
  canEdit?: boolean;
  editedAt?: string;
  isDeleted?: boolean;
  isForwarded?: boolean;
  isBot?: boolean;
  isPending?: boolean;
  isSystem?: boolean;
  isVoice?: boolean;
  reactions?: Array<{ emoji: string; count: number; reactedByMe?: boolean }>;
  metrics?: ChatMetric[];
  attachmentName?: string;
  isMine?: boolean;
  isLocal?: boolean;
  rawChannelId?: string;
  rawCreatedAt?: string;
  rawSenderId?: string | null;
  parentId?: string;
  threadRootId?: string;
  replyTo?: MessageReplyPreview;
  qrImageUrl?: string;
  qrReference?: string;
  callEvent?: MessageCallEvent;
  systemTone?: "announcement" | "system";
};

export type MessageCallEvent = {
  direction: "incoming" | "outgoing";
  durationSeconds?: number;
  initiatorUserId?: string;
  mode: "audio" | "video";
  status: "completed" | "missed";
  targetUserId?: string;
};

export type MessageAttachmentItem = {
  fileId: string;
  id: string;
  checksumSha256?: string | null;
  isAudio?: boolean;
  isImage?: boolean;
  isVideo?: boolean;
  mimeType?: string;
  name: string;
  previewUrl?: string;
  size?: string;
  status?: string;
  tone: "green" | "red" | "slate";
  url?: string;
};

export type ChatChannel = {
  avatarUrl?: string;
  canManage?: boolean;
  createdBy?: string;
  departmentId?: string;
  id: string;
  name: string;
  description: string;
  tone: ChannelTone;
  unreadCount: number;
  isFavorite: boolean;
  isMember?: boolean;
  membershipStatus?: string;
  privateSessionMode?: boolean;
  memberCount: number;
  messages: ChatMessage[];
  peerUserId?: string;
  relativeTime: string;
  slug?: string;
  type?: string;
  userStatus?: PresenceStatus;
};

export type DirectConversation = {
  id: string;
  user: ChatUser;
  lastMessage: string;
  relativeTime: string;
  unreadCount?: number;
};

export type PinnedMessage = {
  id: string;
  author: ChatUser;
  date: string;
  text: string;
};

export type MediaItem = {
  attachment: MessageAttachmentItem;
  id: string;
  label: string;
  name: string;
  url?: string;
};

export type FileItem = {
  checksumSha256?: string | null;
  id: string;
  downloadUrl?: string;
  mimeType?: string;
  name: string;
  size: string;
  status?: string;
  updatedAt: string;
  tone: "green" | "red" | "slate";
};

export type NotificationItem = {
  body: string;
  callId?: string;
  callMode?: "audio" | "video";
  callStatus?: string;
  channelId?: string;
  createdAt: string;
  data?: Record<string, unknown>;
  id: string;
  initiatorUserId?: string;
  isRead: boolean;
  messageId?: string;
  targetUserId?: string;
  title: string;
  type: string;
};
