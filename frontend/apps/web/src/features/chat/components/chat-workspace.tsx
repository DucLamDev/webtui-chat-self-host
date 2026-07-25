"use client";

import { Fragment, type ChangeEvent, type ClipboardEvent, type CSSProperties, type DragEvent, type FormEvent, type ReactNode, type RefObject, type UIEvent, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { usePathname, useSearchParams } from "next/navigation";
import { queryKeys } from "@webtui/api-client";
import { getPlatformServices, type MediaRecorderHandle } from "@webtui/chat-core";
import {
  Avatar,
  Badge,
  Button,
  EmptyState,
  ErrorState,
  Input,
  NavigationRail,
  SegmentedControl,
  Skeleton,
  Toast,
  Tooltip,
  useTheme
} from "@webtui/ui";
import {
  Angry,
  Archive,
  Bell,
  Bot,
  CheckCircle2,
  ChevronLeft,
  Clock3,
  Cloud,
  Edit3,
  FileText,
  Frown,
  Hash,
  Heart,
  Image as ImageIcon,
  Info,
  Laugh,
  LogOut,
  MessageCircle,
  Mic,
  Monitor,
  Minimize2,
  MoreVertical,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  Paperclip,
  Pause,
  Phone,
  PhoneOff,
  Pin,
  Play,
  Plus,
  Reply,
  Search,
  Send,
  Share2,
  Smartphone,
  ShieldCheck,
  Smile,
  SmilePlus,
  Sparkles,
  StopCircle,
  Star,
  Sun,
  Ticket,
  ThumbsUp,
  Trash2,
  Users,
  Video,
  Workflow,
  X,
  Zap
} from "@webtui/icons";
import { useAuth } from "@/features/auth/auth-provider";
import { useAuthStore } from "@/features/auth/auth-store";
import { useDesktopVersionStatus, type DesktopVersionStatus } from "@/features/platform/hooks/use-api-status";
import { api, runtimeEnvironment } from "@/lib/api";
import {
  mapAuthUser,
  type CreateChannelPayload,
  type CreateDepartmentPayload,
  useChatWorkspaceData
} from "../hooks/use-chat-workspace-data";
import { useWebRtcCall, type WebRtcCallOutcome, type WebRtcCallState } from "../hooks/use-webrtc-call";
import type {
  ChannelFilter,
  ChatChannel,
  ChatMessage,
  ChatUser,
  DetailTab,
  FileItem,
  MediaItem,
  MessageAttachmentItem,
  MessageReplyPreview,
  NotificationItem,
  PinnedMessage
} from "../model/types";
import { useUploadStore, type UploadQueueItem } from "../stores/upload-store";
import { getCachedMediaUrl, resolveCachedMediaUrl } from "../model/media-cache";
import { readDraft, writeDraft } from "../model/offline-cache";
import { buildChatTargets } from "../model/chat-targets";
import { buildDepartmentRows, departmentDescendantIds } from "../model/department-tree";
import type {
  AuthSession,
  AuthUser,
  Bot as BotRecord,
  ChannelMember,
  ContactRequest,
  Department,
  DepartmentMember,
  NotificationPreference,
  NotificationPreferenceInput,
  OrderServicesExpiringData,
  OrderServicesExpiringInput,
  OrderPaymentQRData,
  OrderWalletBalanceData,
  OrderWalletDepositQRData,
  Ticket as SupportTicket,
  TicketPriority,
  TicketStatus,
  WorkspaceMember
} from "@webtui/types";
import { AutomationPage } from "./automation-page";
import { parseChatRoute } from "@/lib/chat-route";

const railItems = [
  { id: "messages", label: "Tin nhắn", icon: ConversationSolidIcon },
  { id: "channels", label: "Kênh & Bot", icon: GroupSolidIcon },
  { id: "contacts", label: "Bạn bè", icon: AddContactSolidIcon },
  { id: "departments", label: "Phòng ban", icon: Archive },
  { id: "tickets", label: "Ticket", icon: Ticket },
  { id: "files", label: "File", icon: FileText },
  { id: "bots", label: "Bot", icon: Bot },
  { id: "automation", label: "Automation", icon: Workflow },
  { id: "settings", label: "Cài đặt", icon: SettingsSolidIcon }
] as const;

type RailItemId = (typeof railItems)[number]["id"];
type MessageSidebarTab = "conversations" | "channels";
type ContactsTab = "employees" | "friends" | "discover";
type ChatWorkspaceData = ReturnType<typeof useChatWorkspaceData>;

type ChannelHashStyle = CSSProperties & {
  "--channel-hash-bg": string;
  "--channel-hash-bg-soft": string;
  "--channel-hash-border": string;
  "--channel-hash-dark-bg": string;
  "--channel-hash-dark-border": string;
  "--channel-hash-dark-text": string;
  "--channel-hash-shadow": string;
  "--channel-hash-text": string;
};

const channelHashPalettes = [
  { bg: "#e0f2fe", bgSoft: "#f0f9ff", border: "#bae6fd", darkBg: "#083344", darkBorder: "#155e75", darkText: "#67e8f9", shadow: "rgb(14 165 233 / 22%)", text: "#0284c7" },
  { bg: "#dcfce7", bgSoft: "#f0fdf4", border: "#bbf7d0", darkBg: "#052e16", darkBorder: "#166534", darkText: "#86efac", shadow: "rgb(34 197 94 / 22%)", text: "#16a34a" },
  { bg: "#fef3c7", bgSoft: "#fffbeb", border: "#fde68a", darkBg: "#451a03", darkBorder: "#92400e", darkText: "#fcd34d", shadow: "rgb(245 158 11 / 24%)", text: "#d97706" },
  { bg: "#fee2e2", bgSoft: "#fef2f2", border: "#fecaca", darkBg: "#450a0a", darkBorder: "#991b1b", darkText: "#fca5a5", shadow: "rgb(239 68 68 / 24%)", text: "#dc2626" },
  { bg: "#f3e8ff", bgSoft: "#faf5ff", border: "#e9d5ff", darkBg: "#2e1065", darkBorder: "#6b21a8", darkText: "#d8b4fe", shadow: "rgb(168 85 247 / 24%)", text: "#9333ea" },
  { bg: "#fce7f3", bgSoft: "#fdf2f8", border: "#fbcfe8", darkBg: "#500724", darkBorder: "#9d174d", darkText: "#f9a8d4", shadow: "rgb(236 72 153 / 24%)", text: "#db2777" },
  { bg: "#e0e7ff", bgSoft: "#eef2ff", border: "#c7d2fe", darkBg: "#1e1b4b", darkBorder: "#4338ca", darkText: "#a5b4fc", shadow: "rgb(99 102 241 / 24%)", text: "#4f46e5" },
  { bg: "#ccfbf1", bgSoft: "#f0fdfa", border: "#99f6e4", darkBg: "#042f2e", darkBorder: "#0f766e", darkText: "#5eead4", shadow: "rgb(20 184 166 / 22%)", text: "#0d9488" }
] as const;

const channelHashPaletteBySlug: Record<string, (typeof channelHashPalettes)[number]> = {
  "ban-giam-doc": channelHashPalettes[4],
  "ban-giao-ca": channelHashPalettes[6],
  "gia-han": channelHashPalettes[2],
  "ke-toan": channelHashPalettes[1],
  "ky-thuat": channelHashPalettes[7],
  sale: channelHashPalettes[5],
  "server-alert": channelHashPalettes[3],
  "thong-bao": channelHashPalettes[0],
  ticket: channelHashPalettes[1]
};

function channelHashStyle(channel: ChatChannel): ChannelHashStyle {
  const key = normalizeChannelColorKey(channel.slug || channel.name || channel.id);
  const palette = channelHashPaletteBySlug[key] ?? channelHashPalettes[stableColorIndex(key || channel.id)];

  return {
    "--channel-hash-bg": palette.bg,
    "--channel-hash-bg-soft": palette.bgSoft,
    "--channel-hash-border": palette.border,
    "--channel-hash-dark-bg": palette.darkBg,
    "--channel-hash-dark-border": palette.darkBorder,
    "--channel-hash-dark-text": palette.darkText,
    "--channel-hash-shadow": palette.shadow,
    "--channel-hash-text": palette.text
  };
}

function normalizeChannelColorKey(value: string) {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function stableColorIndex(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index++) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash % channelHashPalettes.length;
}

function botComposerPlaceholder(channel?: ChatChannel | null) {
  switch (channel?.slug) {
    case "gia-han":
      return "Gia Hạn Bot: Email: khach@example.com · Số ngày: 7 · Loại dịch vụ: Tất cả";
    case "ke-toan":
      return "Thanh Toán Bot: Email: khach@example.com · Số tiền: 200000";
    case "ticket":
      return "Ticket Bot: mô tả lỗi, hoặc nhập “Tra ví email@example.com”";
    case "server-alert":
      return "Server Alert Bot: Server: vps-01 · Lỗi: mất ping/port timeout...";
    default:
      return "Nhập tin nhắn...";
  }
}
type ContactResult = {
  avatarUrl?: string | null;
  contactDirection?: "incoming" | "outgoing";
  contactRequestId?: string;
  contactStatus: "accepted" | "none" | "pending" | "rejected";
  email?: string;
  hasConversation: boolean;
  isWorkspaceMember: boolean;
  name: string;
  phoneNumber?: string | null;
  status?: string;
  userId: string;
  username?: string;
};

const channelFilters: Array<{ label: string; value: ChannelFilter }> = [
  { label: "Tất cả", value: "all" },
  { label: "Chưa đọc", value: "unread" },
  { label: "Yêu thích", value: "favorite" }
];

const detailTabs: Array<{ label: string; value: DetailTab }> = [
  { label: "Đã ghim", value: "pinned" },
  { label: "Ảnh", value: "media" },
  { label: "File", value: "files" }
];

const quickReactions = [
  { className: "reaction-choice--like", emoji: "👍", icon: ThumbsUp, label: "Thích" },
  { className: "reaction-choice--love", emoji: "❤️", icon: Heart, label: "Yêu thích" },
  { className: "reaction-choice--haha", emoji: "😂", icon: Laugh, label: "Haha" },
  { className: "reaction-choice--wow", emoji: "😮", icon: SmilePlus, label: "Ngạc nhiên" },
  { className: "reaction-choice--sad", emoji: "😢", icon: Frown, label: "Buồn" },
  { className: "reaction-choice--angry", emoji: "😡", icon: Angry, label: "Giận" }
] as const;

const maxUploadSizeBytes = 100 * 1024 * 1024;
const uploadAcceptList = [
  "image/*",
  "text/*",
  "audio/webm",
  "audio/ogg",
  "audio/mp4",
  "audio/mpeg",
  "audio/wav",
  "audio/x-m4a",
  "application/ogg",
  "application/pdf",
  "application/json",
  "application/zip",
  "application/octet-stream",
  "application/msword",
  "application/vnd.ms-excel",
  "application/vnd.ms-powerpoint",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "application/vnd.openxmlformats-officedocument.presentationml.presentation"
];
const imageUploadAcceptList = ["image/*"];

type NotificationMode = "all" | "mentions" | "muted";

type NotificationPreferences = {
  mode: NotificationMode;
  preview: boolean;
  quietHours: boolean;
  quietStart: string;
  quietEnd: string;
};

type ChatToastNotice = {
  message: string;
  tone: "danger" | "info" | "success";
};

const defaultNotificationPreferences: NotificationPreferences = {
  mode: "all",
  preview: true,
  quietEnd: "07:00",
  quietHours: false,
  quietStart: "22:00"
};

const notificationModeOptions: Array<{ label: string; value: NotificationMode }> = [
  { label: "Tất cả", value: "all" },
  { label: "Nhắc tên", value: "mentions" },
  { label: "Tắt", value: "muted" }
];

function ComposerLikeIcon() {
  return (
    <svg aria-hidden="true" fill="none" height="24" viewBox="0 0 24 24" width="24" xmlns="http://www.w3.org/2000/svg">
      <path
        clipRule="evenodd"
        d="M15.9 4.5C15.9 3 14.418 2 13.26 2c-.806 0-.869.612-.993 1.82-.055.53-.121 1.174-.267 1.93-.386 2.002-1.72 4.56-2.996 5.325V17C9 19.25 9.75 20 13 20h3.773c2.176 0 2.703-1.433 2.899-1.964l.013-.036c.114-.306.358-.547.638-.82.31-.306.664-.653.927-1.18.311-.623.27-1.177.233-1.67-.023-.299-.044-.575.017-.83.064-.27.146-.475.225-.671.143-.356.275-.686.275-1.329 0-1.5-.748-2.498-2.315-2.498H15.5S15.9 6 15.9 4.5zM5.5 10A1.5 1.5 0 0 0 4 11.5v7a1.5 1.5 0 0 0 3 0v-7A1.5 1.5 0 0 0 5.5 10z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

function HamburgerMenuIcon({ size = 20 }: { size?: number }) {
  return (
    <svg aria-hidden="true" fill="none" height={size} viewBox="0 0 24 24" width={size} xmlns="http://www.w3.org/2000/svg">
      <path d="M4 7h16M4 12h16M4 17h16" stroke="currentColor" strokeLinecap="round" strokeWidth="2" />
    </svg>
  );
}

type SolidNavIconProps = {
  "aria-hidden"?: true | "true";
  className?: string;
  size?: number;
};

function ConversationSolidIcon({ size = 20, ...props }: SolidNavIconProps) {
  return (
    <svg fill="none" height={size} viewBox="0 0 24 24" width={size} xmlns="http://www.w3.org/2000/svg" {...props}>
      <path
        clipRule="evenodd"
        d="M12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2C6.47715 2 2 6.47715 2 12C2 13.5997 2.37562 15.1116 3.04346 16.4525C3.22094 16.8088 3.28001 17.2161 3.17712 17.6006L2.58151 19.8267C2.32295 20.793 3.20701 21.677 4.17335 21.4185L6.39939 20.8229C6.78393 20.72 7.19121 20.7791 7.54753 20.9565C8.88837 21.6244 10.4003 22 12 22ZM8 13.25C7.58579 13.25 7.25 13.5858 7.25 14C7.25 14.4142 7.58579 14.75 8 14.75H13.5C13.9142 14.75 14.25 14.4142 14.25 14C14.25 13.5858 13.9142 13.25 13.5 13.25H8ZM7.25 10.5C7.25 10.0858 7.58579 9.75 8 9.75H16C16.4142 9.75 16.75 10.0858 16.75 10.5C16.75 10.9142 16.4142 11.25 16 11.25H8C7.58579 11.25 7.25 10.9142 7.25 10.5Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

function AddContactSolidIcon({ size = 20, ...props }: SolidNavIconProps) {
  return (
    <svg fill="currentColor" height={size} viewBox="924 796 200 200" width={size} xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M1050.379,857.065c0,29.431-20.199,53.292-45.119,53.292c-24.917,0-45.116-23.862-45.116-53.292 c0-29.433,20.198-53.285,45.116-53.285C1030.18,803.78,1050.379,827.632,1050.379,857.065z" />
      <path d="M1065.693,988.22c1.768,0,3.201-1.433,3.201-3.2v-18.697c0-1.767-1.434-3.2-3.201-3.2h-14.774 c-8.276,0-15.012-6.733-15.012-15.012c0-8.277,6.735-15.011,15.012-15.011h17.905c-5.382-6.167-11.636-11.557-18.591-15.94 c-3.404-2.146-7.82-1.797-10.839,0.862c-9.255,8.15-20.709,13.004-33.143,13.004c-12.679,0-24.337-5.043-33.688-13.485 c-2.985-2.694-7.378-3.1-10.81-1.005c-17.754,10.842-31.191,28.053-37.051,48.491c-1.574,5.483-0.468,11.396,2.963,15.956 c3.442,4.557,8.816,7.237,14.521,7.237H1065.693L1065.693,988.22z" />
      <path d="M1116.891,941.001h-25.875v-25.876c0-3.927-3.184-7.11-7.111-7.11c-3.929,0-7.111,3.184-7.111,7.11v25.876h-25.875 c-3.927,0-7.11,3.184-7.11,7.109c0,3.927,3.184,7.11,7.11,7.11h25.875v25.876c0,3.927,3.183,7.11,7.111,7.11 c3.928,0,7.111-3.184,7.111-7.11v-25.876h25.875c3.926,0,7.109-3.184,7.109-7.11C1124,944.185,1120.816,941.001,1116.891,941.001z" />
    </svg>
  );
}

function GroupSolidIcon({ size = 20, ...props }: SolidNavIconProps) {
  return (
    <svg fill="currentColor" height={size} viewBox="924 565.952 200 200" width={size} xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M984.585,626.893c0,14-9.609,25.348-21.461,25.348s-21.459-11.348-21.459-25.348c0-13.999,9.607-25.345,21.459-25.345 S984.585,612.895,984.585,626.893z" />
      <path d="M987.586,671.591c1.549-0.945,3.265-1.56,5.041-1.854c-3.606-5.088-6.161-10.546-7.637-17.078 c-0.404-2.387-3.672-2.667-6.102-0.687c-4.545,3.706-9.849,6.186-15.764,6.186c-6.03,0-11.577-2.399-16.025-6.414 c-1.419-1.283-3.51-1.476-5.142-0.479c-8.444,5.157-14.835,13.344-17.623,23.064c-0.748,2.607-0.223,5.421,1.411,7.59 c1.637,2.166,4.192,3.443,6.906,3.443h38.669C975.947,680.023,981.41,675.362,987.586,671.591z" />
      <path d="M1063.414,626.893c0,14,9.61,25.348,21.462,25.348s21.46-11.348,21.46-25.348c0-13.999-9.608-25.345-21.46-25.345 S1063.414,612.895,1063.414,626.893z" />
      <path d="M1060.413,671.591c-1.549-0.945-3.264-1.56-5.04-1.854c3.605-5.088,6.16-10.546,7.637-17.078 c0.404-2.387,3.674-2.667,6.103-0.687c4.545,3.706,9.849,6.186,15.764,6.186c6.03,0,11.576-2.399,16.024-6.414 c1.42-1.283,3.51-1.476,5.143-0.479c8.443,5.157,14.834,13.344,17.623,23.064c0.748,2.608,0.222,5.421-1.412,7.59 c-1.635,2.166-4.192,3.443-6.906,3.443h-38.668C1072.052,680.023,1066.59,675.362,1060.413,671.591z" />
      <path d="M1082.474,713.402c-4.198-14.654-13.72-27.044-26.327-34.991c-2.487-1.567-5.715-1.313-7.921,0.631 c-6.765,5.958-15.136,9.506-24.226,9.506c-9.268,0-17.791-3.686-24.626-9.856c-2.181-1.97-5.393-2.267-7.901-0.734 c-12.977,7.925-22.8,20.505-27.082,35.445c-1.151,4.008-0.344,8.329,2.166,11.663c2.516,3.329,6.443,5.29,10.615,5.29h92.521 c4.173,0,8.103-1.954,10.618-5.29C1082.822,721.731,1083.625,717.414,1082.474,713.402z" />
      <path d="M1056.98,640.499c0,21.512-14.767,38.955-32.98,38.955s-32.979-17.442-32.979-38.955 c0-21.515,14.765-38.951,32.979-38.951S1056.98,618.984,1056.98,640.499z" />
    </svg>
  );
}

function SettingsSolidIcon({ size = 20, ...props }: SolidNavIconProps) {
  return (
    <svg fill="currentColor" height={size} viewBox="0 0 48 48" width={size} xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M40.2,29.2l5.5-1.5a23,23,0,0,0,0-7.4l-5.5-1.5a1.8,1.8,0,0,1-1.1-2.6l2.8-5a20.6,20.6,0,0,0-5.1-5.1l-5,2.8-.8.2a1.8,1.8,0,0,1-1.8-1.3L27.7,2.3a23,23,0,0,0-7.4,0L18.8,7.8A1.8,1.8,0,0,1,17,9.1l-.8-.2-5-2.8a20.6,20.6,0,0,0-5.1,5.1l2.8,5a1.8,1.8,0,0,1-1.1,2.6L2.3,20.3a23,23,0,0,0,0,7.4l5.5,1.5a1.8,1.8,0,0,1,1.1,2.6l-2.8,5a20.6,20.6,0,0,0,5.1,5.1l5-2.8.8-.2a1.8,1.8,0,0,1,1.8,1.3l1.5,5.5a23,23,0,0,0,7.4,0l1.5-5.5A1.8,1.8,0,0,1,31,38.9l.8.2,5,2.8a20.6,20.6,0,0,0,5.1-5.1l-2.8-5A1.8,1.8,0,0,1,40.2,29.2ZM24,33a9,9,0,1,1,9-9A9,9,0,0,1,24,33Z" />
    </svg>
  );
}

function MobileFeatureMenu({
  activeId,
  items,
  onSelect
}: {
  activeId: RailItemId;
  items: ReadonlyArray<(typeof railItems)[number]>;
  onSelect: (itemId: RailItemId) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const closeOnOutsidePress = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };

    document.addEventListener("pointerdown", closeOnOutsidePress);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePress);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  return (
    <div className="mobile-feature-menu" ref={menuRef}>
      <Button
        aria-expanded={isOpen}
        aria-label={isOpen ? "Đóng menu chức năng" : "Mở menu chức năng"}
        className="mobile-feature-menu__trigger"
        onClick={() => setIsOpen((current) => !current)}
        size="sm"
        variant="icon"
      >
        {isOpen ? <X size={20} /> : <HamburgerMenuIcon size={20} />}
      </Button>
      {isOpen ? (
        <section aria-label="Menu chức năng" className="mobile-feature-menu__popover">
          <strong>Chức năng</strong>
          <nav>
            {items.map((item) => {
              const ItemIcon = item.icon;
              const isActive = item.id === activeId;

              return (
                <button
                  aria-current={isActive ? "page" : undefined}
                  className={isActive ? "mobile-feature-menu__item mobile-feature-menu__item--active" : "mobile-feature-menu__item"}
                  key={item.id}
                  onClick={() => {
                    setIsOpen(false);
                    onSelect(item.id);
                  }}
                  type="button"
                >
                  <ItemIcon size={21} />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </nav>
        </section>
      ) : null}
    </div>
  );
}

function MobileNotificationButton({
  count,
  isOpen,
  onToggle
}: {
  count: number;
  isOpen: boolean;
  onToggle: () => void;
}) {
  return (
    <span className="mobile-notification-action">
      <Button
        aria-expanded={isOpen}
        aria-label="Thông báo"
        className={isOpen ? "notification-button notification-button--active" : "notification-button"}
        onClick={onToggle}
        size="sm"
        variant="icon"
      >
        <Bell size={18} />
        {count ? <span>{count}</span> : null}
      </Button>
    </span>
  );
}

export function ChatWorkspace() {
  const { logout, user } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const routedRailItem = railItemFromRoute(pathname, searchParams);
  const routedMessageSidebarTab = messageSidebarTabFromRoute(pathname, searchParams);
  const [activeRailItem, setActiveRailItem] = useState<RailItemId>(routedRailItem);
  const [messageSidebarTab, setMessageSidebarTab] = useState<MessageSidebarTab>(routedMessageSidebarTab);
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [detailTab, setDetailTab] = useState<DetailTab>("pinned");
  const [searchQuery, setSearchQuery] = useState("");
  const [friendSearchQuery, setFriendSearchQuery] = useState("");
  const [draft, setDraft] = useState("");
  const [toast, setToastNotice] = useState<ChatToastNotice | null>(null);
  const [messageNotice, setMessageNotice] = useState<NotificationItem | null>(null);
  const [isCreateChannelOpen, setIsCreateChannelOpen] = useState(false);
  const [isTicketCreateOpen, setIsTicketCreateOpen] = useState(false);
  const [isNotificationsOpen, setIsNotificationsOpen] = useState(false);
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);
  const [editingBody, setEditingBody] = useState("");
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [messageSearchQuery, setMessageSearchQuery] = useState("");
  const [messageSearchChannelId, setMessageSearchChannelId] = useState("");
  const [messageSearchSenderId, setMessageSearchSenderId] = useState("");
  const [messageSearchKind, setMessageSearchKind] = useState("");
  const [messageSearchDateFrom, setMessageSearchDateFrom] = useState("");
  const [messageSearchDateTo, setMessageSearchDateTo] = useState("");
  const [isMessageSearchOpen, setIsMessageSearchOpen] = useState(false);
  const [threadMessageId, setThreadMessageId] = useState<string | null>(null);
  const [focusedMessageId, setFocusedMessageId] = useState<string | null>(null);
  const handleFocusedMessageSettled = useCallback(() => setFocusedMessageId(null), []);
  const [replyingTo, setReplyingTo] = useState<MessageReplyPreview | null>(null);
  const [forwardingMessageId, setForwardingMessageId] = useState<string | null>(null);
  const [isEmojiPickerOpen, setIsEmojiPickerOpen] = useState(false);
  const [isComposerMoreOpen, setIsComposerMoreOpen] = useState(false);
  const [isDetailPanelOpen, setIsDetailPanelOpen] = useState(false);
  const [isChannelPanelCollapsed, setIsChannelPanelCollapsed] = useState(false);
  const [favoriteChatIds, setFavoriteChatIds] = useState<Set<string>>(() => new Set());
  const [unfavoriteChatIds, setUnfavoriteChatIds] = useState<Set<string>>(() => new Set());
  const [manuallyUnreadChatIds, setManuallyUnreadChatIds] = useState<Set<string>>(() => new Set());
  const [locallyReadChatIds, setLocallyReadChatIds] = useState<Set<string>>(() => new Set());
  const [notificationPreferences, setNotificationPreferences] = useState<NotificationPreferences>(defaultNotificationPreferences);
  const [isAutoStartEnabled, setIsAutoStartEnabled] = useState(false);
  const [isAutoStartLoading, setIsAutoStartLoading] = useState(false);
  const [isDesktopUpdateInstalling, setIsDesktopUpdateInstalling] = useState(false);
  const [isRecording, setIsRecording] = useState(false);
  const [isRecordingPaused, setIsRecordingPaused] = useState(false);
  const [isComposerDragActive, setIsComposerDragActive] = useState(false);
  const [previewedImage, setPreviewedImage] = useState<{ attachment: MessageAttachmentItem; source: string } | null>(null);
  const [recordingSeconds, setRecordingSeconds] = useState(0);
  const mediaRecorderRef = useRef<MediaRecorderHandle | null>(null);
  const recordingCancelledRef = useRef(false);
  const recordingChunksRef = useRef<Blob[]>([]);
  const recordingPausedAtRef = useRef<number | null>(null);
  const recordingPausedMsRef = useRef(0);
  const recordingStartedAtRef = useRef<number | null>(null);
  const accountMenuRef = useRef<HTMLDivElement>(null);
  const seenNotificationIdsRef = useRef<Set<string> | null>(null);
  const desktopUpdateNoticeShownRef = useRef<string | null>(null);
  const typingStopTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const typingPublishedRef = useRef(false);
  const composerInputRef = useRef<HTMLInputElement | null>(null);
  const currentUser = useMemo(() => mapAuthUser(user), [user]);
  const desktopVersionStatus = useDesktopVersionStatus();

  function setToast(message: string | null, tone?: ChatToastNotice["tone"]) {
    setToastNotice(message ? { message, tone: tone ?? inferToastTone(message) } : null);
  }

  useEffect(() => {
    if (!toast) {
      return undefined;
    }
    const duration = toast.tone === "danger" ? 5_000 : toast.tone === "info" ? 3_500 : 2_800;
    const timer = window.setTimeout(() => setToastNotice(null), duration);
    return () => window.clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    if (!messageNotice) {
      return undefined;
    }
    const timer = window.setTimeout(() => setMessageNotice(null), 6_000);
    return () => window.clearTimeout(timer);
  }, [messageNotice]);

  useEffect(() => {
    if (desktopVersionStatus.status !== "update_available" && desktopVersionStatus.status !== "unsupported") {
      return;
    }

    const recommendedVersion = desktopVersionStatus.version?.clients?.desktop?.recommended_version;
    const noticeKey = `${desktopVersionStatus.status}:${recommendedVersion ?? desktopVersionStatus.detail ?? ""}`;
    if (desktopUpdateNoticeShownRef.current === noticeKey) {
      return;
    }

    desktopUpdateNoticeShownRef.current = noticeKey;
    setToast(
      desktopVersionStatus.status === "unsupported"
        ? "Bản desktop hiện tại cần cập nhật để tiếp tục tương thích."
        : "Đã có bản cập nhật desktop mới trong mục Cài đặt.",
      "info"
    );
  }, [desktopVersionStatus.detail, desktopVersionStatus.status, desktopVersionStatus.version]);
  const activeMessageSearchQuery = isMessageSearchOpen ? messageSearchQuery : "";
  const data = useChatWorkspaceData(currentUser, {
    friendSearchQuery,
    messageSearchFilters: {
      channelId: messageSearchChannelId,
      dateFrom: messageSearchDateFrom,
      dateTo: messageSearchDateTo,
      kind: messageSearchKind,
      senderId: messageSearchSenderId
    },
    messageSearchQuery: activeMessageSearchQuery,
    threadMessageId: threadMessageId ?? undefined
  });

  const visibleRailItems = railItems.filter((item) => canAccessRailItem(item.id, data.can));

  useEffect(() => setActiveRailItem(routedRailItem), [routedRailItem]);
  useEffect(() => setMessageSidebarTab(routedMessageSidebarTab), [routedMessageSidebarTab]);
  useEffect(() => {
    const desktopDetailQuery = window.matchMedia("(min-width: 1441px)");
    const syncDetailPanel = () => setIsDetailPanelOpen(desktopDetailQuery.matches);
    syncDetailPanel();
    desktopDetailQuery.addEventListener("change", syncDetailPanel);
    return () => desktopDetailQuery.removeEventListener("change", syncDetailPanel);
  }, []);
  useEffect(() => {
    if (!isProfileMenuOpen) {
      return;
    }

    const closeAccountMenu = (event: PointerEvent) => {
      if (!accountMenuRef.current?.contains(event.target as Node)) {
        setIsProfileMenuOpen(false);
      }
    };
    const closeAccountMenuOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsProfileMenuOpen(false);
      }
    };

    document.addEventListener("pointerdown", closeAccountMenu);
    document.addEventListener("keydown", closeAccountMenuOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeAccountMenu);
      document.removeEventListener("keydown", closeAccountMenuOnEscape);
    };
  }, [isProfileMenuOpen]);
  useEffect(() => {
    if (data.permissionsQuery.isLoading || canAccessRailItem(routedRailItem, data.can)) {
      return;
    }
    setActiveRailItem("messages");
    data.setWorkspaceSection();
  }, [data.can, data.permissionsQuery.isLoading, data.setWorkspaceSection, routedRailItem]);
  const selectedChannelMembersQuery = useQuery({
    enabled: Boolean(data.workspaceId && data.selectedChannelId && data.canAccessSelectedChannel),
    queryFn: () => api.channels.members(data.workspaceId, data.selectedChannelId),
    queryKey: queryKeys.channels.members(data.workspaceId, data.selectedChannelId)
  });
  const selectedChannelMembers = useMemo(
    () => (selectedChannelMembersQuery.data ?? []).filter((member) => member.status === "active" || member.status === "muted"),
    [selectedChannelMembersQuery.data]
  );
  const sidebarBotsQuery = useQuery({
    enabled: Boolean(data.workspaceId && data.can("bot.manage") && activeRailItem === "messages"),
    queryFn: () => api.bots.list(data.workspaceId),
    queryKey: queryKeys.integrations.bots(data.workspaceId)
  });
  const chatTargets = useMemo(() => {
    return buildChatTargets(data.channels, data.directConversations);
  }, [data.channels, data.directConversations]);
  const forwardTargets = useMemo(
    () => chatTargets.filter((target) => target.id !== data.selectedChannelId),
    [chatTargets, data.selectedChannelId]
  );
  const uploadQueue = useUploadStore();
  const queuedUploads = useMemo(
    () => uploadQueue.items.filter((item) => item.status === "queued" || item.status === "failed"),
    [uploadQueue.items]
  );
  const hasComposerContent = Boolean(draft.trim() || queuedUploads.length);

  const canCreateChannel = data.can("channel.create");
  const isDirectChat =
    data.selectedChannel?.type === "direct" || data.selectedChannelWithMessages?.type === "direct";
  const canSendMessage = data.can("message.send") || isDirectChat;
  const canUploadFile = data.can("file.upload") || isDirectChat;
  const canUseComposer = canSendMessage && (!uploadQueue.items.length || canUploadFile);

  const refocusComposerInput = useCallback(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.setTimeout(() => {
      composerInputRef.current?.focus({ preventScroll: true });
    }, 0);
  }, []);

  const effectiveUnreadCount = (chatId: string, unreadCount = 0) => {
    if (manuallyUnreadChatIds.has(chatId)) {
      return Math.max(1, unreadCount);
    }
    return locallyReadChatIds.has(chatId) ? 0 : unreadCount;
  };

  const isFavoriteChat = (chatId: string, serverFavorite = false) =>
    !unfavoriteChatIds.has(chatId) && (serverFavorite || favoriteChatIds.has(chatId));

  const sidebarChannels = useMemo(() => {
    const normalizedQuery = searchQuery.trim().toLowerCase();

    return data.channels.filter((channel) => {
      if (channel.type === "direct") {
        return false;
      }

      const matchesFilter =
        channelFilter === "all" ||
        (channelFilter === "unread" && effectiveUnreadCount(channel.id, channel.unreadCount) > 0) ||
        (channelFilter === "favorite" && isFavoriteChat(channel.id, channel.isFavorite));

      return (
        matchesFilter &&
        (!normalizedQuery ||
          channel.name.toLowerCase().includes(normalizedQuery) ||
          channel.description.toLowerCase().includes(normalizedQuery))
      );
    });
  }, [channelFilter, data.channels, favoriteChatIds, locallyReadChatIds, manuallyUnreadChatIds, searchQuery, unfavoriteChatIds]);

  const filteredConversations = useMemo(() => {
    const normalizedQuery = searchQuery.trim().toLowerCase();

    return data.directConversations.filter((conversation) => {
      const matchesFilter =
        channelFilter === "all" ||
        (channelFilter === "unread" && effectiveUnreadCount(conversation.id, conversation.unreadCount) > 0) ||
        (channelFilter === "favorite" && isFavoriteChat(conversation.id));
      const matchesQuery =
        !normalizedQuery ||
        conversation.user.name.toLowerCase().includes(normalizedQuery) ||
        conversation.lastMessage.toLowerCase().includes(normalizedQuery);
      return matchesFilter && matchesQuery;
    });
  }, [channelFilter, data.directConversations, favoriteChatIds, locallyReadChatIds, manuallyUnreadChatIds, searchQuery, unfavoriteChatIds]);

  const sidebarBots = useMemo(() => {
    const normalizedQuery = searchQuery.trim().toLowerCase();
    return (sidebarBotsQuery.data ?? []).filter((bot) =>
      !normalizedQuery ||
      bot.name.toLowerCase().includes(normalizedQuery) ||
      bot.slug.toLowerCase().includes(normalizedQuery) ||
      (bot.description ?? "").toLowerCase().includes(normalizedQuery)
    );
  }, [searchQuery, sidebarBotsQuery.data]);

  const sidebarConversationUnreadCount = data.directConversations.reduce(
    (total, conversation) => total + (effectiveUnreadCount(conversation.id, conversation.unreadCount) > 0 ? 1 : 0),
    0
  );
  const sidebarChannelUnreadCount = data.channels.reduce(
    (total, channel) => total + (effectiveUnreadCount(channel.id, channel.unreadCount) > 0 ? 1 : 0),
    0
  );
  const unreadChatMessageCount = data.directConversations.reduce(
    (total, conversation) => total + effectiveUnreadCount(conversation.id, conversation.unreadCount),
    data.channels.reduce(
      (channelTotal, channel) =>
        channel.type === "direct" ? channelTotal : channelTotal + effectiveUnreadCount(channel.id, channel.unreadCount),
      0
    )
  );

  const contactResults = useMemo(
    () =>
      buildContactResults({
        currentUserId: currentUser?.id,
        contacts: data.contacts,
        contactRequests: data.contactRequests,
        directConversations: data.directConversations,
        members: data.members,
        query: friendSearchQuery,
        searchUsers: data.searchUsers
      }),
    [currentUser?.id, data.contactRequests, data.contacts, data.directConversations, data.members, data.searchUsers, friendSearchQuery]
  );
  const knownProfileByUserId = useMemo(() => {
    const profiles = new Map<string, ChatUser>();
    for (const conversation of data.directConversations) {
      profiles.set(conversation.user.id, conversation.user);
    }
    for (const record of [...data.contactRequests, ...data.contacts]) {
      const contact = record.user;
      const current = profiles.get(contact.id);
      profiles.set(contact.id, {
        avatarUrl: contact.avatar_url || current?.avatarUrl,
        email: contact.email || current?.email,
        id: contact.id,
        name: contact.display_name || contact.username || contact.email || current?.name || "Người dùng",
        phoneNumber: contact.phone_number || current?.phoneNumber,
        status: profilePresenceStatus(contact.status, current?.status),
        username: contact.username || current?.username
      });
    }
    profiles.set(currentUser.id, currentUser);
    return profiles;
  }, [currentUser, data.contactRequests, data.contacts, data.directConversations]);
  const resolveCallPeerName = useCallback(
    (userId?: string, channelId?: string) => {
      const profileName = userId ? knownProfileByUserId.get(userId)?.name : undefined;
      if (profileName) {
        return profileName;
      }
      if (channelId) {
        return data.directConversations.find((conversation) => conversation.id === channelId)?.user.name;
      }
      return undefined;
    },
    [data.directConversations, knownProfileByUserId]
  );
  const displayWorkspaceMembers = useMemo(
    () => enrichMemberProfiles(data.members, knownProfileByUserId),
    [data.members, knownProfileByUserId]
  );
  const displayChannelMembers = useMemo(
    () => enrichMemberProfiles(selectedChannelMembers, knownProfileByUserId),
    [knownProfileByUserId, selectedChannelMembers]
  );
  const mentionMembers = displayChannelMembers.length ? displayChannelMembers : displayWorkspaceMembers;
  const activeMentionToken = useMemo(() => resolveMentionToken(draft), [draft]);
  const mentionSuggestions = useMemo(
    () => buildMentionSuggestions(mentionMembers, currentUser.id, activeMentionToken?.query ?? ""),
    [activeMentionToken?.query, currentUser.id, mentionMembers]
  );
  const pinnedMessageIds = useMemo(
    () => new Set(data.pinnedMessages.map((message) => message.id)),
    [data.pinnedMessages]
  );
  const pinnedMessages = useMemo(
    () => {
      const memberByUserId = buildMessageAuthorLookup(displayWorkspaceMembers, displayChannelMembers);
      return data.pinnedMessages.map<PinnedMessage>((message) => ({
        author: resolveRenderedMessageAuthor(message, memberByUserId),
        date: message.sentAt,
        id: message.id,
        text: message.body
      }));
    },
    [data.pinnedMessages, displayChannelMembers, displayWorkspaceMembers]
  );
  const incomingContactRequests = useMemo(
    () => data.contactRequests.filter((request) => request.status === "pending" && request.direction === "incoming"),
    [data.contactRequests]
  );
  const unreadActivityNotificationCount = data.notifications.filter(
    (notification) => !notification.isRead && !isMessageNotification(notification)
  ).length;
  const notificationBadgeCount = unreadChatMessageCount + unreadActivityNotificationCount + incomingContactRequests.length;
  useEffect(() => {
    const services = getPlatformServices();
    if (!services.lifecycle.isDesktop) {
      return;
    }
    void services.tray.setUnreadCount(notificationBadgeCount);
  }, [notificationBadgeCount]);
  const remoteTypingLabel = useMemo(() => {
    const userId = data.realtime.typingUserIds[0];
    if (!userId) {
      return "";
    }
    const directUser = data.directConversations.find((conversation) => conversation.user.id === userId)?.user;
    const member = displayWorkspaceMembers.find((item) => item.user_id === userId);
    const name = directUser?.name || member?.display_name || member?.username || member?.email || "Ai đó";
    const extra = data.realtime.typingUserIds.length - 1;
    return extra > 0 ? `${name} và ${extra} người khác đang soạn tin` : `${name} đang soạn tin`;
  }, [data.directConversations, data.realtime.typingUserIds, displayWorkspaceMembers]);

  useEffect(() => {
    const currentIds = new Set(data.notifications.map((notification) => notification.id));
    if (!seenNotificationIdsRef.current) {
      seenNotificationIdsRef.current = currentIds;
      return;
    }

    const newest = data.notifications.find(
      (notification) => !notification.isRead && !seenNotificationIdsRef.current?.has(notification.id)
    );
    data.notifications.forEach((notification) => seenNotificationIdsRef.current?.add(notification.id));
    if (!newest) {
      return;
    }

    const isActiveMessageOpen = activeRailItem === "messages" && newest.channelId === data.selectedChannelId;
    if (isMessageNotification(newest) && !isActiveMessageOpen && shouldShowDesktopNotification(newest, notificationPreferences)) {
      setMessageNotice(newest);
    }

    const notifications = getPlatformServices().notifications;
    if (shouldShowDesktopNotification(newest, notificationPreferences)) {
      const payload = {
        body: notificationPreferences.preview ? newest.body : "Bạn có thông báo mới.",
        data: nativeNotificationData(newest, data.workspaceId),
        tag: newest.id,
        title: newest.title
      };
      const permission = notifications.getPermission();
      if (permission === "granted") {
        void notifications.show(payload);
      } else if (permission === "default") {
        void notifications.requestPermission().then((nextPermission) => {
          if (nextPermission === "granted") {
            return notifications.show(payload);
          }
          return undefined;
        });
      }
    }
  }, [activeRailItem, data.notifications, data.selectedChannelId, data.workspaceId, notificationPreferences]);

  useEffect(() => {
    if (!data.workspaceId || typeof window === "undefined") {
      return;
    }
    let active = true;
    const applyStoredPreferences = (stored: string | null) => {
      if (!active) {
        return;
      }
      if (!stored) {
        setFavoriteChatIds(new Set());
        setUnfavoriteChatIds(new Set());
        setManuallyUnreadChatIds(new Set());
        return;
      }
      const preferences = JSON.parse(stored) as { favorites?: string[]; unfavorites?: string[]; unread?: string[] };
      setFavoriteChatIds(new Set(preferences.favorites ?? []));
      setUnfavoriteChatIds(new Set(preferences.unfavorites ?? []));
      setManuallyUnreadChatIds(new Set(preferences.unread ?? []));
    };
    const resetStoredPreferences = () => {
      if (!active) {
        return;
      }
      setFavoriteChatIds(new Set());
      setUnfavoriteChatIds(new Set());
      setManuallyUnreadChatIds(new Set());
    };

    try {
      const stored = getPlatformServices().storage.getItem(`vpsttt:chat-preferences:${data.workspaceId}`);
      if (stored instanceof Promise) {
        void stored.then(applyStoredPreferences).catch(resetStoredPreferences);
      } else {
        applyStoredPreferences(stored);
      }
    } catch {
      resetStoredPreferences();
    }

    return () => {
      active = false;
    };
  }, [data.workspaceId]);

  useEffect(() => {
    if (!data.workspaceId || typeof window === "undefined") {
      return;
    }

    let active = true;
    const key = notificationPreferencesStorageKey(data.workspaceId);
    const applyPreferences = (preferences: NotificationPreferences) => {
      if (active) {
        setNotificationPreferences(preferences);
      }
    };
    const cachePreferences = (preferences: NotificationPreferences) => {
      try {
        void getPlatformServices().storage.setItem(key, JSON.stringify(preferences), "persistent");
      } catch {
        // Local cache is best-effort; backend remains the source of truth.
      }
    };
    const loadLocalPreferences = () => {
      try {
        const stored = getPlatformServices().storage.getItem(key);
        if (stored instanceof Promise) {
          void stored.then((value) => applyPreferences(parseNotificationPreferences(value))).catch(() => applyPreferences(defaultNotificationPreferences));
        } else {
          applyPreferences(parseNotificationPreferences(stored));
        }
      } catch {
        applyPreferences(defaultNotificationPreferences);
      }
    };

    loadLocalPreferences();
    void api.notifications.getPreferences(data.workspaceId)
      .then((preference) => {
        const next = mapNotificationPreference(preference);
        applyPreferences(next);
        cachePreferences(next);
      })
      .catch(() => undefined);

    return () => {
      active = false;
    };
  }, [data.workspaceId]);

  useEffect(() => {
    if (!getPlatformServices().lifecycle.isDesktop) {
      return;
    }

    let active = true;
    setIsAutoStartLoading(true);
    void getPlatformServices().autostart.isEnabled()
      .then((enabled) => {
        if (active) {
          setIsAutoStartEnabled(enabled);
        }
      })
      .catch(() => undefined)
      .finally(() => {
        if (active) {
          setIsAutoStartLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    setLocallyReadChatIds((current) => {
      const next = new Set(current);
      let changed = false;
      for (const chatId of current) {
        const channel = data.channels.find((item) => item.id === chatId);
        const conversation = data.directConversations.find((item) => item.id === chatId);
        if ((channel && channel.unreadCount === 0) || (conversation && !conversation.unreadCount)) {
          next.delete(chatId);
          changed = true;
        }
      }
      return changed ? next : current;
    });
  }, [data.channels, data.directConversations]);
  const selectedChatChannel = useMemo(() => {
    if (!data.selectedChannelWithMessages) {
      return null;
    }
    const directConversation = data.directConversations.find(
      (conversation) => conversation.id === data.selectedChannelWithMessages?.id
    );
    if (!directConversation) {
      return {
        ...data.selectedChannelWithMessages,
        memberCount: selectedChannelMembersQuery.data ? selectedChannelMembers.length : data.selectedChannelWithMessages.memberCount
      };
    }
    return {
      ...data.selectedChannelWithMessages,
      avatarUrl: directConversation.user.avatarUrl,
      description: "Tin nhắn riêng",
      memberCount: 2,
      name: directConversation.user.name,
      peerUserId: directConversation.user.id,
      userStatus: directConversation.user.status
    };
  }, [data.directConversations, data.selectedChannelWithMessages, selectedChannelMembers, selectedChannelMembersQuery.data]);
  const callControls = useWebRtcCall({
    channelId: data.selectedChannelId,
    channelName: selectedChatChannel?.name,
    currentUserId: currentUser.id,
    enabled: Boolean(data.workspaceId),
    lastSignal: data.realtime.lastCallSignal,
    onCallOutcome: handleCallOutcome,
    peerName: selectedChatChannel?.name,
    peerUserId: selectedChatChannel?.peerUserId,
    resolvePeerName: resolveCallPeerName,
    workspaceId: data.workspaceId
  });
  useIncomingCallRingtone(callControls.callState.status === "incoming");
  useEffect(() => {
    if (!data.selectedChannelId || callControls.callState.status !== "idle") {
      return;
    }
    const incoming = data.notifications.find(
      (notification) => notification.channelId === data.selectedChannelId && isIncomingCallNotification(notification, currentUser.id)
    );
    if (incoming?.callId) {
      void callControls.openIncomingCall(incoming.callId);
    }
  }, [callControls.callState.status, callControls.openIncomingCall, currentUser.id, data.notifications, data.selectedChannelId]);
  function handleCallOutcome(outcome: WebRtcCallOutcome) {
    if (!data.selectedChannelId || selectedChatChannel?.type !== "direct") {
      return;
    }
    data.sendCallEventMutation.mutate({
      callId: outcome.callId,
      durationSeconds: outcome.durationSeconds,
      endedAt: new Date(outcome.endedAt).toISOString(),
      initiatorUserId: outcome.initiatorUserId,
      mode: outcome.mode,
      reason: outcome.reason,
      startedAt: outcome.startedAt ? new Date(outcome.startedAt).toISOString() : undefined,
      status: outcome.status
    });
  }

  const composerPlaceholder = botComposerPlaceholder(selectedChatChannel);
  const selectedChatFiles = useMemo(() => {
    const fileById = new Map<string, FileItem>();
    for (const message of selectedChatChannel?.messages ?? []) {
      for (const attachment of message.attachments ?? []) {
        if (attachment.isImage || attachment.isAudio || attachment.isVideo || fileById.has(attachment.fileId)) {
          continue;
        }
        fileById.set(attachment.fileId, {
          checksumSha256: attachment.checksumSha256,
          downloadUrl: attachment.url,
          id: attachment.fileId,
          mimeType: attachment.mimeType,
          name: attachment.name,
          size: attachment.size ?? attachment.mimeType ?? "File",
          status: attachment.status,
          tone: attachment.tone,
          updatedAt: message.sentAt
        });
      }
    }
    return Array.from(fileById.values());
  }, [selectedChatChannel?.messages]);

  const selectedRailLabel = railItems.find((item) => item.id === activeRailItem)?.label ?? "Tin nhắn";
  const panelTitle = activeRailItem === "files" ? "Tệp tin" : selectedRailLabel;

  useEffect(() => {
    if (!getPlatformServices().lifecycle.isDesktop) {
      return;
    }

    let deepLinkCleanup: (() => void) | undefined;
    let notificationCleanup: (() => void) | undefined;
    const services = getPlatformServices();
    const handleUrls = (urls: string[]) => urls.forEach((url) => openDesktopDeepLink(url));

    void services.deepLinks.registerProtocol("webtui").catch(() => undefined);
    void services.deepLinks.getInitialUrls().then(handleUrls).catch(() => undefined);
    void services.deepLinks.onOpenUrl(handleUrls).then((cleanup) => {
      deepLinkCleanup = cleanup;
    }).catch(() => undefined);
    void services.notifications.onClick((payload) => {
      openDesktopNotificationPayload(payload.data);
    }).then((cleanup) => {
      notificationCleanup = cleanup;
    }).catch(() => undefined);

    return () => {
      deepLinkCleanup?.();
      notificationCleanup?.();
    };
  }, [data.setSelectedChannelId, data.setWorkspaceSection, data.workspaceId]);

  async function handleCreateChannel(input: CreateChannelPayload) {
    if (!canCreateChannel) {
      setToast("Tài khoản hiện tại chưa có quyền tạo kênh.");
      return;
    }

    data.createChannelMutation.mutate(input, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không tạo được kênh."),
      onSuccess: () => {
        setChannelFilter("all");
        setIsCreateChannelOpen(false);
        setToast("Đã tạo kênh mới.");
      }
    });
  }

  function showMessageWorkspace(tab: MessageSidebarTab) {
    closeTransientWorkspaceUi();
    setMessageSidebarTab(tab);
    setThreadMessageId(null);
    setIsMessageSearchOpen(false);
    setMessageSearchQuery("");
    setMessageSearchChannelId("");
    setMessageSearchSenderId("");
    setMessageSearchKind("");
    setMessageSearchDateFrom("");
    setMessageSearchDateTo("");
    setActiveRailItem("messages");
  }

  function handleChannelSelect(channelId: string) {
    const channel = data.channels.find((item) => item.id === channelId);
    if (channel?.privateSessionMode) {
      data.openPrivateSessionMutation.mutate(channelId, {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không mở được phiên làm việc riêng tư."),
        onSuccess: () => showMessageWorkspace("channels")
      });
      return;
    }
    showMessageWorkspace(data.directConversations.some((conversation) => conversation.id === channelId) ? "conversations" : "channels");
    data.setSelectedChannelId(channelId);
    if (manuallyUnreadChatIds.has(channelId)) {
      const nextUnread = new Set(manuallyUnreadChatIds);
      nextUnread.delete(channelId);
      setManuallyUnreadChatIds(nextUnread);
      persistChatPreferences(favoriteChatIds, nextUnread);
    }
  }

  function openDesktopDeepLink(url: string) {
    const target = parseDesktopDeepLink(url);
    if (!target) {
      return;
    }
    openDesktopTarget(target);
  }

  function openDesktopNotificationPayload(payload?: Record<string, unknown>) {
    if (!payload) {
      return;
    }
    const channelId = typeof payload.channelId === "string" ? payload.channelId : typeof payload.channel_id === "string" ? payload.channel_id : "";
    const workspaceId = typeof payload.workspaceId === "string" ? payload.workspaceId : typeof payload.workspace_id === "string" ? payload.workspace_id : data.workspaceId;
    const messageId = typeof payload.messageId === "string" ? payload.messageId : typeof payload.message_id === "string" ? payload.message_id : "";
    const callId = typeof payload.callId === "string" ? payload.callId : typeof payload.call_id === "string" ? payload.call_id : "";
    if (!channelId) {
      if (callId) {
        void callControls.openIncomingCall(callId);
      }
      return;
    }
    openDesktopTarget({ channelId, kind: "channel", messageId, workspaceId });
    if (callId) {
      void callControls.openIncomingCall(callId);
    }
  }

  function openDesktopTarget(target: DesktopDeepLinkTarget) {
    if (target.section && railItems.some((item) => item.id === target.section)) {
      setActiveRailItem(target.section as RailItemId);
      data.setWorkspaceSection(target.section);
      return;
    }

    if (!target.channelId) {
      return;
    }

    const kind = target.kind === "dm" ? "direct" : "channel";
    showMessageWorkspace(target.kind === "dm" ? "conversations" : "channels");
    data.setSelectedChannelId(target.channelId, target.workspaceId || data.workspaceId, kind);
    if (target.messageId) {
      setFocusedMessageId(target.messageId);
      setThreadMessageId(null);
    }
  }

  function handleMessageSidebarTabChange(tab: MessageSidebarTab) {
    setMessageSidebarTab(tab);
    setChannelFilter("all");
  }

  function handleRailSelect(itemId: RailItemId) {
    if (!canAccessRailItem(itemId, data.can)) {
      setToast("Tài khoản của bạn chỉ được sử dụng các chức năng trao đổi trong workspace.");
      return;
    }
    closeTransientWorkspaceUi();
    setIsProfileMenuOpen(false);
    if (itemId === "messages" || itemId === "channels") {
      const nextMessageTab = itemId === "messages" ? "conversations" : "channels";
      handleMessageSidebarTabChange(nextMessageTab);
      setActiveRailItem("messages");
      data.setWorkspaceSection(undefined, nextMessageTab);
      return;
    }
    setActiveRailItem(itemId);
    data.setWorkspaceSection(itemId);
  }

  function closeTransientWorkspaceUi() {
    setIsCreateChannelOpen(false);
    setIsTicketCreateOpen(false);
    setIsNotificationsOpen(false);
    setIsEmojiPickerOpen(false);
    setIsComposerMoreOpen(false);
    setForwardingMessageId(null);
    setEditingBody("");
    setEditingMessageId(null);
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      recordingCancelledRef.current = true;
      handleStopRecording();
    }
  }

  function handleToggleMessageSearch() {
    setIsMessageSearchOpen((current) => {
      if (current) {
        setMessageSearchQuery("");
        setMessageSearchChannelId("");
        setMessageSearchSenderId("");
        setMessageSearchKind("");
        setMessageSearchDateFrom("");
        setMessageSearchDateTo("");
      }
      return !current;
    });
  }

  function handleToggleNotifications() {
    const willOpen = !isNotificationsOpen;
    setIsNotificationsOpen(willOpen);
    const notifications = getPlatformServices().notifications;
    if (willOpen && notifications.getPermission() === "default") {
      void notifications.requestPermission();
    }
  }

  function handleNotificationPreferencesChange(next: NotificationPreferences) {
    const normalized = normalizeNotificationPreferences(next);
    setNotificationPreferences(normalized);
    if (data.workspaceId) {
      void getPlatformServices().storage.setItem(
        notificationPreferencesStorageKey(data.workspaceId),
        JSON.stringify(normalized),
        "persistent"
      );
      if (isTimeValue(normalized.quietStart) && isTimeValue(normalized.quietEnd)) {
        void api.notifications.updatePreferences(toNotificationPreferenceInput(data.workspaceId, normalized)).catch(() => {
          setToast("Không đồng bộ được cài đặt thông báo lên máy chủ.");
        });
      }
    }
  }

  async function handleAutoStartChange(enabled: boolean) {
    if (!getPlatformServices().lifecycle.isDesktop) {
      setToast("Tự khởi động chỉ hỗ trợ trên ứng dụng desktop.");
      return;
    }

    setIsAutoStartLoading(true);
    try {
      if (enabled) {
        await getPlatformServices().autostart.enable();
      } else {
        await getPlatformServices().autostart.disable();
      }
      setIsAutoStartEnabled(enabled);
      setToast(enabled ? "Đã bật tự khởi động cùng hệ điều hành." : "Đã tắt tự khởi động.");
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Không cập nhật được tự khởi động.");
    } finally {
      setIsAutoStartLoading(false);
    }
  }

  async function handleDesktopUpdateInstall() {
    const services = getPlatformServices();

    if (!services.lifecycle.isDesktop) {
      if (desktopVersionStatus.updateUrl) {
        await services.links.openExternal(desktopVersionStatus.updateUrl);
        return;
      }
      setToast("Cập nhật tự động chỉ hỗ trợ trên ứng dụng desktop.");
      return;
    }

    setIsDesktopUpdateInstalling(true);
    setToast("Đang kiểm tra bản cập nhật desktop...", "info");

    try {
      const result = await services.updates.checkAndInstall();
      if (!result.available) {
        setToast("Phiên bản desktop đang là mới nhất.", "success");
        return;
      }

      setToast(`Đã cài bản ${result.version}. Khởi động lại ứng dụng để hoàn tất.`, "success");
    } catch (error) {
      if (desktopVersionStatus.updateUrl) {
        setToast("Không cập nhật tự động được, đang mở file cài đặt mới.", "info");
        await services.links.openExternal(desktopVersionStatus.updateUrl);
        return;
      }
      setToast(error instanceof Error ? error.message : "Không cập nhật được ứng dụng desktop.");
    } finally {
      setIsDesktopUpdateInstalling(false);
    }
  }

  function handleCloseMessageSearch() {
    setIsMessageSearchOpen(false);
    setMessageSearchQuery("");
    setMessageSearchChannelId("");
    setMessageSearchSenderId("");
    setMessageSearchKind("");
    setMessageSearchDateFrom("");
    setMessageSearchDateTo("");
  }

  useEffect(() => {
    function handleWorkspaceShortcut(event: KeyboardEvent) {
      const target = event.target;
      const isEditableTarget =
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement ||
        (target instanceof HTMLElement && target.isContentEditable);
      const isModKey = event.ctrlKey || event.metaKey;

      if (isModKey && event.key.toLowerCase() === "k") {
        event.preventDefault();
        handleToggleMessageSearch();
        return;
      }

      if (!isEditableTarget && event.key === "/" && canUseComposer) {
        event.preventDefault();
        composerInputRef.current?.focus();
        return;
      }

      if (event.key === "Escape") {
        setIsEmojiPickerOpen(false);
        setIsNotificationsOpen(false);
        if (isMessageSearchOpen) {
          handleCloseMessageSearch();
        }
      }
    }

    window.addEventListener("keydown", handleWorkspaceShortcut);
    return () => window.removeEventListener("keydown", handleWorkspaceShortcut);
  }, [canUseComposer, isMessageSearchOpen]);

  useEffect(() => {
    if (!getPlatformServices().lifecycle.isDesktop) {
      return;
    }

    function handleExternalLinkClick(event: MouseEvent) {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }
      const link = target.closest("a[href]");
      if (!(link instanceof HTMLAnchorElement) || !shouldOpenExternally(link)) {
        return;
      }
      event.preventDefault();
      void getPlatformServices().links.openExternal(link.href).catch(() => {
        setToast("Không mở được liên kết bên ngoài.");
      });
    }

    document.addEventListener("click", handleExternalLinkClick);
    return () => document.removeEventListener("click", handleExternalLinkClick);
  }, []);

  function persistChatPreferences(favorites: Set<string>, unread: Set<string>, unfavorites = unfavoriteChatIds) {
    if (!data.workspaceId || typeof window === "undefined") {
      return;
    }
    void getPlatformServices().storage.setItem(
      `vpsttt:chat-preferences:${data.workspaceId}`,
      JSON.stringify({ favorites: [...favorites], unfavorites: [...unfavorites], unread: [...unread] }),
      "persistent"
    );
  }

  function handleToggleFavorite(chatId: string) {
    const next = new Set(favoriteChatIds);
    const nextUnfavorites = new Set(unfavoriteChatIds);
    const serverFavorite = data.channels.find((channel) => channel.id === chatId)?.isFavorite ?? false;
    const isCurrentlyFavorite = isFavoriteChat(chatId, serverFavorite);
    if (isCurrentlyFavorite) {
      next.delete(chatId);
      if (serverFavorite) {
        nextUnfavorites.add(chatId);
      }
    } else {
      next.add(chatId);
      nextUnfavorites.delete(chatId);
    }
    setFavoriteChatIds(next);
    setUnfavoriteChatIds(nextUnfavorites);
    persistChatPreferences(next, manuallyUnreadChatIds, nextUnfavorites);
  }

  function handleMarkUnread(chatId: string) {
    const next = new Set(manuallyUnreadChatIds).add(chatId);
    setManuallyUnreadChatIds(next);
    setLocallyReadChatIds((current) => {
      const updated = new Set(current);
      updated.delete(chatId);
      return updated;
    });
    persistChatPreferences(favoriteChatIds, next);
  }

  async function handlePickUploadFiles(kind: "file" | "image") {
    if (!canUploadFile) {
      setToast(kind === "image" ? "Tài khoản hiện tại chưa có quyền gửi ảnh." : "Tài khoản hiện tại chưa có quyền upload file.");
      return;
    }

    try {
      const files = await getPlatformServices().files.pickFiles({
        accept: kind === "image" ? imageUploadAcceptList : uploadAcceptList,
        multiple: true,
        title: kind === "image" ? "Chọn ảnh" : "Chọn file"
      });
      addFilesToUploadQueue(files, { imageOnly: kind === "image", source: "picker" });
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Không chọn được file.");
    }
  }

  function addFilesToUploadQueue(
    files: File[],
    options: { imageOnly?: boolean; source?: "drop" | "input" | "paste" | "picker" } = {}
  ) {
    if (!files.length) {
      return;
    }

    const { accepted, rejected } = validateUploadFiles(files, options.imageOnly);
    if (accepted.length) {
      uploadQueue.addFiles(accepted);
    }

    if (!rejected.length) {
      return;
    }

    const firstReason = rejected[0]?.reason ?? "File không hợp lệ.";
    const suffix = rejected.length > 1 ? ` (+${rejected.length - 1} file khác)` : "";
    setToast(`${firstReason}${suffix}`);
  }

  function handleComposerDragOver(event: DragEvent<HTMLDivElement>) {
    if (!event.dataTransfer.types.includes("Files")) {
      return;
    }
    event.preventDefault();
    if (canUploadFile) {
      event.dataTransfer.dropEffect = "copy";
      setIsComposerDragActive(true);
    }
  }

  function handleComposerDragLeave(event: DragEvent<HTMLDivElement>) {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
      setIsComposerDragActive(false);
    }
  }

  function handleComposerDrop(event: DragEvent<HTMLDivElement>) {
    if (!event.dataTransfer.files.length) {
      return;
    }
    event.preventDefault();
    setIsComposerDragActive(false);

    if (!canUploadFile) {
      setToast("Tài khoản hiện tại chưa có quyền upload file.");
      return;
    }

    addFilesToUploadQueue(Array.from(event.dataTransfer.files), { source: "drop" });
  }

  function handleComposerPaste(event: ClipboardEvent<HTMLInputElement>) {
    const itemFiles = Array.from(event.clipboardData.items)
      .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
      .map((item) => item.getAsFile())
      .filter((file): file is File => file !== null);
    const clipboardFiles = itemFiles.length
      ? itemFiles
      : Array.from(event.clipboardData.files).filter((file) => file.type.startsWith("image/"));

    const imageFiles = clipboardFiles.map((file, index) => {
      if (file.name) {
        return file;
      }

      const extension = file.type.includes("jpeg") ? "jpg" : file.type.includes("webp") ? "webp" : "png";
      return new File([file], `anh-dan-${Date.now()}-${index}.${extension}`, {
        type: file.type || "image/png"
      });
    });

    if (!imageFiles.length) {
      return;
    }

    event.preventDefault();

    if (!canUploadFile) {
      setToast("Tài khoản hiện tại chưa có quyền gửi ảnh.");
      return;
    }

    addFilesToUploadQueue(imageFiles, { imageOnly: true, source: "paste" });
  }

  function handleEmojiSelect(emoji: string) {
    handleDraftChange(`${draft}${emoji}`);
    setIsEmojiPickerOpen(false);
  }

  function handleInsertMention(member: ChannelMember | WorkspaceMember) {
    const token = resolveMentionToken(draft);
    if (!token) {
      return;
    }

    handleDraftChange(`${draft.slice(0, token.start)}@${mentionMemberName(member)} `);
    window.requestAnimationFrame(() => composerInputRef.current?.focus());
  }

  function handleDraftChange(value: string) {
    setDraft(value);
    if (data.workspaceId && data.selectedChannelId) {
      void writeDraft(data.workspaceId, data.selectedChannelId, value).catch(() => undefined);
    }
    if (typingStopTimerRef.current) {
      clearTimeout(typingStopTimerRef.current);
      typingStopTimerRef.current = null;
    }
    const isTyping = Boolean(value.trim());
    if (isTyping && !typingPublishedRef.current) {
      data.realtime.publishTyping(true);
      typingPublishedRef.current = true;
    } else if (!isTyping && typingPublishedRef.current) {
      data.realtime.publishTyping(false);
      typingPublishedRef.current = false;
    }
    if (isTyping) {
      typingStopTimerRef.current = setTimeout(() => {
        data.realtime.publishTyping(false);
        typingPublishedRef.current = false;
        typingStopTimerRef.current = null;
      }, 1_400);
    }
  }

  function handleContactPrimaryAction(contact: ContactResult) {
    if (contact.contactStatus === "none" || contact.contactStatus === "rejected") {
      data.sendContactRequestMutation.mutate(contact.userId, {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không gửi được lời mời kết bạn."),
        onSuccess: () => setToast(`Đã gửi lời mời kết bạn đến ${contact.name}.`)
      });
      return;
    }

    if (contact.contactStatus === "pending" && contact.contactDirection === "incoming" && contact.contactRequestId) {
      data.acceptContactRequestMutation.mutate(contact.contactRequestId, {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không đồng ý được lời mời kết bạn."),
        onSuccess: () => {
          setToast(`Bạn và ${contact.name} đã là bạn bè.`);
          void openAcceptedContact(contact);
        }
      });
      return;
    }

    if (contact.contactStatus === "pending") {
      setToast("Lời mời kết bạn đang chờ phản hồi.");
      return;
    }

    void openAcceptedContact(contact);
  }

  function handleContactSecondaryAction(contact: ContactResult) {
    if (contact.contactStatus !== "pending" || !contact.contactRequestId) {
      return;
    }

    if (contact.contactDirection === "incoming") {
      data.rejectContactRequestMutation.mutate(contact.contactRequestId, {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không từ chối được lời mời kết bạn."),
        onSuccess: () => setToast(`Đã từ chối lời mời từ ${contact.name}.`)
      });
      return;
    }

    data.cancelContactRequestMutation.mutate(contact.contactRequestId, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không hủy được lời mời kết bạn."),
      onSuccess: () => setToast(`Đã hủy lời mời kết bạn đến ${contact.name}.`)
    });
  }

  async function openAcceptedContact(contact: ContactResult) {
    if (!data.workspaceId) {
      setToast(
        data.workspacesQuery.isLoading || data.workspacesQuery.isFetching
          ? "Đang khởi tạo workspace cho tài khoản, vui lòng chờ trong giây lát."
          : "Workspace mặc định chưa sẵn sàng. Vui lòng tải lại trang để hệ thống tự đồng bộ."
      );
      return;
    }

    handleStartDirectConversation(contact.userId);
  }

  async function handleToggleRecording() {
    if (isRecording) {
      handleStopRecording();
      return;
    }

    if (!canUploadFile) {
      setToast("Tài khoản hiện tại chưa có quyền upload file ghi âm.");
      return;
    }

    try {
      const preferredMimeType = preferredVoiceMimeType();
      let recordingMimeType = preferredMimeType || "audio/webm";
      recordingCancelledRef.current = false;
      recordingChunksRef.current = [];
      recordingPausedAtRef.current = null;
      recordingPausedMsRef.current = 0;
      recordingStartedAtRef.current = Date.now();

      const recorder = await getPlatformServices().media.createAudioRecorder({
        audioBitsPerSecond: 64_000,
        mimeType: preferredMimeType,
        onDataAvailable: (blob) => {
          recordingChunksRef.current.push(blob);
        },
        onError: () => {
          setToast("Ghi âm bị gián đoạn. Vui lòng thử lại.");
        },
        onStop: () => {
          const pausedMs = recordingPausedAtRef.current
            ? recordingPausedMsRef.current + Date.now() - recordingPausedAtRef.current
            : recordingPausedMsRef.current;
          const blob = new Blob(recordingChunksRef.current, { type: recordingMimeType });
          const extension = voiceFileExtension(recordingMimeType);
          const durationSeconds = Math.max(
            1,
            Math.round((Date.now() - (recordingStartedAtRef.current ?? Date.now()) - pausedMs) / 1000)
          );
          if (recordingCancelledRef.current) {
            setToast("Đã hủy bản ghi âm.");
          } else {
            const file = new File([blob], `voice-${Date.now()}.${extension}`, { type: recordingMimeType });
            if (file.size > 0) {
              uploadQueue.addVoice(file, durationSeconds);
              setToast("Đã ghi âm xong. Nhấn Gửi để gửi tin nhắn thoại.");
            } else {
              setToast("Không thu được âm thanh. Vui lòng kiểm tra micro và thử lại.");
            }
          }
          recordingCancelledRef.current = false;
          recordingStartedAtRef.current = null;
          mediaRecorderRef.current = null;
          recordingChunksRef.current = [];
          recordingPausedAtRef.current = null;
          recordingPausedMsRef.current = 0;
          setRecordingSeconds(0);
          setIsRecording(false);
          setIsRecordingPaused(false);
        }
      });

      mediaRecorderRef.current = recorder;
      recordingMimeType = recorder.mimeType || recordingMimeType;
      recorder.start(250);
      setRecordingSeconds(0);
      setIsRecording(true);
      setIsRecordingPaused(false);
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Không bật được micro.");
      recordingCancelledRef.current = false;
      recordingStartedAtRef.current = null;
      mediaRecorderRef.current = null;
      recordingPausedAtRef.current = null;
      recordingPausedMsRef.current = 0;
      setIsRecording(false);
      setIsRecordingPaused(false);
    }
  }

  function handleStopRecording() {
    const recorder = mediaRecorderRef.current;
    if (!recorder || recorder.state === "inactive") {
      return;
    }
    recorder.stop();
  }

  function handlePauseRecording() {
    const recorder = mediaRecorderRef.current;
    if (!recorder || recorder.state !== "recording" || !recorder.pause) {
      return;
    }
    recorder.pause();
    recordingPausedAtRef.current = Date.now();
    setIsRecordingPaused(true);
  }

  function handleResumeRecording() {
    const recorder = mediaRecorderRef.current;
    if (!recorder || recorder.state !== "paused" || !recorder.resume) {
      return;
    }
    if (recordingPausedAtRef.current) {
      recordingPausedMsRef.current += Date.now() - recordingPausedAtRef.current;
      recordingPausedAtRef.current = null;
    }
    recorder.resume();
    setIsRecordingPaused(false);
  }

  function handleCancelRecording() {
    if (!mediaRecorderRef.current || mediaRecorderRef.current.state === "inactive") {
      return;
    }
    recordingCancelledRef.current = true;
    handleStopRecording();
  }

  useEffect(() => {
    if (!isRecording || isRecordingPaused) {
      return;
    }
    const timer = window.setInterval(() => setRecordingSeconds((seconds) => seconds + 1), 1000);
    return () => window.clearInterval(timer);
  }, [isRecording, isRecordingPaused]);

  useEffect(() => {
    if (isRecording && !isRecordingPaused && recordingSeconds >= 300 && mediaRecorderRef.current?.state === "recording") {
      handleStopRecording();
      setToast("Tin nhắn thoại đã đạt giới hạn 5 phút và được dừng tự động.");
    }
  }, [isRecording, isRecordingPaused, recordingSeconds]);

  useEffect(
    () => () => {
      if (typingStopTimerRef.current) {
        clearTimeout(typingStopTimerRef.current);
        typingStopTimerRef.current = null;
      }
      if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
        recordingCancelledRef.current = true;
        mediaRecorderRef.current.stop();
      }
      data.realtime.publishTyping(false);
      typingPublishedRef.current = false;
    },
    [data.realtime.publishTyping]
  );

  useEffect(() => {
    let disposed = false;
    if (!data.workspaceId || !data.selectedChannelId) {
      setDraft("");
      setReplyingTo(null);
      return undefined;
    }
    // Never expose the previous conversation's draft while IndexedDB is loading.
    setDraft("");
    setReplyingTo(null);
    void readDraft(data.workspaceId, data.selectedChannelId)
      .then((value) => {
        if (!disposed) {
          setDraft(value);
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, [data.selectedChannelId, data.workspaceId]);

  function sendComposerMessage(body: string, uploads: UploadQueueItem[]) {
    if (!body && !uploads.length) {
      return;
    }

    if (!canUseComposer) {
      setToast(uploads.length ? "Bạn cần quyền gửi tin nhắn và upload file." : "Bạn cần quyền gửi tin nhắn.");
      return;
    }

    const mentionedUserIds = collectMentionedUserIds(body, mentionMembers);

    data.sendMessageMutation.mutate(
      {
        body,
        mentionedUserIds,
        parentId: replyingTo?.messageId,
        replyTo: replyingTo ?? undefined,
        uploads
      },
      {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không gửi được nội dung."),
        onSettled: refocusComposerInput,
        onSuccess: (result) => {
          setDraft("");
          setReplyingTo(null);
          if (data.workspaceId && data.selectedChannelId) {
            void writeDraft(data.workspaceId, data.selectedChannelId, "").catch(() => undefined);
          }
          data.realtime.publishTyping(false);
          typingPublishedRef.current = false;
          uploadQueue.clearAttached();
          if (result.queued) {
            setToast("Đang offline, tin nhắn đã được lưu vào hàng chờ gửi.");
            return;
          }
          if (result.failedUploadNames.length) {
            setToast(`Tin nhắn đã gửi, ${result.failedUploadNames.length} file cần thử lại.`);
          }
        }
      }
    );
  }

  function handleMobileBackToList() {
    setIsDetailPanelOpen(false);
    setThreadMessageId(null);
    handleCloseMessageSearch();
    data.setSelectedChannelId("");
  }

  function handleSendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    sendComposerMessage(draft.trim(), queuedUploads);
  }

  function handleSendLike() {
    sendComposerMessage("👍", []);
  }

  function handleReplyToMessage(message: ChatMessage, author: ChatUser) {
    setReplyingTo(createMessageReplyPreview(message, author));
    setIsEmojiPickerOpen(false);
    setIsComposerMoreOpen(false);
    refocusComposerInput();
  }

  function handleDownload(file: FileItem) {
    data.downloadMutation.mutate(file, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không tải được file."),
      onSuccess: (blob) => {
        void saveDownloadedBlob(blob, file);
      }
    });
  }

  async function saveDownloadedBlob(blob: Blob, file: FileItem) {
    if (file.checksumSha256) {
      const isValid = await verifyBlobChecksum(blob, file.checksumSha256).catch(() => true);
      if (!isValid) {
        setToast("Checksum file không khớp. File tải xuống có thể đã bị lỗi, vui lòng thử lại.");
        return;
      }
    }

    await getPlatformServices().files.saveBlob(blob, file.name).catch((error: unknown) => {
      setToast(error instanceof Error ? error.message : "Không lưu được file.");
    });
  }

  function handleDownloadAttachment(attachment: MessageAttachmentItem) {
    handleDownload({
      checksumSha256: attachment.checksumSha256,
      id: attachment.fileId,
      mimeType: attachment.mimeType,
      name: attachment.name,
      size: attachment.size ?? "",
      tone: attachment.tone,
      updatedAt: ""
    });
  }

  function handlePreviewAttachment(attachment: MessageAttachmentItem, source?: string) {
    const resolvedSource = source ?? attachment.previewUrl ?? attachment.url ?? getCachedMediaUrl(attachment.fileId);
    if (!resolvedSource) {
      return;
    }
    setPreviewedImage({ attachment, source: resolvedSource });
  }

  function handlePreviewMediaItem(item: MediaItem, source?: string) {
    handlePreviewAttachment(item.attachment, source);
  }

  function handleOpenNotification(notification: NotificationItem) {
    data.markNotificationReadMutation.mutate(notification.id, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không đánh dấu được thông báo.")
    });

    if (notification.channelId) {
      data.setSelectedChannelId(notification.channelId);
      setActiveRailItem("messages");
    }

    if (notification.messageId) {
      setThreadMessageId(null);
      setFocusedMessageId(notification.messageId);
    }

    if (isIncomingCallNotification(notification, currentUser.id) && notification.callId) {
      void callControls.openIncomingCall(notification.callId);
    }

    setIsNotificationsOpen(false);
  }

  function handleMarkAllNotificationsRead() {
    data.markAllNotificationsReadMutation.mutate(undefined, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không đánh dấu được thông báo."),
      onSuccess: () => setToast("Đã đánh dấu tất cả thông báo là đã đọc.")
    });
  }

  function handleAcceptIncomingRequest(request: ContactRequest) {
    data.acceptContactRequestMutation.mutate(request.id, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không đồng ý được lời mời kết bạn."),
      onSuccess: () => {
        setToast(`Bạn và ${request.user.display_name || request.user.username} đã là bạn bè.`);
        setIsNotificationsOpen(false);
        void openAcceptedContact(contactResultFromRequest(request, data));
      }
    });
  }

  function handleRejectIncomingRequest(request: ContactRequest) {
    data.rejectContactRequestMutation.mutate(request.id, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không từ chối được lời mời kết bạn."),
      onSuccess: () => setToast(`Đã từ chối lời mời từ ${request.user.display_name || request.user.username}.`)
    });
  }

  function handleStartDirectConversation(userId: string, workspaceId?: string) {
    if (!userId) {
      return;
    }

    data.createDirectConversationMutation.mutate(workspaceId ? { participantId: userId, workspaceId } : userId, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không tạo được hội thoại riêng."),
      onSuccess: (conversation) => {
        const channelId = conversation.channel_id ?? conversation.id;
        if (channelId) {
          data.setSelectedChannelId(channelId, workspaceId, "direct");
        }
        setThreadMessageId(null);
        setActiveRailItem("messages");
      }
    });
  }

  function handleStartEdit(message: ChatMessage) {
    if (!message.canEdit) {
      setToast("Bạn không có quyền sửa tin nhắn này.");
      return;
    }

    setEditingMessageId(message.id);
    setEditingBody(message.body);
  }

  function handleSubmitEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = editingBody.trim();

    if (!editingMessageId || !body) {
      return;
    }

    data.editMessageMutation.mutate(
      {
        body,
        messageId: editingMessageId
      },
      {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không sửa được tin nhắn."),
        onSuccess: () => {
          setEditingBody("");
          setEditingMessageId(null);
        }
      }
    );
  }

  function handleDeleteMessage(message: ChatMessage) {
    if (!message.canDelete) {
      setToast("Bạn không có quyền xóa tin nhắn này.");
      return;
    }

    data.deleteMessageMutation.mutate(
      { messageId: message.id },
      {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không xóa được tin nhắn."),
        onSuccess: () => {
          if (threadMessageId === message.id) {
            setThreadMessageId(null);
          }
        }
      }
    );
  }

  function handleToggleReaction(message: ChatMessage, emoji: string) {
    const reaction = message.reactions?.find((item) => item.emoji === emoji);

    data.toggleReactionMutation.mutate(
      {
        emoji,
        messageId: message.id,
        reactedByMe: reaction?.reactedByMe
      },
      {
        onError: (error) => setToast(error instanceof Error ? error.message : "Không cập nhật được reaction.")
      }
    );
  }

  function handleToggleMessagePin(message: ChatMessage, isPinned: boolean) {
    const mutation = isPinned ? data.unpinMessageMutation : data.pinMessageMutation;

    mutation.mutate(message.id, {
      onError: (error) => setToast(error instanceof Error ? error.message : "Không cập nhật được tin ghim.")
    });
  }

  function handleOpenThread(messageId: string) {
    setThreadMessageId(messageId);
    setIsDetailPanelOpen(true);
  }

  function handleLoadOlderMessages() {
    return data.loadOlderMessages().catch((error: unknown) =>
      setToast(error instanceof Error ? error.message : "Không tải được tin nhắn cũ.")
    );
  }

  return (
    <main
      className={`chat-app-shell chat-app-shell--zalo${activeRailItem === "messages" ? selectedChatChannel ? " chat-app-shell--mobile-chat" : " chat-app-shell--mobile-list" : " chat-app-shell--section chat-app-shell--mobile-section"}${activeRailItem !== "messages" && activeRailItem !== "contacts" ? " chat-app-shell--section-full" : ""}${activeRailItem === "messages" && data.selectedChannel && !data.canAccessSelectedChannel ? " chat-app-shell--no-detail" : ""}${activeRailItem === "messages" && isDetailPanelOpen ? " chat-app-shell--detail-open" : " chat-app-shell--detail-closed"}${isChannelPanelCollapsed ? " chat-app-shell--channel-collapsed" : ""}`}
      aria-label="Màn hình chat WebTui"
    >
      <div className="navigation-rail-slot" ref={accountMenuRef}>
        <NavigationRail
          activeId={activeRailItem === "messages" && messageSidebarTab === "channels" ? "channels" : activeRailItem}
          ariaLabel="Điều hướng chính"
          brandLogoAlt="WebTui Chat"
          brandLogoSrc="/brand/logo_webtui.png"
          isProfileMenuOpen={isProfileMenuOpen}
          items={visibleRailItems}
          onProfileClick={() => setIsProfileMenuOpen((current) => !current)}
          onSelect={(itemId) => handleRailSelect(itemId as RailItemId)}
          profile={{ name: currentUser.name, src: currentUser.avatarUrl, status: currentUser.status }}
        />
        {isProfileMenuOpen ? (
          <section aria-label="Tài khoản" className="account-menu" role="menu">
            <header className="account-menu__identity">
              <Avatar name={currentUser.name} size="lg" src={currentUser.avatarUrl} status={currentUser.status} />
              <span>
                <strong>{currentUser.name}</strong>
                <small>{currentUser.email || currentUser.username || "Tài khoản WebTui"}</small>
                <i><span /> Đang hoạt động</i>
              </span>
            </header>
            <div className="account-menu__actions">
              <button onClick={() => handleRailSelect("settings")} role="menuitem" type="button">
                <SettingsSolidIcon size={17} />
                <span><strong>Hồ sơ & cài đặt</strong></span>
              </button>
              <button onClick={toggleTheme} role="menuitem" type="button">
                {theme === "dark" ? <Sun size={17} /> : <Moon size={17} />}
                <span><strong>{theme === "dark" ? "Dùng giao diện sáng" : "Dùng giao diện tối"}</strong></span>
              </button>
              <button className="account-menu__logout" onClick={logout} role="menuitem" type="button">
                <LogOut size={17} />
                <span><strong>Đăng xuất</strong></span>
              </button>
            </div>
          </section>
        ) : null}
      </div>

      <section className="channel-panel" aria-label="Kênh và hội thoại">
        <header className={activeRailItem === "messages" ? "panel-heading panel-heading--messages" : "panel-heading"}>
          {activeRailItem === "messages" ? (
            <div className="message-panel-heading__primary">
              <div className="channel-search channel-search--heading">
                <Input
                  aria-label="Tìm kiếm hội thoại, kênh hoặc bot"
                  leftAddon={<Search size={16} />}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="Tìm kiếm..."
                  value={searchQuery}
                />
              </div>
            </div>
          ) : (
            <div>
              <p>{panelTitle}</p>
            </div>
          )}
          <div className="panel-heading__actions">
            <MobileFeatureMenu
              activeId={activeRailItem === "messages" && messageSidebarTab === "channels" ? "channels" : activeRailItem}
              items={visibleRailItems}
              onSelect={handleRailSelect}
            />
            <Tooltip label="Thông báo">
              <Button
                aria-label="Thông báo"
                className={isNotificationsOpen ? "notification-button notification-button--active" : "notification-button"}
                onClick={handleToggleNotifications}
                size="sm"
                variant="icon"
              >
                <Bell size={18} />
                {notificationBadgeCount ? <span>{notificationBadgeCount}</span> : null}
              </Button>
            </Tooltip>
            {activeRailItem === "messages" ? (
              <Tooltip label={canCreateChannel ? "Tạo kênh" : "Thiếu quyền tạo kênh"}>
                <Button
                  aria-label="Tạo kênh"
                  disabled={!data.workspaceId || !canCreateChannel || data.createChannelMutation.isPending}
                  onClick={() => setIsCreateChannelOpen((current) => !current)}
                  size="sm"
                  variant="icon"
                >
                  <Plus size={18} />
                </Button>
              </Tooltip>
            ) : null}
            <Tooltip
              className="channel-panel-toggle-wrap"
              label={isChannelPanelCollapsed ? "Mở rộng danh sách" : "Thu gọn danh sách"}
            >
              <Button
                aria-label={isChannelPanelCollapsed ? "Mở rộng danh sách" : "Thu gọn danh sách"}
                className="channel-panel-toggle"
                onClick={() => setIsChannelPanelCollapsed((current) => !current)}
                size="sm"
                variant="icon"
              >
                {isChannelPanelCollapsed ? <PanelLeftOpen size={19} /> : <PanelLeftClose size={19} />}
              </Button>
            </Tooltip>
          </div>
        </header>

        {isNotificationsOpen ? (
          <NotificationDropdown
            contactRequests={incomingContactRequests}
            isLoading={data.notificationsQuery.isLoading || data.contactRequestsQuery.isLoading}
            isMutatingContactRequest={data.acceptContactRequestMutation.isPending || data.rejectContactRequestMutation.isPending}
            isMarkingAllRead={data.markAllNotificationsReadMutation.isPending}
            notifications={data.notifications}
            onAcceptContactRequest={handleAcceptIncomingRequest}
            onMarkAllRead={handleMarkAllNotificationsRead}
            onOpenNotification={handleOpenNotification}
            onOpenContacts={() => {
              handleRailSelect("contacts");
              setIsNotificationsOpen(false);
            }}
            onRejectContactRequest={handleRejectIncomingRequest}
          />
        ) : null}

        {isCreateChannelOpen ? (
          <CreateChannelForm
            isPending={data.createChannelMutation.isPending}
            onCancel={() => setIsCreateChannelOpen(false)}
            onSubmit={handleCreateChannel}
          />
        ) : null}

        {activeRailItem === "messages" && isChannelPanelCollapsed ? (
          <div className="collapsed-channel-list" aria-label="Danh sách hội thoại thu gọn">
            <div className="collapsed-channel-list__tabs" role="tablist" aria-label="Loại danh sách">
              <button
                aria-label="Hội thoại"
                aria-selected={messageSidebarTab === "conversations"}
                className={messageSidebarTab === "conversations" ? "collapsed-channel-list__tab collapsed-channel-list__tab--active" : "collapsed-channel-list__tab"}
                onClick={() => handleMessageSidebarTabChange("conversations")}
                role="tab"
                title="Hội thoại"
                type="button"
              >
                <ConversationSolidIcon size={20} />
                <span>{sidebarConversationUnreadCount || data.directConversations.length}</span>
              </button>
              <button
                aria-label="Kênh và bot"
                aria-selected={messageSidebarTab === "channels"}
                className={messageSidebarTab === "channels" ? "collapsed-channel-list__tab collapsed-channel-list__tab--active" : "collapsed-channel-list__tab"}
                onClick={() => handleMessageSidebarTabChange("channels")}
                role="tab"
                title="Kênh và bot"
                type="button"
              >
                <GroupSolidIcon size={20} />
                <span>{sidebarChannelUnreadCount || data.channels.length}</span>
              </button>
            </div>
            <div className="collapsed-channel-list__items">
              {messageSidebarTab === "conversations"
                ? filteredConversations.map((item) => {
                    const unreadCount = effectiveUnreadCount(item.id, item.unreadCount);
                    return (
                      <button
                        aria-label={`Mở hội thoại với ${item.user.name}`}
                        className={item.id === data.selectedChannelId ? "collapsed-channel-list__item collapsed-channel-list__item--active" : "collapsed-channel-list__item"}
                        key={item.id}
                        onClick={() => handleChannelSelect(item.id)}
                        title={item.user.name}
                        type="button"
                      >
                        <Avatar name={item.user.name} size="md" src={item.user.avatarUrl} status={item.user.status} />
                        {unreadCount ? <UnreadBadge count={unreadCount} /> : null}
                      </button>
                    );
                  })
                : sidebarChannels.map((channel) => {
                    const unreadCount = effectiveUnreadCount(channel.id, channel.unreadCount);
                    return (
                      <button
                        aria-label={`Mở kênh ${channel.name}`}
                        className={channel.id === data.selectedChannelId ? "collapsed-channel-list__item collapsed-channel-list__item--active" : "collapsed-channel-list__item"}
                        key={channel.id}
                        onClick={() => handleChannelSelect(channel.id)}
                        title={channel.name}
                        type="button"
                      >
                        <span className={`channel-hash channel-hash--${channel.tone}`} style={channelHashStyle(channel)}>
                          <Hash size={19} />
                        </span>
                        {unreadCount ? <UnreadBadge count={unreadCount} /> : null}
                      </button>
                    );
                  })}
              {messageSidebarTab === "channels" && data.can("bot.manage") ? (
                <button
                  aria-label="Quản lý bot"
                  className="collapsed-channel-list__item collapsed-channel-list__item--bot"
                  onClick={() => handleRailSelect("bots")}
                  title="Quản lý bot"
                  type="button"
                >
                  <Bot size={22} />
                </button>
              ) : null}
            </div>
          </div>
        ) : null}

        {activeRailItem === "messages" ? (
          isChannelPanelCollapsed ? null : (
            <>
            <SegmentedControl
              aria-label="Bộ lọc hội thoại"
              className="channel-filter-tabs"
              onValueChange={setChannelFilter}
              options={channelFilters}
              value={channelFilter}
            />

            {messageSidebarTab === "conversations" ? (
              <div className="message-sidebar-tab-content" id="message-sidebar-conversations" role="tabpanel">
                <div className="list-section conversations">
                  {data.directConversationsQuery.isLoading ? (
                    <PanelSkeleton />
                  ) : filteredConversations.length ? (
                    filteredConversations.map((item) => {
                      const unreadCount = effectiveUnreadCount(item.id, item.unreadCount);
                      return (
                        <button
                          className={`conversation-row${item.id === data.selectedChannelId ? " conversation-row--active" : ""}${unreadCount ? " conversation-row--unread" : ""}`}
                          key={item.id}
                          onClick={() => handleChannelSelect(item.id)}
                          title={isChannelPanelCollapsed ? item.user.name : undefined}
                          type="button"
                        >
                          <Avatar name={item.user.name} size="md" src={item.user.avatarUrl} status={item.user.status} />
                          <span className="conversation-row__body">
                            <strong>{item.user.name}</strong>
                            <small>{item.lastMessage}</small>
                          </span>
                          <span className="conversation-row__meta">
                            <time>{item.relativeTime}</time>
                            <UnreadBadge count={unreadCount} />
                            <Tooltip label={isFavoriteChat(item.id) ? "Bỏ yêu thích" : "Yêu thích"}>
                              <span
                                aria-label={isFavoriteChat(item.id) ? "Bỏ yêu thích" : "Yêu thích"}
                                className={isFavoriteChat(item.id) ? "pin-action pin-action--active" : "pin-action"}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  handleToggleFavorite(item.id);
                                }}
                                onKeyDown={(event) => {
                                  if (event.key === "Enter" || event.key === " ") {
                                    event.preventDefault();
                                    event.stopPropagation();
                                    handleToggleFavorite(item.id);
                                  }
                                }}
                                role="button"
                                tabIndex={0}
                              >
                                <Star size={15} />
                              </span>
                            </Tooltip>
                          </span>
                        </button>
                      );
                    })
                  ) : (
                    <div className="conversation-empty">
                      <EmptyState
                        description={channelFilter === "all" ? "Tìm bạn bè, gửi lời mời và bắt đầu nhắn tin riêng như Zalo." : "Không có hội thoại nào phù hợp với bộ lọc hiện tại."}
                        title={channelFilter === "unread" ? "Không có tin chưa đọc" : channelFilter === "favorite" ? "Chưa có hội thoại yêu thích" : "Chưa có hội thoại"}
                      />
                      {channelFilter === "all" ? (
                        <Button onClick={() => handleRailSelect("contacts")} size="sm" variant="secondary">
                          <Users size={15} />
                          Tìm bạn bè
                        </Button>
                      ) : null}
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div className="message-sidebar-tab-content" id="message-sidebar-channels" role="tabpanel">
                <div className="list-section channels-section">
                  {data.workspacesQuery.isLoading || data.channelsQuery.isLoading ? (
                    <PanelSkeleton />
                  ) : sidebarChannels.length ? (
                    sidebarChannels.map((channel) => {
                      const unreadCount = effectiveUnreadCount(channel.id, channel.unreadCount);
                      return (
                        <button
                          className={`channel-row${channel.id === data.selectedChannelId ? " channel-row--active" : ""}${unreadCount ? " channel-row--unread" : ""}`}
                          key={channel.id}
                          onClick={() => handleChannelSelect(channel.id)}
                          title={isChannelPanelCollapsed ? channel.name : undefined}
                          type="button"
                        >
                          <span className={`channel-hash channel-hash--${channel.tone}`} style={channelHashStyle(channel)}>#</span>
                          <span className="channel-row__body">
                            <strong>{channel.name}</strong>
                            <small>{channel.description}</small>
                          </span>
                          <span className="channel-row__meta">
                            <time>{channel.relativeTime}</time>
                            <UnreadBadge count={unreadCount} />
                            <Tooltip label={isFavoriteChat(channel.id, channel.isFavorite) ? "Bỏ yêu thích" : "Yêu thích"}>
                              <span
                                aria-label={isFavoriteChat(channel.id, channel.isFavorite) ? "Bỏ yêu thích" : "Yêu thích"}
                                className={isFavoriteChat(channel.id, channel.isFavorite) ? "pin-action pin-action--active" : "pin-action"}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  handleToggleFavorite(channel.id);
                                }}
                                onKeyDown={(event) => {
                                  if (event.key === "Enter" || event.key === " ") {
                                    event.preventDefault();
                                    event.stopPropagation();
                                    handleToggleFavorite(channel.id);
                                  }
                                }}
                                role="button"
                                tabIndex={0}
                              >
                                <Star size={15} />
                              </span>
                            </Tooltip>
                          </span>
                        </button>
                      );
                    })
                  ) : (
                    <EmptyState
                      description={channelFilter === "all" ? "Kênh dùng cho nhóm, bot và thông báo chung." : "Không có kênh nào phù hợp với bộ lọc hiện tại."}
                      title={channelFilter === "unread" ? "Không có kênh chưa đọc" : channelFilter === "favorite" ? "Chưa có kênh yêu thích" : "Chưa có kênh"}
                    />
                  )}
                </div>

                {channelFilter === "all" && data.can("bot.manage") ? (
                  <div className="list-section sidebar-bots-section">
                    <span className="section-label">Bot workspace</span>
                    {sidebarBotsQuery.isLoading ? (
                      <PanelSkeleton />
                    ) : sidebarBots.length ? (
                      sidebarBots.map((bot) => (
                        <button className="sidebar-bot-row" key={bot.id} onClick={() => handleRailSelect("bots")} title={isChannelPanelCollapsed ? bot.name : undefined} type="button">
                          <span className="sidebar-bot-row__avatar">
                            {bot.avatar_url ? <img alt="" src={bot.avatar_url} /> : <Bot size={20} />}
                            <i />
                          </span>
                          <span className="sidebar-bot-row__body">
                            <strong>{bot.name}</strong>
                            <small>{bot.description || `@${bot.slug}`}</small>
                          </span>
                          <span className="sidebar-bot-row__status">{bot.status === "active" ? "Bật" : "Tắt"}</span>
                        </button>
                      ))
                    ) : (
                      <button className="sidebar-bot-row sidebar-bot-row--module" onClick={() => handleRailSelect("bots")} title={isChannelPanelCollapsed ? "Quản lý bot" : undefined} type="button">
                        <span className="sidebar-bot-row__avatar"><Bot size={20} /></span>
                        <span className="sidebar-bot-row__body">
                          <strong>Quản lý bot</strong>
                          <small>Tạo bot đầu tiên cho workspace</small>
                        </span>
                        <span className="sidebar-bot-row__arrow">›</span>
                      </button>
                    )}
                  </div>
                ) : null}
              </div>
            )}
            </>
          )
        ) : (
          <SidebarContextPanel
            activeRailItem={activeRailItem}
            contacts={contactResults}
            isPending={
              data.createDirectConversationMutation.isPending ||
              data.sendContactRequestMutation.isPending ||
              data.acceptContactRequestMutation.isPending
            }
            isSearchingContacts={data.searchUsersQuery.isFetching}
            onContactAction={handleContactPrimaryAction}
            onSearchChange={setFriendSearchQuery}
            query={friendSearchQuery}
          />
        )}
      </section>

      {isNotificationsOpen ? (
        <div className="mobile-notification-layer">
          <NotificationDropdown
            contactRequests={incomingContactRequests}
            isLoading={data.notificationsQuery.isLoading || data.contactRequestsQuery.isLoading}
            isMutatingContactRequest={data.acceptContactRequestMutation.isPending || data.rejectContactRequestMutation.isPending}
            isMarkingAllRead={data.markAllNotificationsReadMutation.isPending}
            notifications={data.notifications}
            onAcceptContactRequest={handleAcceptIncomingRequest}
            onMarkAllRead={handleMarkAllNotificationsRead}
            onOpenNotification={handleOpenNotification}
            onOpenContacts={() => {
              handleRailSelect("contacts");
              setIsNotificationsOpen(false);
            }}
            onRejectContactRequest={handleRejectIncomingRequest}
          />
        </div>
      ) : null}

      <section
        className="chat-main"
        aria-label={
          activeRailItem === "messages"
            ? data.selectedChannelWithMessages
              ? `Nội dung kênh ${data.selectedChannelWithMessages.name}`
              : "Nội dung kênh"
            : selectedRailLabel
        }
      >
        {activeRailItem !== "messages" ? (
          <header className="mobile-section-topbar">
            <strong>{selectedRailLabel}</strong>
            <div className="mobile-section-topbar__actions">
              {activeRailItem === "tickets" ? (
                <Button
                  aria-controls="ticket-create-form"
                  aria-expanded={isTicketCreateOpen}
                  aria-label={isTicketCreateOpen ? "Đóng biểu mẫu tạo ticket" : "Tạo ticket"}
                  className="mobile-ticket-create-button"
                  onClick={() => setIsTicketCreateOpen((current) => !current)}
                  size="sm"
                >
                  {isTicketCreateOpen ? <X size={17} /> : <Plus size={17} />}
                  <span>{isTicketCreateOpen ? "Đóng" : "Tạo ticket"}</span>
                </Button>
              ) : null}
              <MobileNotificationButton count={notificationBadgeCount} isOpen={isNotificationsOpen} onToggle={handleToggleNotifications} />
              <MobileFeatureMenu activeId={activeRailItem} items={visibleRailItems} onSelect={handleRailSelect} />
            </div>
          </header>
        ) : null}
        {activeRailItem !== "messages" ? (
          <WorkspaceSectionPage
            activeRailItem={activeRailItem}
            canManageBots={data.can("bot.manage")}
            canManageCronjobs={data.can("cronjob.manage")}
            canManageTickets={data.can("ticket.manage")}
            canUseOrderBot={data.can("order.view")}
            canUseOrderBilling={data.can("order.billing")}
            canManageWebhooks={data.can("webhook.manage")}
            canOpenAdmin={data.can("admin.view")}
            channels={data.channels.filter((channel) => channel.type !== "direct")}
            contacts={contactResults}
            currentUser={currentUser}
            desktopVersionStatus={desktopVersionStatus}
            departments={data.departments}
            files={data.files}
            friendSearchQuery={friendSearchQuery}
            isCreatingDirectConversation={
              data.createDirectConversationMutation.isPending ||
              data.sendContactRequestMutation.isPending ||
              data.acceptContactRequestMutation.isPending ||
              data.rejectContactRequestMutation.isPending ||
              data.cancelContactRequestMutation.isPending
            }
            isLoadingChannels={data.channelsQuery.isLoading}
            isLoadingContacts={
              data.contactsQuery.isLoading ||
              data.contactRequestsQuery.isLoading ||
              data.membersQuery.isLoading ||
              data.searchUsersQuery.isFetching
            }
            isLoadingFiles={data.filesQuery.isLoading}
            isLoadingDepartments={data.permissionsQuery.isLoading || data.departmentsQuery.isLoading}
            isTicketCreateOpen={isTicketCreateOpen}
            isMutatingChannelMembership={
              data.requestChannelJoinMutation.isPending ||
              data.inviteChannelMemberMutation.isPending ||
              data.approveChannelJoinMutation.isPending ||
              data.rejectChannelJoinMutation.isPending
            }
            joinRequestsByChannelId={data.joinRequestsByChannelId}
            isAutoStartEnabled={isAutoStartEnabled}
            isAutoStartLoading={isAutoStartLoading}
            isDesktopUpdateInstalling={isDesktopUpdateInstalling}
            onChannelSelect={handleChannelSelect}
            onAutoStartChange={(enabled) => void handleAutoStartChange(enabled)}
            onDesktopUpdateInstall={() => void handleDesktopUpdateInstall()}
            onApproveChannelJoin={(channelId, userId) =>
              data.approveChannelJoinMutation.mutate({ channelId, userId }, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không phê duyệt được yêu cầu."),
                onSuccess: () => setToast("Đã phê duyệt thành viên vào kênh.")
              })
            }
            onDownloadFile={handleDownload}
            onInviteChannelMember={(channelId, userId) =>
              data.inviteChannelMemberMutation.mutate({ channelId, userId }, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không mời được thành viên."),
                onSuccess: () => setToast("Đã thêm thành viên vào kênh.")
              })
            }
            onRejectChannelJoin={(channelId, userId) =>
              data.rejectChannelJoinMutation.mutate({ channelId, userId }, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không từ chối được yêu cầu."),
                onSuccess: () => setToast("Đã từ chối yêu cầu tham gia.")
              })
            }
            onRequestChannelJoin={(channelId) =>
              data.requestChannelJoinMutation.mutate(channelId, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không gửi được yêu cầu tham gia."),
                onSuccess: () => setToast("Đã gửi yêu cầu tham gia kênh.")
              })
            }
            onCreateDepartment={(input) =>
              data.createDepartmentMutation.mutate(input, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không tạo được phòng ban."),
                onSuccess: () => setToast("Đã tạo phòng ban mới.")
              })
            }
            onFriendSearchChange={setFriendSearchQuery}
            onProfileSubmit={(input) =>
              data.updateProfileMutation.mutate(input, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không cập nhật được hồ sơ."),
                onSuccess: () => setToast("Đã cập nhật hồ sơ.")
              })
            }
            onSecondaryContactAction={handleContactSecondaryAction}
            notificationPreferences={notificationPreferences}
            onNotificationPreferencesChange={handleNotificationPreferencesChange}
            onStartConversation={handleContactPrimaryAction}
            onThemeToggle={toggleTheme}
            onTicketCreateOpenChange={setIsTicketCreateOpen}
            theme={theme}
            canManageDepartments={data.can("workspace.manage")}
            isCreatingDepartment={data.createDepartmentMutation.isPending}
            isUpdatingProfile={data.updateProfileMutation.isPending}
            workspaceId={data.workspaceId}
            workspaceMembers={displayWorkspaceMembers}
          />
        ) : data.workspacesQuery.isError ? (
          <ErrorState
            action={
              <Button onClick={() => void data.workspacesQuery.refetch()} size="sm" variant="secondary">
                Thử tải lại
              </Button>
            }
            description="Không thể kết nối dữ liệu tài khoản. Hãy kiểm tra mạng và thử lại."
            title="Không tải được dữ liệu chat"
          />
        ) : selectedChatChannel && !data.canAccessSelectedChannel ? (
          <ChannelAccessView
            channel={selectedChatChannel}
            isPending={data.requestChannelJoinMutation.isPending}
            mobileFeatureMenu={
              <div className="mobile-chat-utilities">
                <MobileNotificationButton count={notificationBadgeCount} isOpen={isNotificationsOpen} onToggle={handleToggleNotifications} />
              </div>
            }
            onBack={handleMobileBackToList}
            onRequestJoin={() =>
              data.requestChannelJoinMutation.mutate(selectedChatChannel.id, {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không gửi được yêu cầu tham gia."),
                onSuccess: () => setToast("Đã gửi yêu cầu. Vui lòng chờ chủ kênh phê duyệt.")
              })
            }
          />
        ) : selectedChatChannel ? (
          <>
            <ChatHeader
              callStatus={callControls.callState.status}
              channel={selectedChatChannel}
              isDetailPanelOpen={isDetailPanelOpen}
              isFavorite={isFavoriteChat(selectedChatChannel.id, selectedChatChannel.isFavorite)}
              isMembersLoading={selectedChannelMembersQuery.isLoading}
              isSearchOpen={isMessageSearchOpen}
              members={displayChannelMembers}
              mobileFeatureMenu={
                <div className="mobile-chat-utilities">
                  <MobileNotificationButton count={notificationBadgeCount} isOpen={isNotificationsOpen} onToggle={handleToggleNotifications} />
                </div>
              }
              onBack={handleMobileBackToList}
              onMarkUnread={() => handleMarkUnread(selectedChatChannel.id)}
              onStartAudioCall={selectedChatChannel.type === "direct" ? () => void callControls.startCall("audio") : undefined}
              onStartVideoCall={selectedChatChannel.type === "direct" ? () => void callControls.startCall("video") : undefined}
              onToggleDetailPanel={() => setIsDetailPanelOpen((current) => !current)}
              onToggleFavorite={() => handleToggleFavorite(selectedChatChannel.id)}
              onToggleSearch={handleToggleMessageSearch}
            />
            {isMessageSearchOpen ? (
              <div className="message-toolbar">
                <div className="message-toolbar__search">
                  <Input
                    aria-label="Tìm tin nhắn"
                    autoFocus
                    leftAddon={<Search size={17} />}
                    onChange={(event) => setMessageSearchQuery(event.target.value)}
                    placeholder="Tìm tin nhắn..."
                    value={messageSearchQuery}
                  />
                  <div className="message-search-filters">
                    <select aria-label="Lọc theo kênh" onChange={(event) => setMessageSearchChannelId(event.target.value)} value={messageSearchChannelId}>
                      <option value="">Tất cả kênh</option>
                      {chatTargets.map((target) => <option key={target.id} value={target.id}>{target.name}</option>)}
                    </select>
                    <select aria-label="Lọc theo người gửi" onChange={(event) => setMessageSearchSenderId(event.target.value)} value={messageSearchSenderId}>
                      <option value="">Tất cả người gửi</option>
                      {displayWorkspaceMembers.map((member) => <option key={member.user_id} value={member.user_id}>{member.display_name || member.username || member.email}</option>)}
                    </select>
                    <select aria-label="Lọc theo loại nội dung" onChange={(event) => setMessageSearchKind(event.target.value)} value={messageSearchKind}>
                      <option value="">Mọi nội dung</option>
                      <option value="text">Văn bản</option>
                      <option value="file">File</option>
                      <option value="system">Hệ thống</option>
                      <option value="bot">Bot</option>
                      <option value="event">Sự kiện</option>
                    </select>
                    <label>Từ ngày<input onChange={(event) => setMessageSearchDateFrom(event.target.value)} type="date" value={messageSearchDateFrom} /></label>
                    <label>Đến ngày<input onChange={(event) => setMessageSearchDateTo(event.target.value)} type="date" value={messageSearchDateTo} /></label>
                  </div>
                </div>
                <Tooltip label="Đóng tìm kiếm">
                  <Button aria-label="Đóng tìm kiếm" onClick={handleCloseMessageSearch} type="button" variant="icon">
                    <X size={18} />
                  </Button>
                </Tooltip>
              </div>
            ) : null}
            {data.offlineReadMode || data.queuedOutboxCount ? (
              <div className="offline-read-banner" role="status">
                <span>{data.offlineReadMode ? "Đang hiển thị dữ liệu đã lưu offline." : "Kết nối đã sẵn sàng."}</span>
                {data.queuedOutboxCount ? <strong>{data.queuedOutboxCount} tin đang chờ gửi</strong> : null}
                {data.queuedOutboxCount ? (
                  <button onClick={() => void data.flushOutbox()} type="button">Gửi lại</button>
                ) : null}
              </div>
            ) : null}
            {data.messagesQuery.isError && !selectedChatChannel.messages.length ? (
              <ErrorState
                action={
                  <Button onClick={() => void data.messagesQuery.refetch()} size="sm" variant="secondary">
                    Thử tải lại
                  </Button>
                }
                className="chat-load-error"
                description="Kết nối có thể bị gián đoạn hoặc phiên truy cập vừa hết hạn."
                title="Không tải được tin nhắn"
              />
            ) : data.messagesQuery.isLoading ? (
              <TimelineSkeleton />
            ) : (
              <MessageTimeline
                currentUserId={currentUser.id}
                editingBody={editingBody}
                editingMessageId={editingMessageId}
                focusedMessageId={focusedMessageId}
                hasOlderMessages={data.hasOlderMessages}
                isEditingPending={data.editMessageMutation.isPending}
                isLoadingOlderMessages={data.isLoadingOlderMessages}
                messages={selectedChatChannel.messages}
                onCancelEdit={() => {
                  setEditingBody("");
                  setEditingMessageId(null);
                }}
                onChangeEditingBody={setEditingBody}
                onDeleteMessage={handleDeleteMessage}
                onDownloadAttachment={handleDownloadAttachment}
                onForwardMessage={setForwardingMessageId}
                onPreviewAttachment={handlePreviewAttachment}
                onResolveAttachment={data.downloadAttachment}
                onLoadOlderMessages={handleLoadOlderMessages}
                onOpenThread={handleOpenThread}
                onReplyMessage={handleReplyToMessage}
                onFocusedMessageSettled={handleFocusedMessageSettled}
                onRetryCall={(mode) => void callControls.startCall(mode)}
                onSearchResultSelect={(message) => {
                  if (message.rawChannelId && message.rawChannelId !== data.selectedChannelId) {
                    data.setSelectedChannelId(message.rawChannelId);
                  }
                  setThreadMessageId(null);
                  setFocusedMessageId(message.id);
                  handleCloseMessageSearch();
                }}
                onStartEdit={handleStartEdit}
                onSubmitEdit={handleSubmitEdit}
                onTogglePin={handleToggleMessagePin}
                onToggleReaction={handleToggleReaction}
                pinnedMessageIds={pinnedMessageIds}
                readMembers={displayChannelMembers}
                searchQuery={activeMessageSearchQuery}
                searchResults={data.messageSearchResults}
                showAuthorName={selectedChatChannel.type !== "direct"}
                timelineId={selectedChatChannel.id}
                workspaceMembers={displayWorkspaceMembers}
              />
            )}
            {!canSendMessage ? (
              <div className="permission-note">Tài khoản hiện tại chưa có quyền gửi tin nhắn trong cuộc trò chuyện này.</div>
            ) : null}
            <div
              className={isComposerDragActive ? "composer-wrap composer-wrap--drag-active" : "composer-wrap"}
              onDragLeave={handleComposerDragLeave}
              onDragOver={handleComposerDragOver}
              onDrop={handleComposerDrop}
            >
              {isComposerDragActive ? (
                <div className="composer-drop-hint" aria-hidden="true">
                  Thả file để đính kèm
                </div>
              ) : null}
              {uploadQueue.items.length ? (
                <UploadQueue
                  disabled={data.sendMessageMutation.isPending}
                  items={uploadQueue.items}
                  onRemove={uploadQueue.remove}
                  onRetry={uploadQueue.retry}
                />
              ) : null}
              {remoteTypingLabel ? <TypingDots label={remoteTypingLabel} /> : null}
              {replyingTo ? (
                <ReplyComposerPreview onCancel={() => setReplyingTo(null)} preview={replyingTo} />
              ) : null}
              <form className="composer" onSubmit={handleSendMessage}>
                {isRecording ? (
                  <div className="recording-status" role="status">
                    <span className="recording-status__dot" />
                    <strong>{isRecordingPaused ? "Đã tạm dừng" : `Đang ghi ${formatRecordingTime(recordingSeconds)}`}</strong>
                    <button
                      aria-label={isRecordingPaused ? "Tiếp tục ghi âm" : "Tạm dừng ghi âm"}
                      onClick={isRecordingPaused ? handleResumeRecording : handlePauseRecording}
                      type="button"
                    >
                      {isRecordingPaused ? <Play size={14} /> : <Pause size={14} />}
                    </button>
                    <button aria-label="Hủy ghi âm" onClick={handleCancelRecording} type="button">
                      <X size={14} />
                    </button>
                  </div>
                ) : null}
                <div className="composer-input-group">
                  {activeMentionToken && mentionSuggestions.length ? (
                    <div className="mention-suggestions" role="listbox" aria-label="Gợi ý nhắc tên">
                      {mentionSuggestions.map((member) => (
                        <button
                          aria-label={`Nhắc tên ${mentionMemberName(member)}`}
                          aria-selected="false"
                          key={member.user_id}
                          onClick={() => handleInsertMention(member)}
                          onMouseDown={(event) => event.preventDefault()}
                          role="option"
                          type="button"
                        >
                          <Avatar name={mentionMemberName(member)} size="sm" src={member.avatar_url ?? undefined} />
                          <span>
                            <strong>{mentionMemberName(member)}</strong>
                            <small>{member.username ? `@${member.username}` : member.email ?? "Thành viên"}</small>
                          </span>
                        </button>
                      ))}
                    </div>
                  ) : null}
                  <input
                    aria-label="Nhập tin nhắn"
                    disabled={data.sendMessageMutation.isPending || !canSendMessage}
                    onChange={(event) => handleDraftChange(event.target.value)}
                    onPaste={handleComposerPaste}
                    placeholder={composerPlaceholder}
                    ref={composerInputRef}
                    value={draft}
                  />
                  <div className="composer-inline-actions">
                    <Tooltip className="composer-leading-tooltip" label="Biểu cảm">
                      <span className="composer-action-wrap">
                        <Button
                          aria-label="Biểu cảm"
                          onClick={() => {
                            setIsComposerMoreOpen(false);
                            setIsEmojiPickerOpen((current) => !current);
                          }}
                          type="button"
                          variant="icon"
                        >
                          <Smile size={20} />
                        </Button>
                        {isEmojiPickerOpen ? <EmojiPicker onSelect={handleEmojiSelect} /> : null}
                      </span>
                    </Tooltip>
                    <span className="composer-desktop-attachment">
                      <Tooltip label="Gửi hình ảnh">
                        <Button
                          aria-label="Gửi hình ảnh"
                          disabled={data.sendMessageMutation.isPending || !canUploadFile}
                          onClick={() => void handlePickUploadFiles("image")}
                          type="button"
                          variant="icon"
                        >
                          <ImageIcon size={20} />
                        </Button>
                      </Tooltip>
                    </span>
                    <span className="composer-desktop-attachment">
                      <Tooltip label="Đính kèm file">
                        <Button
                          aria-label="Đính kèm file"
                          disabled={data.sendMessageMutation.isPending || !canUploadFile}
                          onClick={() => void handlePickUploadFiles("file")}
                          type="button"
                          variant="icon"
                        >
                          <Paperclip size={20} />
                        </Button>
                      </Tooltip>
                    </span>
                    <Tooltip label={isRecording ? "Dừng ghi âm" : "Gửi tin nhắn thoại"}>
                      <Button
                        aria-label={isRecording ? "Dừng ghi âm" : "Gửi tin nhắn thoại"}
                        className={isRecording ? "record-button record-button--active" : "record-button"}
                        disabled={data.sendMessageMutation.isPending || !canUploadFile}
                        onClick={handleToggleRecording}
                        type="button"
                        variant="icon"
                      >
                        {isRecording ? <StopCircle size={20} /> : <Mic size={20} />}
                      </Button>
                    </Tooltip>
                    <span className="composer-more-wrap">
                      <Tooltip label="Thêm nội dung">
                        <Button
                          aria-expanded={isComposerMoreOpen}
                          aria-label="Thêm nội dung"
                          onClick={() => {
                            setIsEmojiPickerOpen(false);
                            setIsComposerMoreOpen((current) => !current);
                          }}
                          type="button"
                          variant="icon"
                        >
                          <MoreVertical size={20} />
                        </Button>
                      </Tooltip>
                      {isComposerMoreOpen ? (
                        <div aria-label="Tùy chọn đính kèm" className="composer-more-menu" role="menu">
                          <button
                            disabled={data.sendMessageMutation.isPending || !canUploadFile}
                            onClick={() => {
                              setIsComposerMoreOpen(false);
                              void handlePickUploadFiles("image");
                            }}
                            role="menuitem"
                            type="button"
                          >
                            <ImageIcon size={19} />
                            <span>Gửi hình ảnh</span>
                          </button>
                          <button
                            disabled={data.sendMessageMutation.isPending || !canUploadFile}
                            onClick={() => {
                              setIsComposerMoreOpen(false);
                              void handlePickUploadFiles("file");
                            }}
                            role="menuitem"
                            type="button"
                          >
                            <Paperclip size={19} />
                            <span>Đính kèm file</span>
                          </button>
                        </div>
                      ) : null}
                    </span>
                  </div>
                  <Button
                    aria-label={hasComposerContent ? "Gửi tin nhắn" : "Gửi lượt thích"}
                    className={hasComposerContent ? "composer-submit-icon-button composer-send-icon-button" : "composer-submit-icon-button composer-like-button"}
                    disabled={data.sendMessageMutation.isPending || !canUseComposer}
                    onClick={hasComposerContent ? undefined : handleSendLike}
                    type={hasComposerContent ? "submit" : "button"}
                    variant="icon"
                  >
                    {hasComposerContent ? <Send aria-hidden="true" size={22} /> : <ComposerLikeIcon />}
                  </Button>
                </div>
              </form>
            </div>
          </>
        ) : (
          <div className="chat-empty-state">
            <span>
              <MessageCircle size={36} />
            </span>
            <h2>Chọn một cuộc trò chuyện</h2>
            <p>Chọn hội thoại, kênh hoặc tìm bạn bè để bắt đầu nhắn tin.</p>
          </div>
        )}
      </section>

      {activeRailItem === "messages" && isDetailPanelOpen && (!data.selectedChannel || data.canAccessSelectedChannel) ? (
        <RightDetailPanel
          activeTab={detailTab}
          channelMembers={displayChannelMembers}
          files={selectedChatFiles}
          isDirectChat={selectedChatChannel?.type === "direct"}
          isLoading={data.messagesQuery.isLoading || data.pinnedMessagesQuery.isLoading || data.channelMediaQuery.isLoading}
          isSendingThread={data.sendThreadMessageMutation.isPending}
          isThreadLoading={data.threadQuery.isLoading}
          mediaItems={data.mediaItems}
          onClose={() => setIsDetailPanelOpen(false)}
          onCloseThread={() => setThreadMessageId(null)}
          onFileSelect={handleDownload}
          onMediaSelect={handlePreviewMediaItem}
          onResolveMedia={data.downloadAttachment}
          onSendThread={async (body) => {
            try {
              await data.sendThreadMessageMutation.mutateAsync(body);
              setToast("Đã gửi trả lời trong luồng.");
              return true;
            } catch (error) {
              setToast(error instanceof Error ? error.message : "Không gửi được trả lời.");
              return false;
            }
          }}
          onTabChange={setDetailTab}
          pinnedMessages={pinnedMessages}
          threadMessages={data.threadMessages}
          threadMessageId={threadMessageId}
          workspaceMembers={displayWorkspaceMembers}
        />
      ) : null}

      <CallPanel
        callState={callControls.callState}
        hasMediaSession={callControls.hasMediaSession}
        onAccept={() => void callControls.acceptCall()}
        onEnd={callControls.endCall}
        onReject={callControls.rejectCall}
        mediaContainerRef={callControls.mediaContainerRef}
      />

      {forwardingMessageId ? (
        <ForwardMessageDialog
          channels={forwardTargets}
          isPending={data.forwardMessageMutation.isPending}
          onCancel={() => setForwardingMessageId(null)}
          onSubmit={(targetChannelId) =>
            data.forwardMessageMutation.mutate(
              { messageId: forwardingMessageId, targetChannelId },
              {
                onError: (error) => setToast(error instanceof Error ? error.message : "Không chuyển tiếp được tin nhắn."),
                onSuccess: () => {
                  setForwardingMessageId(null);
                  setToast("Đã chuyển tiếp tin nhắn.");
                }
              }
            )
          }
        />
      ) : null}

      {previewedImage ? (
        <ImagePreviewDialog
          attachment={previewedImage.attachment}
          onClose={() => setPreviewedImage(null)}
          onDownload={() => handleDownloadAttachment(previewedImage.attachment)}
          source={previewedImage.source}
        />
      ) : null}

      {messageNotice ? (
        <MessageNotificationToast
          notification={messageNotice}
          onClose={() => setMessageNotice(null)}
          onOpen={(notification) => {
            setMessageNotice(null);
            handleOpenNotification(notification);
          }}
        />
      ) : null}

      {toast ? (
        <div className="toast-stack">
          <Toast role={toast.tone === "danger" ? "alert" : "status"} tone={toast.tone}>{toast.message}</Toast>
        </div>
      ) : null}
    </main>
  );
}

function inferToastTone(message: string): ChatToastNotice["tone"] {
  const normalized = message.toLocaleLowerCase("vi");
  if (
    normalized.includes("không ") ||
    normalized.includes("chưa có quyền") ||
    normalized.includes("thiếu quyền") ||
    normalized.includes("thất bại") ||
    normalized.includes("failed") ||
    normalized.includes("lỗi")
  ) {
    return "danger";
  }
  if (normalized.includes("đang ") || normalized.includes("offline") || normalized.includes("chờ ")) {
    return "info";
  }
  return "success";
}

function SidebarContextPanel({
  activeRailItem,
  contacts,
  isPending,
  isSearchingContacts,
  onContactAction,
  onSearchChange,
  query
}: {
  activeRailItem: RailItemId;
  contacts: ContactResult[];
  isPending: boolean;
  isSearchingContacts: boolean;
  onContactAction: (contact: ContactResult) => void;
  onSearchChange: (value: string) => void;
  query: string;
}) {
  const isContactsPanel = activeRailItem === "contacts";
  const isSearching = query.trim().length >= 2;

  return (
    <div className="sidebar-context-panel">
      {isContactsPanel ? (
        <section aria-label="Tìm và kết bạn" className="sidebar-contact-finder">
          <Input
            aria-label="Tìm người theo tên, email hoặc số điện thoại"
            leftAddon={<Search size={16} />}
            onChange={(event) => onSearchChange(event.target.value)}
            placeholder="Tên, email hoặc số điện thoại"
            value={query}
          />
          {!isSearching ? (
            <p>Nhập ít nhất 2 ký tự để tìm và gửi lời mời kết bạn.</p>
          ) : isSearchingContacts ? (
            <PanelSkeleton />
          ) : contacts.length ? (
            <div className="sidebar-contact-finder__results">
              {contacts.slice(0, 6).map((contact) => (
                <article key={contact.userId}>
                  <Avatar name={contact.name} size="sm" src={contact.avatarUrl ?? undefined} />
                  <span>
                    <strong>{contact.name}</strong>
                    <small>{contact.phoneNumber || contact.email || contact.username || "Chưa có thông tin liên hệ"}</small>
                  </span>
                  <Button
                    disabled={isPending || (contact.contactStatus === "pending" && contact.contactDirection === "outgoing")}
                    onClick={() => onContactAction(contact)}
                    size="sm"
                    variant={contact.contactStatus === "pending" && contact.contactDirection === "outgoing" ? "secondary" : "primary"}
                  >
                    {contactActionLabel(contact)}
                  </Button>
                </article>
              ))}
            </div>
          ) : (
            <p>Không tìm thấy người phù hợp.</p>
          )}
        </section>
      ) : null}
    </div>
  );
}

function WorkspaceSectionPage({
  activeRailItem,
  canManageBots,
  canManageCronjobs,
  canManageDepartments,
  canManageTickets,
  canManageWebhooks,
  canOpenAdmin,
  canUseOrderBilling,
  canUseOrderBot,
  channels,
  contacts,
  currentUser,
  desktopVersionStatus,
  departments,
  files,
  friendSearchQuery,
  isCreatingDirectConversation,
  isCreatingDepartment,
  isLoadingChannels,
  isLoadingContacts,
  isLoadingDepartments,
  isLoadingFiles,
  isMutatingChannelMembership,
  isTicketCreateOpen,
  isUpdatingProfile,
  isAutoStartEnabled,
  isAutoStartLoading,
  isDesktopUpdateInstalling,
  joinRequestsByChannelId,
  notificationPreferences,
  onApproveChannelJoin,
  onAutoStartChange,
  onChannelSelect,
  onCreateDepartment,
  onDesktopUpdateInstall,
  onDownloadFile,
  onInviteChannelMember,
  onRejectChannelJoin,
  onRequestChannelJoin,
  onFriendSearchChange,
  onProfileSubmit,
  onNotificationPreferencesChange,
  onSecondaryContactAction,
  onStartConversation,
  onThemeToggle,
  onTicketCreateOpenChange,
  theme,
  workspaceId,
  workspaceMembers
}: {
  activeRailItem: RailItemId;
  canManageBots: boolean;
  canManageCronjobs: boolean;
  canManageDepartments: boolean;
  canManageTickets: boolean;
  canManageWebhooks: boolean;
  canOpenAdmin: boolean;
  canUseOrderBilling: boolean;
  canUseOrderBot: boolean;
  channels: ChatChannel[];
  contacts: ContactResult[];
  currentUser: ChatUser;
  desktopVersionStatus: DesktopVersionStatus;
  departments: Department[];
  files: FileItem[];
  friendSearchQuery: string;
  isCreatingDirectConversation: boolean;
  isCreatingDepartment: boolean;
  isLoadingChannels: boolean;
  isLoadingContacts: boolean;
  isLoadingDepartments: boolean;
  isLoadingFiles: boolean;
  isMutatingChannelMembership: boolean;
  isTicketCreateOpen: boolean;
  isUpdatingProfile: boolean;
  isAutoStartEnabled: boolean;
  isAutoStartLoading: boolean;
  isDesktopUpdateInstalling: boolean;
  joinRequestsByChannelId: Map<string, ChannelMember[]>;
  notificationPreferences: NotificationPreferences;
  onApproveChannelJoin: (channelId: string, userId: string) => void;
  onAutoStartChange: (enabled: boolean) => void;
  onChannelSelect: (channelId: string) => void;
  onCreateDepartment: (input: CreateDepartmentPayload) => void;
  onDesktopUpdateInstall: () => void;
  onDownloadFile: (file: FileItem) => void;
  onInviteChannelMember: (channelId: string, userId: string) => void;
  onRejectChannelJoin: (channelId: string, userId: string) => void;
  onRequestChannelJoin: (channelId: string) => void;
  onFriendSearchChange: (value: string) => void;
  onProfileSubmit: (input: {
    avatar_url?: string | null;
    display_name?: string;
    phone_number?: string | null;
  }) => void;
  onNotificationPreferencesChange: (preferences: NotificationPreferences) => void;
  onSecondaryContactAction: (contact: ContactResult) => void;
  onStartConversation: (contact: ContactResult) => void;
  onThemeToggle: () => void;
  onTicketCreateOpenChange: (isOpen: boolean) => void;
  theme: "dark" | "light";
  workspaceId?: string;
  workspaceMembers: WorkspaceMember[];
}) {
  if (activeRailItem === "contacts") {
    return (
      <ContactsPage
        contacts={contacts}
        isCreatingDirectConversation={isCreatingDirectConversation}
        isLoading={isLoadingContacts}
        onSearchChange={onFriendSearchChange}
        onSecondaryAction={onSecondaryContactAction}
        onStartConversation={onStartConversation}
        query={friendSearchQuery}
        workspaceId={workspaceId}
      />
    );
  }

  if (activeRailItem === "channels") {
    return (
      <ChannelsDirectoryPage
        channels={channels}
        isLoading={isLoadingChannels}
        isMutatingMembership={isMutatingChannelMembership}
        joinRequestsByChannelId={joinRequestsByChannelId}
        onApproveJoin={onApproveChannelJoin}
        onChannelSelect={onChannelSelect}
        onInviteMember={onInviteChannelMember}
        onRejectJoin={onRejectChannelJoin}
        onRequestJoin={onRequestChannelJoin}
        workspaceMembers={workspaceMembers}
      />
    );
  }

  if (activeRailItem === "departments") {
    return (
      <DepartmentsPage
        canManage={canManageDepartments}
        channels={channels}
        departments={departments}
        isCreating={isCreatingDepartment}
        isLoading={isLoadingDepartments}
        onCreate={onCreateDepartment}
        workspaceId={workspaceId}
        workspaceMembers={workspaceMembers}
      />
    );
  }

  if (activeRailItem === "files") {
    return <FilesPage files={files} isLoading={isLoadingFiles} onDownloadFile={onDownloadFile} />;
  }

  if (activeRailItem === "tickets") {
    return (
      <TicketsPage
        canManage={canManageTickets}
        channels={channels}
        isCreateOpen={isTicketCreateOpen}
        onCreateOpenChange={onTicketCreateOpenChange}
        workspaceId={workspaceId}
        workspaceMembers={workspaceMembers}
      />
    );
  }

  if (activeRailItem === "settings") {
    return (
      <SettingsPage
        currentUser={currentUser}
        desktopVersionStatus={desktopVersionStatus}
        isAutoStartEnabled={isAutoStartEnabled}
        isAutoStartLoading={isAutoStartLoading}
        isDesktopUpdateInstalling={isDesktopUpdateInstalling}
        isUpdatingProfile={isUpdatingProfile}
        notificationPreferences={notificationPreferences}
        canOpenAdmin={canOpenAdmin}
        onAutoStartChange={onAutoStartChange}
        onDesktopUpdateInstall={onDesktopUpdateInstall}
        onNotificationPreferencesChange={onNotificationPreferencesChange}
        onProfileSubmit={onProfileSubmit}
        onThemeToggle={onThemeToggle}
        theme={theme}
      />
    );
  }

  if (activeRailItem === "bots") {
    return (
      <BotsPage
        canBillOrder={canUseOrderBilling}
        canManage={canManageBots}
        canUseOrder={canUseOrderBot}
        channels={channels}
        workspaceId={workspaceId}
      />
    );
  }

  if (activeRailItem === "automation") {
    return (
      <AutomationPage
        canManageCronjobs={canManageCronjobs}
        canManageWebhooks={canManageWebhooks}
        channels={channels}
        workspaceId={workspaceId}
      />
    );
  }

  return <OperationalPage activeRailItem={activeRailItem} />;
}

function ContactsPage({
  contacts,
  isCreatingDirectConversation,
  isLoading,
  onSearchChange,
  onSecondaryAction,
  onStartConversation,
  query,
  workspaceId
}: {
  contacts: ContactResult[];
  isCreatingDirectConversation: boolean;
  isLoading: boolean;
  onSearchChange: (value: string) => void;
  onSecondaryAction: (contact: ContactResult) => void;
  onStartConversation: (contact: ContactResult) => void;
  query: string;
  workspaceId?: string;
}) {
  const isSearching = query.trim().length >= 2;
  const [activeTab, setActiveTab] = useState<ContactsTab>("employees");
  const employeeContacts = contacts.filter((contact) => contact.isWorkspaceMember);
  const friendContacts = contacts.filter((contact) => contact.contactStatus === "accepted");
  const discoverContacts = contacts.filter(
    (contact) => !contact.isWorkspaceMember && contact.contactStatus !== "accepted"
  );
  const visibleContacts = activeTab === "employees"
    ? employeeContacts
    : activeTab === "friends"
      ? friendContacts
      : discoverContacts;
  const tabItems: Array<{ count: number; label: string; value: ContactsTab }> = [
    { count: employeeContacts.length, label: "Nội bộ", value: "employees" },
    { count: friendContacts.length, label: "Bạn bè", value: "friends" },
    { count: discoverContacts.length, label: "Khám phá", value: "discover" }
  ];

  return (
    <div className="workspace-page contacts-page">
      <div className="contacts-toolbar">
        <nav aria-label="Phân loại danh bạ" className="contacts-tabs">
          {tabItems.map((tab) => (
            <button
              aria-current={activeTab === tab.value ? "page" : undefined}
              className={activeTab === tab.value ? "contacts-tabs__item contacts-tabs__item--active" : "contacts-tabs__item"}
              key={tab.value}
              onClick={() => setActiveTab(tab.value)}
              type="button"
            >
              <span>{tab.label}</span>
              <strong><span>{tab.count}</span></strong>
            </button>
          ))}
        </nav>

        <section className="zalo-search-panel">
          <div className="zalo-search-panel__icon">
            <Search size={20} />
          </div>
          <div>
            <Input
              aria-label="Tìm người dùng bằng tên, số điện thoại hoặc email"
              onChange={(event) => onSearchChange(event.target.value)}
              placeholder={activeTab === "employees" ? "Tìm trong nội bộ" : activeTab === "friends" ? "Tìm trong bạn bè" : "Tìm theo tên, email hoặc số điện thoại"}
              value={query}
            />
          </div>
        </section>
      </div>

      {isLoading ? (
        <PanelSkeleton />
      ) : visibleContacts.length ? (
        <div className="workspace-data-table-shell">
          <table className="workspace-data-table contacts-data-table">
            <thead>
              <tr>
                <th scope="col">Liên hệ</th>
                <th scope="col">Số điện thoại</th>
                <th scope="col">Phân loại</th>
                <th scope="col">Trạng thái</th>
                <th className="workspace-data-table__actions-heading" scope="col">Thao tác</th>
              </tr>
            </thead>
            <tbody>
              {visibleContacts.map((contact) => (
                <tr key={contact.userId}>
                  <td data-label="Liên hệ">
                    <div className="workspace-data-table__identity">
                      <Avatar name={contact.name} size="md" src={contact.avatarUrl ?? undefined} />
                      <span>
                        <strong>{contact.name}</strong>
                        <small>{contact.email ?? contact.username ?? "Chưa có email"}</small>
                      </span>
                    </div>
                  </td>
                  <td data-label="Số điện thoại">{contact.phoneNumber || "Chưa có số điện thoại"}</td>
                  <td data-label="Phân loại">
                    <Badge tone={contact.isWorkspaceMember ? "blue" : "slate"}>
                      {contact.isWorkspaceMember ? "Nhân viên hệ thống" : "Ngoài workspace"}
                    </Badge>
                  </td>
                  <td data-label="Trạng thái">
                    <Badge tone={contact.contactStatus === "accepted" ? "green" : contact.contactStatus === "pending" ? "orange" : "blue"}>
                      {contactBadgeLabel(contact)}
                    </Badge>
                  </td>
                  <td data-label="Thao tác">
                    <div className="workspace-data-table__actions">
                      <Button
                        disabled={
                          isCreatingDirectConversation ||
                          (contact.contactStatus === "pending" && contact.contactDirection === "outgoing")
                        }
                        onClick={() => onStartConversation(contact)}
                        size="sm"
                        variant={contact.contactStatus === "pending" && contact.contactDirection === "outgoing" ? "secondary" : "primary"}
                      >
                        <MessageCircle size={16} />
                        {contactActionLabel(contact)}
                      </Button>
                      {contact.contactStatus === "pending" && contact.contactRequestId ? (
                        <Button
                          disabled={isCreatingDirectConversation}
                          onClick={() => onSecondaryAction(contact)}
                          size="sm"
                          variant="ghost"
                        >
                          {contact.contactDirection === "incoming" ? "Từ chối" : "Hủy lời mời"}
                        </Button>
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState
          description={
            !workspaceId
              ? "Đang chuẩn bị dữ liệu để mở hội thoại riêng."
              : activeTab === "discover" && !isSearching
                ? "Nhập ít nhất 2 ký tự để tìm người dùng ngoài workspace và gửi lời mời kết bạn."
                : isSearching
                  ? "Không tìm thấy người dùng phù hợp với từ khóa này trong nhóm đang chọn."
                  : activeTab === "friends"
                    ? "Bạn chưa có bạn bè trong danh bạ."
                    : "Workspace chưa có nhân viên nào khác."
          }
          title={!workspaceId ? "Đang chuẩn bị" : activeTab === "discover" && !isSearching ? "Tìm người lạ để kết bạn" : isSearching ? "Không có kết quả" : "Chưa có liên hệ"}
        />
      )}
    </div>
  );
}

function contactActionLabel(contact: ContactResult): string {
  if (contact.contactStatus === "accepted") {
    return contact.hasConversation ? "Mở chat" : "Nhắn tin";
  }
  if (contact.contactStatus === "pending" && contact.contactDirection === "incoming") {
    return "Đồng ý";
  }
  if (contact.contactStatus === "pending") {
    return "Đang chờ";
  }
  return "Gửi lời mời";
}

function contactBadgeLabel(contact: ContactResult): string {
  if (contact.contactStatus === "accepted") {
    return "Bạn bè";
  }
  if (contact.contactStatus === "pending" && contact.contactDirection === "incoming") {
    return "Lời mời đến";
  }
  if (contact.contactStatus === "pending") {
    return "Đã gửi lời mời";
  }
  return "Có thể kết bạn";
}

function ChannelsDirectoryPage({
  channels,
  isLoading,
  isMutatingMembership,
  joinRequestsByChannelId,
  onApproveJoin,
  onChannelSelect,
  onInviteMember,
  onRejectJoin,
  onRequestJoin,
  workspaceMembers
}: {
  channels: ChatChannel[];
  isLoading: boolean;
  isMutatingMembership: boolean;
  joinRequestsByChannelId: Map<string, ChannelMember[]>;
  onApproveJoin: (channelId: string, userId: string) => void;
  onChannelSelect: (channelId: string) => void;
  onInviteMember: (channelId: string, userId: string) => void;
  onRejectJoin: (channelId: string, userId: string) => void;
  onRequestJoin: (channelId: string) => void;
  workspaceMembers: WorkspaceMember[];
}) {
  return (
    <div className="workspace-page">
      {isLoading ? (
        <PanelSkeleton />
      ) : channels.length ? (
        <div className="workspace-data-table-shell">
          <table className="workspace-data-table channels-data-table">
            <thead>
              <tr>
                <th scope="col">Kênh</th>
                <th scope="col">Mô tả</th>
                <th scope="col">Thành viên</th>
                <th scope="col">Chưa đọc</th>
                <th scope="col">Trạng thái</th>
                <th className="workspace-data-table__actions-heading" scope="col">Thao tác</th>
              </tr>
            </thead>
            <tbody>
              {channels.map((channel) => (
                <tr key={channel.id}>
                  <td data-label="Kênh">
                    <div className="workspace-data-table__identity">
                      <span className={`channel-hash channel-hash--${channel.tone}`} style={channelHashStyle(channel)}>
                        <Hash size={18} />
                      </span>
                      <span><strong>{channel.name}</strong></span>
                    </div>
                  </td>
                  <td className="workspace-data-table__description" data-label="Mô tả">{channel.description || "Chưa có mô tả"}</td>
                  <td data-label="Thành viên">{channel.memberCount}</td>
                  <td data-label="Chưa đọc">
                    <Badge tone={channel.unreadCount ? "red" : "slate"}>{channel.unreadCount}</Badge>
                  </td>
                  <td data-label="Trạng thái">
                    <Badge
                      tone={
                        channel.isMember
                          ? "green"
                          : channel.privateSessionMode
                            ? "blue"
                            : channel.membershipStatus === "invited"
                              ? "orange"
                              : "slate"
                      }
                    >
                      {channel.isMember
                        ? "Đã tham gia"
                        : channel.privateSessionMode
                          ? "Phiên riêng"
                          : channel.membershipStatus === "invited"
                            ? "Chờ duyệt"
                            : "Chưa tham gia"}
                    </Badge>
                  </td>
                  <td data-label="Thao tác">
                    <div className="workspace-data-table__actions workspace-data-table__actions--stacked">
                      {channel.isMember ? (
                        <Button onClick={() => onChannelSelect(channel.id)} size="sm">
                          <MessageCircle size={16} /> Mở kênh
                        </Button>
                      ) : channel.privateSessionMode ? (
                        <Button disabled={isMutatingMembership} onClick={() => onChannelSelect(channel.id)} size="sm">
                          <MessageCircle size={16} /> Mở kênh
                        </Button>
                      ) : channel.membershipStatus === "invited" ? (
                        <Button disabled size="sm" variant="secondary">Đang chờ duyệt</Button>
                      ) : (
                        <Button disabled={isMutatingMembership} onClick={() => onRequestJoin(channel.id)} size="sm" variant="secondary">
                          <Users size={16} /> Yêu cầu tham gia
                        </Button>
                      )}
                      {channel.canManage ? (
                        <ChannelMembershipManager
                          channel={channel}
                          isPending={isMutatingMembership}
                          joinRequests={joinRequestsByChannelId.get(channel.id) ?? []}
                          onApprove={onApproveJoin}
                          onInvite={onInviteMember}
                          onReject={onRejectJoin}
                          workspaceMembers={workspaceMembers}
                        />
                      ) : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState description="Tạo kênh ở panel bên trái khi tài khoản có quyền." title="Chưa có kênh" />
      )}
    </div>
  );
}

function ChannelMembershipManager({
  channel,
  isPending,
  joinRequests,
  onApprove,
  onInvite,
  onReject,
  workspaceMembers
}: {
  channel: ChatChannel;
  isPending: boolean;
  joinRequests: ChannelMember[];
  onApprove: (channelId: string, userId: string) => void;
  onInvite: (channelId: string, userId: string) => void;
  onReject: (channelId: string, userId: string) => void;
  workspaceMembers: WorkspaceMember[];
}) {
  const [userId, setUserId] = useState("");

  return (
    <details className="channel-membership-manager">
      <summary>Quản lý thành viên {joinRequests.length ? `(${joinRequests.length} chờ duyệt)` : ""}</summary>
      <div className="channel-invite-row">
        <select aria-label={`Chọn thành viên mời vào ${channel.name}`} onChange={(event) => setUserId(event.target.value)} value={userId}>
          <option value="">Chọn thành viên workspace</option>
          {workspaceMembers.map((member) => (
            <option key={member.user_id} value={member.user_id}>
              {member.display_name || member.username || member.email || member.user_id}
            </option>
          ))}
        </select>
        <Button disabled={isPending || !userId} onClick={() => { onInvite(channel.id, userId); setUserId(""); }} size="sm" type="button">
          Mời vào kênh
        </Button>
      </div>
      {joinRequests.length ? (
        <div className="channel-join-requests">
          {joinRequests.map((request) => (
            <article key={request.user_id}>
              <span><strong>{request.display_name || request.username || request.email || "Người dùng"}</strong><small>Yêu cầu tham gia</small></span>
              <Button disabled={isPending} onClick={() => onApprove(channel.id, request.user_id)} size="sm">Duyệt</Button>
              <Button disabled={isPending} onClick={() => onReject(channel.id, request.user_id)} size="sm" variant="ghost">Từ chối</Button>
            </article>
          ))}
        </div>
      ) : <small>Chưa có yêu cầu tham gia mới.</small>}
    </details>
  );
}

function DepartmentsPage({
  canManage,
  channels,
  departments,
  isCreating,
  isLoading,
  onCreate,
  workspaceId,
  workspaceMembers
}: {
  canManage: boolean;
  channels: ChatChannel[];
  departments: Department[];
  isCreating: boolean;
  isLoading: boolean;
  onCreate: (input: CreateDepartmentPayload) => void;
  workspaceId?: string;
  workspaceMembers: WorkspaceMember[];
}) {
  const queryClient = useQueryClient();
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [parentId, setParentId] = useState("");
  const [query, setQuery] = useState("");
  const [coverageFilter, setCoverageFilter] = useState<"all" | "missing-lead" | "empty" | "no-channel">("all");
  const [selectedDepartmentId, setSelectedDepartmentId] = useState("");
  const [editName, setEditName] = useState("");
  const [editSlug, setEditSlug] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editParentId, setEditParentId] = useState("");
  const [memberUserId, setMemberUserId] = useState("");
  const [memberRole, setMemberRole] = useState<"lead" | "member">("member");
  const [channelId, setChannelId] = useState("");
  const [isDeleteConfirming, setIsDeleteConfirming] = useState(false);
  const [feedback, setFeedback] = useState<{ message: string; tone: "error" | "success" } | null>(null);

  const selectedDepartment = departments.find((department) => department.id === selectedDepartmentId) ?? null;
  const departmentRows = useMemo(() => buildDepartmentRows(departments), [departments]);
  const normalizedQuery = query.trim().toLocaleLowerCase("vi");
  const visibleRows = departmentRows.filter(({ department }) => {
    const matchesQuery = !normalizedQuery ||
      `${department.name} ${department.slug} ${department.description ?? ""}`.toLocaleLowerCase("vi").includes(normalizedQuery);
    const matchesCoverage = coverageFilter === "all" ||
      (coverageFilter === "missing-lead" && !department.lead_count) ||
      (coverageFilter === "empty" && !department.member_count) ||
      (coverageFilter === "no-channel" && !department.channel_count);
    return matchesQuery && matchesCoverage;
  });
  const invalidParentIds = useMemo(
    () => selectedDepartment ? departmentDescendantIds(departments, selectedDepartment.id) : new Set<string>(),
    [departments, selectedDepartment]
  );

  const membersQuery = useQuery({
    enabled: Boolean(workspaceId && selectedDepartmentId),
    queryFn: () => api.departments.members(workspaceId ?? "", selectedDepartmentId),
    queryKey: workspaceId && selectedDepartmentId
      ? queryKeys.departments.members(workspaceId, selectedDepartmentId)
      : ["departments", "members", "none"]
  });
  const departmentMembers = membersQuery.data ?? [];
  const memberIds = new Set(departmentMembers.map((member) => member.user_id));
  const assignableMembers = workspaceMembers.filter((member) => !memberIds.has(member.user_id));
  const assignedChannels = channels.filter((channel) => channel.departmentId === selectedDepartmentId);
  const assignableChannels = channels.filter((channel) => channel.departmentId !== selectedDepartmentId);

  const updateMutation = useMutation({
    mutationFn: (input: { departmentId: string; description: string; name: string; parentId: string; slug: string }) =>
      api.departments.update(workspaceId ?? "", input.departmentId, {
        description: input.description,
        name: input.name,
        parent_id: input.parentId,
        slug: input.slug
      }),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không cập nhật được phòng ban."), tone: "error" }),
    onSuccess: async (department) => {
      setEditName(department.name);
      setEditSlug(department.slug);
      setEditDescription(department.description ?? "");
      setEditParentId(department.parent_id ?? "");
      setFeedback({ message: "Đã cập nhật thông tin phòng ban.", tone: "success" });
      await queryClient.invalidateQueries({ queryKey: queryKeys.departments.all(workspaceId ?? "") });
    }
  });
  const deleteMutation = useMutation({
    mutationFn: (departmentId: string) => api.departments.delete(workspaceId ?? "", departmentId),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không xóa được phòng ban."), tone: "error" }),
    onSuccess: async () => {
      setSelectedDepartmentId("");
      setIsDeleteConfirming(false);
      setFeedback({ message: "Đã xóa phòng ban. Các phòng ban con được đưa về cấp gốc.", tone: "success" });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.departments.all(workspaceId ?? "") }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId ?? "") })
      ]);
    }
  });
  const upsertMemberMutation = useMutation({
    mutationFn: (input: { departmentId: string; role: "lead" | "member"; userId: string }) =>
      api.departments.addMember(workspaceId ?? "", input.departmentId, input.userId, input.role),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không cập nhật được thành viên phòng ban."), tone: "error" }),
    onSuccess: async (_member, input) => {
      setMemberUserId("");
      setFeedback({ message: "Đã cập nhật thành viên phòng ban.", tone: "success" });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.departments.members(workspaceId ?? "", input.departmentId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.departments.all(workspaceId ?? "") })
      ]);
    }
  });
  const removeMemberMutation = useMutation({
    mutationFn: (input: { departmentId: string; userId: string }) =>
      api.departments.removeMember(workspaceId ?? "", input.departmentId, input.userId),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không xóa được thành viên khỏi phòng ban."), tone: "error" }),
    onSuccess: async (_result, input) => {
      setFeedback({ message: "Đã xóa thành viên khỏi phòng ban.", tone: "success" });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.departments.members(workspaceId ?? "", input.departmentId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.departments.all(workspaceId ?? "") })
      ]);
    }
  });
  const assignChannelMutation = useMutation({
    mutationFn: (input: { channelId: string; departmentId: string }) =>
      api.channels.update(workspaceId ?? "", input.channelId, { department_id: input.departmentId }),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không cập nhật được kênh của phòng ban."), tone: "error" }),
    onSuccess: async () => {
      setChannelId("");
      setFeedback({ message: "Đã cập nhật kênh của phòng ban.", tone: "success" });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId ?? "") }),
        queryClient.invalidateQueries({ queryKey: queryKeys.departments.all(workspaceId ?? "") })
      ]);
    }
  });

  const isMutating = updateMutation.isPending || deleteMutation.isPending || upsertMemberMutation.isPending || removeMemberMutation.isPending || assignChannelMutation.isPending;
  const assignedChannelCount = channels.filter((channel) => channel.departmentId).length;
  const missingLeadCount = departments.filter((department) => !department.lead_count).length;
  const selectedLeadCount = departmentMembers.filter((member) => member.role === "lead").length;

  function openDepartment(department: Department) {
    setSelectedDepartmentId(department.id);
    setEditName(department.name);
    setEditSlug(department.slug);
    setEditDescription(department.description ?? "");
    setEditParentId(department.parent_id ?? "");
    setIsDeleteConfirming(false);
    setFeedback(null);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cleanName = name.trim();
    if (!cleanName) {
      return;
    }
    onCreate({
      description: description.trim(),
      name: cleanName,
      parent_id: parentId || undefined,
      slug: slugify(slug || cleanName)
    });
    setName("");
    setSlug("");
    setDescription("");
    setParentId("");
    setIsFormOpen(false);
  }

  return (
    <div className="workspace-page departments-page">
      <header className="workspace-page__header">
        <div>
          <h1>Phòng ban</h1>
        </div>
        <Button disabled={!canManage} onClick={() => setIsFormOpen((current) => !current)} size="sm">
          <Plus size={16} /> Tạo phòng ban
        </Button>
      </header>

      <section className="department-overview-grid">
        <article>
          <span><Users size={18} /></span>
          <div><strong>{departments.length}</strong><small>Tổng phòng ban</small></div>
        </article>
        <article>
          <span><ShieldCheck size={18} /></span>
          <div><strong>{missingLeadCount}</strong><small>Chưa có trưởng phòng</small></div>
        </article>
        <article>
          <span><Hash size={18} /></span>
          <div><strong>{assignedChannelCount}/{channels.length}</strong><small>Kênh đã gán phòng ban</small></div>
        </article>
        <article>
          <span><ShieldCheck size={18} /></span>
          <div><strong>{workspaceMembers.length}</strong><small>Thành viên workspace</small></div>
        </article>
      </section>

      {isFormOpen ? (
        <form className="department-create-form" onSubmit={handleSubmit}>
          <label>Tên phòng ban<input onChange={(event) => { setName(event.target.value); setSlug((current) => current || slugify(event.target.value)); }} placeholder="Ví dụ: Kinh doanh" required value={name} /></label>
          <label>Slug<input onChange={(event) => setSlug(event.target.value)} placeholder="kinh-doanh" required value={slug} /></label>
          <label>Thuộc phòng ban<select onChange={(event) => setParentId(event.target.value)} value={parentId}><option value="">Không có phòng ban cha</option>{departments.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
          <label className="department-create-form__description">Mô tả<textarea onChange={(event) => setDescription(event.target.value)} placeholder="Chức năng của phòng ban" value={description} /></label>
          <div><Button disabled={isCreating || !name.trim() || !slug.trim()} size="sm" type="submit">{isCreating ? "Đang tạo..." : "Tạo phòng ban"}</Button><Button onClick={() => setIsFormOpen(false)} size="sm" type="button" variant="ghost">Hủy</Button></div>
        </form>
      ) : null}

      {canManage ? (
        <div className="department-toolbar">
          <Input
            aria-label="Tìm phòng ban"
            leftAddon={<Search size={17} />}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Tìm theo tên, slug hoặc mô tả..."
            value={query}
          />
          <select
            aria-label="Lọc trạng thái phân công phòng ban"
            className="department-coverage-filter"
            onChange={(event) => setCoverageFilter(event.target.value as typeof coverageFilter)}
            value={coverageFilter}
          >
            <option value="all">Tất cả trạng thái</option>
            <option value="missing-lead">Chưa có trưởng phòng</option>
            <option value="empty">Chưa có nhân sự</option>
            <option value="no-channel">Chưa có kênh</option>
          </select>
          <Badge tone="blue">{departments.length} phòng ban</Badge>
        </div>
      ) : null}

      {feedback ? (
        <div className={`department-feedback department-feedback--${feedback.tone}`} role="status">
          <span>{feedback.message}</span>
          <button aria-label="Đóng thông báo" onClick={() => setFeedback(null)} type="button"><X size={15} /></button>
        </div>
      ) : null}

      {isLoading ? (
        <PanelSkeleton />
      ) : !canManage ? (
        <EmptyState description="Bạn cần quyền quản lý workspace để xem và tạo phòng ban." title="Không có quyền quản lý phòng ban" />
      ) : visibleRows.length ? (
        <div className="workspace-data-table-shell">
          <table className="workspace-data-table departments-data-table">
            <thead>
              <tr>
                <th scope="col">Phòng ban</th>
                <th scope="col">Mô tả</th>
                <th scope="col">Trưởng phòng</th>
                <th scope="col">Nhân sự</th>
                <th scope="col">Kênh</th>
                <th scope="col">Cấp</th>
                <th className="workspace-data-table__actions-heading" scope="col">Thao tác</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map(({ department, depth }) => (
                <tr className={department.id === selectedDepartmentId ? "workspace-data-table__row--active" : undefined} key={department.id}>
                  <td className="department-table__identity-cell" data-label="Phòng ban">
                    <div className="workspace-data-table__identity" style={{ paddingLeft: `${Math.min(depth, 4) * 14}px` }}>
                      <span className="department-table-icon"><Users size={17} /></span>
                      <span>
                        <strong>{department.name}</strong>
                        <small>#{department.slug}{department.parent_id ? ` · ${departmentName(departments, department.parent_id)}` : " · cấp gốc"}</small>
                      </span>
                    </div>
                  </td>
                  <td className="workspace-data-table__description department-table__description-cell" data-label="Mô tả">{department.description || "Chưa có mô tả"}</td>
                  <td className="department-table__metric-cell" data-label="Trưởng phòng">
                    <Badge tone={department.lead_count ? "green" : "orange"}>
                      {department.lead_count ?? 0}
                    </Badge>
                  </td>
                  <td className="department-table__metric-cell" data-label="Nhân sự">{department.member_count ?? 0}</td>
                  <td className="department-table__metric-cell" data-label="Kênh">{department.channel_count ?? 0}</td>
                  <td className="department-table__level-cell" data-label="Cấp">{department.parent_id ? <Badge tone="slate">Cấp {depth + 1}</Badge> : <Badge tone="blue">Gốc</Badge>}</td>
                  <td className="department-table__actions-cell" data-label="Thao tác">
                    <div className="workspace-data-table__actions">
                      <Button onClick={() => openDepartment(department)} size="sm" variant="secondary">Quản lý</Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : query.trim() || coverageFilter !== "all" ? (
        <EmptyState description="Thử từ khóa hoặc trạng thái phân công khác." title="Không tìm thấy phòng ban phù hợp" />
      ) : (
        <EmptyState description="Tạo phòng ban đầu tiên để tổ chức thành viên theo đội nhóm." title="Chưa có phòng ban" />
      )}

      {selectedDepartment ? (
        <section className="department-detail-panel">
          <header>
            <div>
              <span className="workspace-page__eyebrow">Chi tiết phòng ban</span>
              <h2>{selectedDepartment.name}</h2>
              <div className="department-detail-badges">
                <Badge tone="blue">{selectedLeadCount} trưởng phòng</Badge>
                <Badge tone="slate">{assignedChannels.length} kênh</Badge>
              </div>
              <p>#{selectedDepartment.slug} · {departmentMembers.length} thành viên · {assignedChannels.length} kênh</p>
            </div>
            <Button aria-label="Đóng chi tiết phòng ban" onClick={() => setSelectedDepartmentId("")} variant="icon"><X size={18} /></Button>
          </header>

          <div className="department-detail-grid">
            <form
              className="department-editor"
              onSubmit={(event) => {
                event.preventDefault();
                if (!editName.trim()) return;
                updateMutation.mutate({
                  departmentId: selectedDepartment.id,
                  description: editDescription.trim(),
                  name: editName.trim(),
                  parentId: editParentId,
                  slug: slugify(editSlug)
                });
              }}
            >
              <h3>Thông tin</h3>
              <label>Tên phòng ban<input onChange={(event) => setEditName(event.target.value)} required value={editName} /></label>
              <label>Slug<input onChange={(event) => setEditSlug(event.target.value)} required value={editSlug} /></label>
              <label>
                Phòng ban cấp trên
                <select onChange={(event) => setEditParentId(event.target.value)} value={editParentId}>
                  <option value="">Không có · cấp gốc</option>
                  {departments
                    .filter((department) => !invalidParentIds.has(department.id))
                    .map((department) => <option key={department.id} value={department.id}>{department.name}</option>)}
                </select>
              </label>
              <label>Mô tả<textarea onChange={(event) => setEditDescription(event.target.value)} value={editDescription} /></label>
              <div className="department-editor__actions">
                <Button disabled={isMutating || !editName.trim() || !editSlug.trim()} size="sm" type="submit">Lưu thay đổi</Button>
                {!isDeleteConfirming ? (
                  <Button disabled={isMutating} onClick={() => setIsDeleteConfirming(true)} size="sm" type="button" variant="ghost">Xóa phòng ban</Button>
                ) : (
                  <span className="department-delete-confirm">
                    <strong>Xác nhận xóa?</strong>
                    <Button disabled={isMutating} onClick={() => deleteMutation.mutate(selectedDepartment.id)} size="sm" type="button">Xóa</Button>
                    <Button disabled={isMutating} onClick={() => setIsDeleteConfirming(false)} size="sm" type="button" variant="ghost">Hủy</Button>
                  </span>
                )}
              </div>
            </form>

            <div className="department-members-manager">
              <h3>Thành viên</h3>
              <div className="department-member-add">
                <select aria-label="Chọn thành viên workspace" onChange={(event) => setMemberUserId(event.target.value)} value={memberUserId}>
                  <option value="">Chọn thành viên</option>
                  {assignableMembers.map((member) => (
                    <option key={member.user_id} value={member.user_id}>{workspaceMemberName(member)}</option>
                  ))}
                </select>
                <select aria-label="Vai trò phòng ban" onChange={(event) => setMemberRole(event.target.value as "lead" | "member")} value={memberRole}>
                  <option value="member">Thành viên</option>
                  <option value="lead">Trưởng phòng</option>
                </select>
                <Button
                  disabled={isMutating || !memberUserId}
                  onClick={() => upsertMemberMutation.mutate({ departmentId: selectedDepartment.id, role: memberRole, userId: memberUserId })}
                  size="sm"
                  type="button"
                >
                  Thêm
                </Button>
              </div>
              {membersQuery.isLoading ? <Skeleton style={{ height: 90 }} /> : membersQuery.isError ? (
                <ErrorState
                  action={<Button onClick={() => void membersQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>}
                  description="Không tải được danh sách thành viên phòng ban."
                  title="Lỗi dữ liệu thành viên"
                />
              ) : departmentMembers.length ? (
                <div className="department-member-list">
                  {departmentMembers.map((member: DepartmentMember) => (
                    <article key={member.user_id}>
                      <Avatar
                        name={departmentMemberName(member)}
                        size="sm"
                        src={member.avatar_url || workspaceMembers.find((item) => item.user_id === member.user_id)?.avatar_url || undefined}
                      />
                      <span><strong>{departmentMemberName(member)}</strong><small>{member.email || member.username}</small></span>
                      <select
                        aria-label={`Vai trò của ${departmentMemberName(member)}`}
                        disabled={isMutating}
                        onChange={(event) => upsertMemberMutation.mutate({
                          departmentId: selectedDepartment.id,
                          role: event.target.value as "lead" | "member",
                          userId: member.user_id
                        })}
                        value={member.role}
                      >
                        <option value="member">Thành viên</option>
                        <option value="lead">Trưởng phòng</option>
                      </select>
                      <Button
                        aria-label={`Xóa ${departmentMemberName(member)} khỏi phòng ban`}
                        disabled={isMutating}
                        onClick={() => removeMemberMutation.mutate({ departmentId: selectedDepartment.id, userId: member.user_id })}
                        size="sm"
                        variant="ghost"
                      >
                        Xóa
                      </Button>
                    </article>
                  ))}
                </div>
              ) : <EmptyState description="Thêm người trong workspace và gán vai trò trưởng phòng hoặc thành viên." title="Chưa có thành viên" />}
            </div>

            <div className="department-channel-manager">
              <h3>Kênh của phòng ban</h3>
              <div className="department-channel-add">
                <select aria-label="Chọn kênh để gán" onChange={(event) => setChannelId(event.target.value)} value={channelId}>
                  <option value="">Chọn kênh</option>
                  {assignableChannels.map((channel) => <option key={channel.id} value={channel.id}>{channel.name}</option>)}
                </select>
                <Button
                  disabled={isMutating || !channelId}
                  onClick={() => assignChannelMutation.mutate({ channelId, departmentId: selectedDepartment.id })}
                  size="sm"
                  type="button"
                >
                  Gán kênh
                </Button>
              </div>
              {assignedChannels.length ? (
                <div className="department-channel-list">
                  {assignedChannels.map((channel) => (
                    <article key={channel.id}>
                      <span className={`channel-hash channel-hash--${channel.tone}`} style={channelHashStyle(channel)}>#</span>
                      <span><strong>{channel.name}</strong><small>{channel.description}</small></span>
                      <Button
                        disabled={isMutating}
                        onClick={() => assignChannelMutation.mutate({ channelId: channel.id, departmentId: "" })}
                        size="sm"
                        variant="ghost"
                      >
                        Bỏ khỏi phòng ban
                      </Button>
                    </article>
                  ))}
                </div>
              ) : <small>Chưa có kênh nào được gán cho phòng ban.</small>}
            </div>
          </div>
        </section>
      ) : null}
    </div>
  );
}

function departmentName(departments: Department[], departmentId: string) {
  return departments.find((department) => department.id === departmentId)?.name ?? "phòng ban đã xóa";
}

function workspaceMemberName(member: WorkspaceMember) {
  return member.display_name || member.username || member.email || member.user_id;
}

function departmentMemberName(member: DepartmentMember) {
  return member.display_name || member.username || member.email || member.user_id;
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

const legacyTicketStatusOptions: Array<{ label: string; value: TicketStatus }> = [
  { label: "Má»Ÿ", value: "open" },
  { label: "Äang chá»", value: "pending" },
  { label: "ÄÃ£ xá»­ lÃ½", value: "resolved" },
  { label: "ÄÃ£ Ä‘Ã³ng", value: "closed" }
];

const legacyTicketPriorityOptions: Array<{ label: string; value: TicketPriority }> = [
  { label: "Tháº¥p", value: "low" },
  { label: "BÃ¬nh thÆ°á»ng", value: "normal" },
  { label: "Cao", value: "high" },
  { label: "Kháº©n cáº¥p", value: "urgent" }
];

const ticketStatusText: Record<TicketStatus, string> = {
  closed: "Đã đóng",
  open: "Mới",
  pending: "Đang chờ",
  resolved: "Đã xử lý"
};

const ticketPriorityText: Record<TicketPriority, string> = {
  high: "Cao",
  low: "Thấp",
  normal: "Bình thường",
  urgent: "Khẩn cấp"
};

const ticketStatusOptions = legacyTicketStatusOptions.map((option) => ({
  ...option,
  label: ticketStatusText[option.value]
}));

const ticketPriorityOptions = legacyTicketPriorityOptions.map((option) => ({
  ...option,
  label: ticketPriorityText[option.value]
}));

function TicketsPage({
  canManage,
  channels,
  isCreateOpen,
  onCreateOpenChange,
  workspaceId,
  workspaceMembers
}: {
  canManage: boolean;
  channels: ChatChannel[];
  isCreateOpen: boolean;
  onCreateOpenChange: (isOpen: boolean) => void;
  workspaceId?: string;
  workspaceMembers: WorkspaceMember[];
}) {
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState<TicketStatus | "all">("all");
  const [selectedTicketId, setSelectedTicketId] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [channelId, setChannelId] = useState("");
  const [priority, setPriority] = useState<TicketPriority>("normal");
  const [assignedTo, setAssignedTo] = useState("");
  const [feedback, setFeedback] = useState<{ message: string; tone: "error" | "success" } | null>(null);
  const ticketStatus = statusFilter === "all" ? "" : statusFilter;

  const ticketsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.tickets.list(workspaceId as string, { status: ticketStatus, limit: 80 }),
    queryKey: queryKeys.tickets.all(workspaceId ?? "", ticketStatus)
  });
  const tickets = ticketsQuery.data ?? [];
  const selectedTicket = tickets.find((ticket) => ticket.id === selectedTicketId) ?? tickets[0];
  const assignableMembers = workspaceMembers.filter((member) => member.status === "active" || member.status === "muted");
  const openCount = tickets.filter((ticket) => ticket.status === "open" || ticket.status === "pending").length;
  const urgentCount = tickets.filter((ticket) => ticket.priority === "urgent" || ticket.priority === "high").length;

  useEffect(() => {
    if (tickets.length && !tickets.some((ticket) => ticket.id === selectedTicketId)) {
      setSelectedTicketId(tickets[0].id);
    }
  }, [tickets, selectedTicketId]);

  const createTicketMutation = useMutation({
    mutationFn: () => api.tickets.create(workspaceId as string, {
      assigned_to: assignedTo || undefined,
      channel_id: channelId || undefined,
      description: description.trim(),
      priority,
      title: title.trim()
    }),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không tạo được ticket."), tone: "error" }),
    onSuccess: async (ticket) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tickets.all(workspaceId ?? "", ticketStatus) });
      setSelectedTicketId(ticket.id);
      setTitle("");
      setDescription("");
      setChannelId("");
      setPriority("normal");
      setAssignedTo("");
      onCreateOpenChange(false);
      setFeedback({ message: "Đã tạo ticket.", tone: "success" });
    }
  });

  const updateTicketMutation = useMutation({
    mutationFn: ({ ticket, values }: { ticket: SupportTicket; values: { assigned_to?: string; channel_id?: string; description?: string; priority?: TicketPriority; status?: TicketStatus; title?: string } }) =>
      api.tickets.update(workspaceId as string, ticket.id, {
        assigned_to: values.assigned_to ?? undefined,
        channel_id: values.channel_id ?? undefined,
        description: values.description,
        priority: values.priority,
        status: values.status,
        title: values.title
      }),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không cập nhật được ticket."), tone: "error" }),
    onSuccess: async (ticket) => {
      setSelectedTicketId(ticket.id);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tickets.all(workspaceId ?? "", ticketStatus) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tickets.detail(workspaceId ?? "", ticket.id) })
      ]);
      setFeedback({ message: "Đã cập nhật ticket.", tone: "success" });
    }
  });

  function handleCreateTicket(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!workspaceId || !title.trim()) {
      return;
    }
    createTicketMutation.mutate();
  }

  return (
    <div className="workspace-page tickets-page">
      <header className="workspace-page__header tickets-page__header">
        <div>
          <h1>Ticket</h1>
        </div>
        <Button className="tickets-page__create-button" onClick={() => onCreateOpenChange(!isCreateOpen)} size="sm">
          {isCreateOpen ? <X size={16} /> : <Plus size={16} />}
          {isCreateOpen ? "Đóng" : "Tạo ticket"}
        </Button>
      </header>

      <section className="ticket-summary-strip">
        <div><strong>{tickets.length}</strong><small>Ticket trong bộ lọc</small></div>
        <div><strong>{openCount}</strong><small>Cần xử lý</small></div>
        <div><strong>{urgentCount}</strong><small>Ưu tiên cao</small></div>
        <SegmentedControl
          aria-label="Lọc ticket"
          onValueChange={(value: TicketStatus | "all") => setStatusFilter(value)}
          options={[{ label: "Tất cả", value: "all" as const }, ...ticketStatusOptions]}
          value={statusFilter}
        />
      </section>

      {feedback ? (
        <div className={`bot-feedback bot-feedback--${feedback.tone}`} role="status">
          <span>{feedback.tone === "success" ? <CheckCircle2 size={17} /> : <Info size={17} />}{feedback.message}</span>
          <button aria-label="Đóng thông báo" onClick={() => setFeedback(null)} type="button"><X size={15} /></button>
        </div>
      ) : null}

      {isCreateOpen ? (
        <form className="ticket-create-form" id="ticket-create-form" onSubmit={handleCreateTicket}>
          <label>Tiêu đề<input autoFocus onChange={(event) => setTitle(event.target.value)} placeholder="Khách không truy cập được VPS" required value={title} /></label>
          <label>Kênh liên kết<select onChange={(event) => setChannelId(event.target.value)} value={channelId}>
            <option value="">Không gắn kênh</option>
            {channels.map((channel) => <option key={channel.id} value={channel.id}>#{channel.name}</option>)}
          </select></label>
          <label>Ưu tiên<select onChange={(event) => setPriority(event.target.value as TicketPriority)} value={priority}>
            {ticketPriorityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select></label>
          <label>Người phụ trách<select onChange={(event) => setAssignedTo(event.target.value)} value={assignedTo}>
            <option value="">Chưa gắn</option>
            {assignableMembers.map((member) => <option key={member.user_id} value={member.user_id}>{workspaceMemberName(member)}</option>)}
          </select></label>
          <label className="ticket-create-form__description">Mô tả<textarea onChange={(event) => setDescription(event.target.value)} placeholder="Tóm tắt triệu chứng, khách hàng, dịch vụ bị ảnh hưởng..." value={description} /></label>
          <Button disabled={createTicketMutation.isPending || !title.trim()} size="sm" type="submit">
            {createTicketMutation.isPending ? "Đang tạo..." : "Tạo ticket"}
          </Button>
        </form>
      ) : null}

      <div className="ticket-workspace-grid">
        <section className="ticket-list-panel">
          {ticketsQuery.isLoading ? (
            <PanelSkeleton />
          ) : ticketsQuery.isError ? (
            <ErrorState action={<Button onClick={() => void ticketsQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>} description="Không tải được ticket." title="Lỗi ticket" />
          ) : tickets.length ? (
            tickets.map((ticket) => (
              <button className={ticket.id === selectedTicket?.id ? "ticket-row ticket-row--active" : "ticket-row"} key={ticket.id} onClick={() => setSelectedTicketId(ticket.id)} type="button">
                <span><Ticket size={18} /></span>
                <span>
                  <strong>{ticket.title}</strong>
                  <small>{cleanTicketChannelLabel(channels, ticket.channel_id)} - {cleanTicketAssigneeLabel(workspaceMembers, ticket.assigned_to)}</small>
                </span>
                <Badge tone={ticket.priority === "urgent" || ticket.priority === "high" ? "red" : "slate"}>{ticketPriorityLabel(ticket.priority)}</Badge>
                <Badge tone={ticket.status === "closed" || ticket.status === "resolved" ? "green" : "blue"}>{ticketStatusLabel(ticket.status)}</Badge>
              </button>
            ))
          ) : (
            <EmptyState description="Không có ticket nào trong bộ lọc hiện tại." title="Chưa có ticket" />
          )}
        </section>

        <aside className="ticket-detail-panel">
          {selectedTicket ? (
            <>
              <header>
                <span><Ticket size={22} /></span>
                <div>
                  <h2>{selectedTicket.title}</h2>
                  <p>{cleanTicketChannelLabel(channels, selectedTicket.channel_id)} - cập nhật {cleanTicketDate(selectedTicket.updated_at)}</p>
                </div>
              </header>
              <p>{selectedTicket.description || "Chưa có mô tả chi tiết."}</p>
              <div className="ticket-detail-meta">
                <span><strong>Trạng thái</strong><Badge tone="blue">{ticketStatusLabel(selectedTicket.status)}</Badge></span>
                <span><strong>Ưu tiên</strong><Badge tone={selectedTicket.priority === "urgent" || selectedTicket.priority === "high" ? "red" : "slate"}>{ticketPriorityLabel(selectedTicket.priority)}</Badge></span>
                <span><strong>Phụ trách</strong><small>{cleanTicketAssigneeLabel(workspaceMembers, selectedTicket.assigned_to)}</small></span>
                <span><strong>Tạo lúc</strong><small>{cleanTicketDate(selectedTicket.created_at)}</small></span>
              </div>
              {canManage ? (
                <div className="ticket-detail-actions">
                  <select aria-label="Trạng thái ticket" onChange={(event) => updateTicketMutation.mutate({ ticket: selectedTicket, values: { status: event.target.value as TicketStatus } })} value={selectedTicket.status}>
                    {ticketStatusOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                  <select aria-label="Ưu tiên ticket" onChange={(event) => updateTicketMutation.mutate({ ticket: selectedTicket, values: { priority: event.target.value as TicketPriority } })} value={selectedTicket.priority}>
                    {ticketPriorityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                  <select aria-label="Người phụ trách ticket" onChange={(event) => updateTicketMutation.mutate({ ticket: selectedTicket, values: { assigned_to: event.target.value || "" } })} value={selectedTicket.assigned_to ?? ""}>
                    <option value="">Chưa gắn</option>
                    {assignableMembers.map((member) => <option key={member.user_id} value={member.user_id}>{workspaceMemberName(member)}</option>)}
                  </select>
                  <Button disabled={updateTicketMutation.isPending || selectedTicket.status === "closed"} onClick={() => updateTicketMutation.mutate({ ticket: selectedTicket, values: { status: "closed" } })} size="sm">
                    Đóng ticket
                  </Button>
                </div>
              ) : (
                <small>Bạn có thể tạo và theo dõi ticket; cập nhật vòng đời cần quyền <code>ticket.manage</code>.</small>
              )}
            </>
          ) : (
            <EmptyState description="Chọn một ticket để xem vòng đời và người phụ trách." title="Chưa chọn ticket" />
          )}
        </aside>
      </div>
    </div>
  );
}

function ticketStatusLabel(status: TicketStatus) {
  return ticketStatusOptions.find((option) => option.value === status)?.label ?? status;
}

function ticketPriorityLabel(priority: TicketPriority) {
  return ticketPriorityOptions.find((option) => option.value === priority)?.label ?? priority;
}
function cleanTicketChannelLabel(channels: ChatChannel[], channelId?: string | null) {
  const channel = channels.find((item) => item.id === channelId);
  return channel ? `#${channel.name}` : "Chưa gắn kênh";
}

function cleanTicketAssigneeLabel(members: WorkspaceMember[], userId?: string | null) {
  const member = members.find((item) => item.user_id === userId);
  return member ? workspaceMemberName(member) : "Chưa gắn";
}

function cleanTicketDate(value?: string | null) {
  if (!value) return "Chưa có";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("vi-VN", { dateStyle: "short", timeStyle: "short" });
}

function FilesPage({
  files,
  isLoading,
  onDownloadFile
}: {
  files: FileItem[];
  isLoading: boolean;
  onDownloadFile: (file: FileItem) => void;
}) {
  const [query, setQuery] = useState("");
  const normalizedQuery = query.trim().toLocaleLowerCase("vi");
  const visibleFiles = normalizedQuery
    ? files.filter((file) => `${file.name} ${file.mimeType ?? ""}`.toLocaleLowerCase("vi").includes(normalizedQuery))
    : files;

  return (
    <div className="workspace-page">
      <div className="directory-toolbar">
        <Input
          aria-label="Tìm file"
          leftAddon={<Search size={17} />}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Tìm theo tên hoặc loại file..."
          value={query}
        />
        <Badge tone="blue">{files.length} file</Badge>
      </div>

      {isLoading ? (
        <PanelSkeleton />
      ) : visibleFiles.length ? (
        <div className="workspace-data-table-shell">
          <table className="workspace-data-table files-data-table">
            <thead>
              <tr>
                <th scope="col">Tệp</th>
                <th scope="col">Loại nội dung</th>
                <th scope="col">Kích thước</th>
                <th scope="col">Cập nhật</th>
                <th className="workspace-data-table__actions-heading" scope="col">Thao tác</th>
              </tr>
            </thead>
            <tbody>
              {visibleFiles.map((file) => (
                <tr key={file.id}>
                  <td className="file-table__identity-cell" data-label="Tệp">
                    <div className="workspace-data-table__identity">
                      <span className={`file-icon file-icon--${file.tone}`}><FileText size={18} /></span>
                      <span>
                        <strong>{file.name}</strong>
                        {fileSecurityLabel(file.status) ? <small className={`file-security file-security--${fileSecurityTone(file.status)}`}>{fileSecurityLabel(file.status)}</small> : null}
                      </span>
                    </div>
                  </td>
                  <td className="workspace-data-table__description file-table__type-cell" data-label="Loại nội dung">{file.mimeType || "Không xác định"}</td>
                  <td className="file-table__size-cell" data-label="Kích thước">{file.size}</td>
                  <td className="file-table__updated-cell" data-label="Cập nhật">{file.updatedAt}</td>
                  <td className="file-table__actions-cell" data-label="Thao tác">
                    <div className="workspace-data-table__actions">
                      <Button onClick={() => onDownloadFile(file)} size="sm" variant="secondary">
                        Tải xuống
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : query.trim() ? (
        <EmptyState description="Thử một tên file hoặc loại nội dung khác." title="Không tìm thấy file" />
      ) : (
        <EmptyState description="Chưa có file được chia sẻ trong các cuộc trò chuyện." title="Chưa có file" />
      )}
    </div>
  );
}

function SettingsPage({
  canOpenAdmin,
  currentUser,
  desktopVersionStatus,
  isAutoStartEnabled,
  isAutoStartLoading,
  isDesktopUpdateInstalling,
  isUpdatingProfile,
  notificationPreferences,
  onAutoStartChange,
  onDesktopUpdateInstall,
  onNotificationPreferencesChange,
  onProfileSubmit,
  onThemeToggle,
  theme
}: {
  canOpenAdmin: boolean;
  currentUser: ChatUser;
  desktopVersionStatus: DesktopVersionStatus;
  isAutoStartEnabled: boolean;
  isAutoStartLoading: boolean;
  isDesktopUpdateInstalling: boolean;
  isUpdatingProfile: boolean;
  notificationPreferences: NotificationPreferences;
  onAutoStartChange: (enabled: boolean) => void;
  onDesktopUpdateInstall: () => void;
  onNotificationPreferencesChange: (preferences: NotificationPreferences) => void;
  onProfileSubmit: (input: {
    avatar_url?: string | null;
    display_name?: string;
    phone_number?: string | null;
  }) => void;
  onThemeToggle: () => void;
  theme: "dark" | "light";
}) {
  const { logout } = useAuth();
  const queryClient = useQueryClient();
  const currentSessionId = useAuthStore((state) => state.sessionId);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const [avatarValue, setAvatarValue] = useState(currentUser.avatarUrl ?? "");
  const [avatarError, setAvatarError] = useState<string | null>(null);
  const [sessionActionError, setSessionActionError] = useState<string | null>(null);
  const hasDesktopUpdateNotice =
    desktopVersionStatus.status === "update_available" || desktopVersionStatus.status === "unsupported";
  const recommendedDesktopVersion = desktopVersionStatus.version?.clients?.desktop?.recommended_version;

  useEffect(() => setAvatarValue(currentUser.avatarUrl ?? ""), [currentUser.avatarUrl]);

  const sessionsQuery = useQuery({
    queryFn: () => api.auth.sessions(),
    queryKey: queryKeys.auth.sessions
  });
  const revokeSessionMutation = useMutation({
    mutationFn: (sessionId: string) => api.auth.revokeSession(sessionId),
    onError: (error) => setSessionActionError(error instanceof Error ? error.message : "Không thu hồi được phiên đăng nhập."),
    onSuccess: async (_, sessionId) => {
      setSessionActionError(null);
      if (sessionId === currentSessionId) {
        logout();
        return;
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.auth.sessions });
    }
  });
  const revokeAllSessionsMutation = useMutation({
    mutationFn: () => api.auth.revokeAllSessions(),
    onError: (error) => setSessionActionError(error instanceof Error ? error.message : "Không thu hồi được các phiên đăng nhập."),
    onSuccess: () => logout()
  });
  const sessions = useMemo(
    () => [...(sessionsQuery.data ?? [])].sort((left, right) => {
      if (left.id === currentSessionId) return -1;
      if (right.id === currentSessionId) return 1;
      if (Boolean(left.revoked_at) !== Boolean(right.revoked_at)) return left.revoked_at ? 1 : -1;
      return new Date(right.last_seen_at || right.created_at || 0).getTime() - new Date(left.last_seen_at || left.created_at || 0).getTime();
    }),
    [currentSessionId, sessionsQuery.data]
  );
  const activeSessionCount = sessions.filter((session) => !session.revoked_at).length;
  const isDesktopRuntime = getPlatformServices().lifecycle.isDesktop;

  async function handleAvatarFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) {
      return;
    }
    if (!file.type.startsWith("image/")) {
      setAvatarError("Vui lòng chọn file ảnh.");
      return;
    }
    if (file.size > 8 * 1024 * 1024) {
      setAvatarError("Ảnh đại diện không được vượt quá 8 MB.");
      return;
    }
    try {
      setAvatarValue(await resizeAvatarFile(file));
      setAvatarError(null);
    } catch {
      setAvatarError("Không đọc được ảnh đã chọn.");
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onProfileSubmit({
      avatar_url: formValue(form, "avatar_url") || null,
      display_name: formValue(form, "display_name"),
      phone_number: formValue(form, "phone_number") || null
    });
  }

  function handleOpenAdminPanel() {
    void getPlatformServices().links.openExternal(adminPanelUrl());
  }

  return (
    <div className="workspace-page settings-page">
      <div className="settings-grid">
        <section className="settings-card settings-card--profile">
          <div className="settings-card__heading">
            <div>
              <h2>Hồ sơ cá nhân</h2>
            </div>
          </div>
          <form className="profile-form" onSubmit={handleSubmit}>
            <input name="avatar_url" readOnly type="hidden" value={avatarValue} />
            <div className="profile-form__row">
              <div className="avatar-upload-field">
                <Avatar name={currentUser.name} size="lg" src={avatarValue || undefined} />
                <Button aria-label="Đổi ảnh đại diện" onClick={() => avatarInputRef.current?.click()} size="sm" type="button" variant="secondary">
                  <ImageIcon size={16} /> Đổi ảnh
                </Button>
                <input
                  accept="image/*"
                  className="visually-hidden"
                  onChange={handleAvatarFile}
                  ref={avatarInputRef}
                  type="file"
                />
              </div>
              <label>
                Tên hiển thị
                <input defaultValue={currentUser.name} name="display_name" placeholder="Tên của bạn" />
              </label>
              <label>
                Số điện thoại
                <input defaultValue={currentUser.phoneNumber ?? ""} name="phone_number" placeholder="Số điện thoại" />
              </label>
              <Button disabled={isUpdatingProfile} size="sm" type="submit">
                {isUpdatingProfile ? "Đang lưu..." : "Lưu hồ sơ"}
              </Button>
            </div>
            {avatarError ? <small className="profile-form__error">{avatarError}</small> : null}
          </form>
        </section>
        {canOpenAdmin ? (
          <section className="settings-card settings-card--admin-link">
            <div>
              <Monitor size={22} />
              <h2>Admin Panel</h2>
            </div>
            <p>Mo bang quan tri day du tren trinh duyet ngoai.</p>
            <Button onClick={handleOpenAdminPanel} size="sm" type="button" variant="secondary">
              <Share2 size={15} /> Mo Admin Panel
            </Button>
          </section>
        ) : null}
        <section className="settings-card settings-card--appearance">
          <div>
            {theme === "dark" ? <Moon size={22} /> : <Sun size={22} />}
            <h2>Giao diện</h2>
          </div>
          <p>Chế độ hiện tại: {theme === "dark" ? "tối" : "sáng"}.</p>
          <Button onClick={onThemeToggle} size="sm" variant="secondary">
            Chuyển chế độ
          </Button>
        </section>
        <div className="settings-desktop-grid">
          <section className={`settings-card settings-card--desktop-version settings-card--desktop-version-${desktopVersionStatus.status}`}>
          <div>
            <Cloud size={22} />
            <h2>Phiên bản desktop</h2>
          </div>
          <p>{desktopVersionStatus.label}</p>
          {desktopVersionStatus.detail ? <small className="settings-card__muted">{desktopVersionStatus.detail}</small> : null}
          {hasDesktopUpdateNotice ? (
            <div className="desktop-version-notice" role="status">
              <strong>{desktopVersionStatus.status === "unsupported" ? "Cần cập nhật để tiếp tục sử dụng" : "Có bản cập nhật mới"}</strong>
              <span>
                {recommendedDesktopVersion
                  ? `Phiên bản ${recommendedDesktopVersion} đã sẵn sàng. Bấm Cập nhật ngay để cài đặt.`
                  : "Bấm Cập nhật ngay để tải và cài đặt phiên bản mới nhất."}
              </span>
            </div>
          ) : null}
          <div className="desktop-version-actions">
            <Badge tone={desktopVersionBadgeTone(desktopVersionStatus.status)}>
              {desktopVersionStatus.status === "unsupported"
                ? "Cần cập nhật"
                : desktopVersionStatus.status === "update_available"
                  ? "Có bản mới"
                  : desktopVersionStatus.status === "offline"
                    ? "Chưa kiểm tra"
                    : desktopVersionStatus.status === "checking"
                      ? "Đang kiểm tra"
                      : "Tương thích"}
            </Badge>
            {isDesktopRuntime || desktopVersionStatus.updateUrl ? (
              <Button
                disabled={isDesktopUpdateInstalling || desktopVersionStatus.status === "checking"}
                onClick={onDesktopUpdateInstall}
                size="sm"
                type="button"
                variant={hasDesktopUpdateNotice ? "primary" : "secondary"}
              >
                {isDesktopRuntime ? <Cloud size={15} /> : <Share2 size={15} />}
                {isDesktopUpdateInstalling ? "Đang cập nhật..." : isDesktopRuntime ? "Cập nhật ngay" : "Mở trang cập nhật"}
              </Button>
            ) : null}
          </div>
          </section>
          <section className="settings-card settings-card--notifications">
          <div>
            <Bell size={22} />
            <h2>Thông báo desktop</h2>
          </div>
          <SegmentedControl
            aria-label="Chế độ thông báo"
            onValueChange={(value: NotificationMode) => onNotificationPreferencesChange({ ...notificationPreferences, mode: value })}
            options={notificationModeOptions}
            value={notificationPreferences.mode}
          />
          <label className="settings-toggle-row">
            <span>
              <strong>Hiển thị nội dung xem trước</strong>
              <small>{notificationPreferences.preview ? "Thông báo native hiện tiêu đề và nội dung." : "Thông báo native chỉ báo có tin mới."}</small>
            </span>
            <input
              checked={notificationPreferences.preview}
              onChange={(event) => onNotificationPreferencesChange({ ...notificationPreferences, preview: event.target.checked })}
              type="checkbox"
            />
          </label>
          <label className="settings-toggle-row">
            <span>
              <strong>Không làm phiền theo giờ</strong>
              <small>Không phát native notification trong khoảng giờ đã chọn.</small>
            </span>
            <input
              checked={notificationPreferences.quietHours}
              onChange={(event) => onNotificationPreferencesChange({ ...notificationPreferences, quietHours: event.target.checked })}
              type="checkbox"
            />
          </label>
          <div className="settings-time-row">
            <label>
              Bắt đầu
              <input
                disabled={!notificationPreferences.quietHours}
                onChange={(event) => onNotificationPreferencesChange({ ...notificationPreferences, quietStart: event.target.value })}
                type="time"
                value={notificationPreferences.quietStart}
              />
            </label>
            <label>
              Kết thúc
              <input
                disabled={!notificationPreferences.quietHours}
                onChange={(event) => onNotificationPreferencesChange({ ...notificationPreferences, quietEnd: event.target.value })}
                type="time"
                value={notificationPreferences.quietEnd}
              />
            </label>
          </div>
          <label className="settings-toggle-row">
            <span>
              <strong>Tự khởi động cùng hệ điều hành</strong>
              <small>Mặc định tắt; chỉ áp dụng cho bản desktop.</small>
            </span>
            <input
              checked={isAutoStartEnabled}
              disabled={isAutoStartLoading}
              onChange={(event) => onAutoStartChange(event.target.checked)}
              type="checkbox"
            />
          </label>
          </section>
        </div>
        <section className="settings-card settings-card--sessions">
          <div className="sessions-heading">
            <span className="sessions-heading__icon"><ShieldCheck size={22} /></span>
            <div>
              <h2>Phiên đăng nhập</h2>
            </div>
            <span className="sessions-count"><strong>{activeSessionCount}</strong> phiên đang hoạt động</span>
            <Button
              className="sessions-logout-all"
              disabled={revokeAllSessionsMutation.isPending || !activeSessionCount}
              onClick={() => {
                if (window.confirm("Đăng xuất khỏi tất cả thiết bị? Bạn sẽ cần đăng nhập lại.")) {
                  revokeAllSessionsMutation.mutate();
                }
              }}
              size="sm"
              variant="secondary"
            >
              <LogOut size={16} />
              {revokeAllSessionsMutation.isPending ? "Đang đăng xuất..." : "Đăng xuất tất cả thiết bị"}
            </Button>
          </div>
          {sessionsQuery.isLoading ? (
            <div className="session-list session-list--loading">
              <Skeleton style={{ height: 116 }} />
              <Skeleton style={{ height: 116 }} />
            </div>
          ) : sessionsQuery.isError ? (
            <ErrorState
              action={<Button onClick={() => void sessionsQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>}
              description="Không tải được danh sách thiết bị."
              title="Lỗi phiên đăng nhập"
            />
          ) : sessions.length ? (
            <div className="workspace-data-table-shell">
              <table className="workspace-data-table sessions-data-table">
                <thead>
                  <tr>
                    <th scope="col">Thiết bị</th>
                    <th scope="col">Địa chỉ IP</th>
                    <th scope="col">Hoạt động</th>
                    <th scope="col">Hết hạn</th>
                    <th scope="col">Trạng thái</th>
                    <th className="workspace-data-table__actions-heading" scope="col">Thao tác</th>
                  </tr>
                </thead>
                <tbody>
                  {sessions.map((session: AuthSession) => {
                    const isCurrent = session.id === currentSessionId;
                    const isRevoked = Boolean(session.revoked_at);
                    const isMobile = isMobileSession(session);
                    return (
                      <tr className={isCurrent ? "workspace-data-table__row--current" : isRevoked ? "workspace-data-table__row--revoked" : undefined} key={session.id}>
                        <td data-label="Thiết bị">
                          <div className="workspace-data-table__identity session-table__device">
                            <span className="session-device-icon" aria-hidden="true">
                              {isMobile ? <Smartphone size={19} /> : <Monitor size={19} />}
                            </span>
                            <span>
                              <strong>{session.device_name || sessionDeviceLabel(session)}</strong>
                              <small title={session.user_agent ?? undefined}>{session.user_agent || "Không có thông tin trình duyệt"}</small>
                            </span>
                          </div>
                        </td>
                        <td data-label="Địa chỉ IP">{session.ip_address || "Không xác định"}</td>
                        <td data-label="Hoạt động">{formatRelativeSessionDate(session.last_seen_at || session.created_at)}</td>
                        <td data-label="Hết hạn">{session.expires_at ? formatSessionDate(session.expires_at) : "Không xác định"}</td>
                        <td data-label="Trạng thái">
                          <Badge tone={isCurrent ? "green" : isRevoked ? "slate" : "blue"}>
                            {isCurrent ? "Thiết bị này" : isRevoked ? "Đã thu hồi" : "Đang hoạt động"}
                          </Badge>
                        </td>
                        <td data-label="Thao tác">
                          <div className="workspace-data-table__actions">
                            {!isCurrent && !isRevoked ? (
                              <Button
                                disabled={revokeSessionMutation.isPending || revokeAllSessionsMutation.isPending}
                                onClick={() => {
                                  if (window.confirm(`Thu hồi phiên trên ${session.device_name || sessionDeviceLabel(session)}?`)) {
                                    revokeSessionMutation.mutate(session.id);
                                  }
                                }}
                                size="sm"
                                variant="secondary"
                              >
                                {revokeSessionMutation.isPending && revokeSessionMutation.variables === session.id ? "Đang thu hồi..." : "Thu hồi phiên"}
                              </Button>
                            ) : <span className="session-table__no-action">—</span>}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : <EmptyState description="Các thiết bị đăng nhập sẽ xuất hiện tại đây." title="Chưa có phiên đăng nhập" />}
          {sessionActionError ? <p className="session-action-error" role="alert">{sessionActionError}</p> : null}
        </section>
      </div>
    </div>
  );
}

function formatSessionDate(value?: string | null) {
  if (!value) {
    return "Không rõ thời gian";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("vi-VN");
}

function formatRelativeSessionDate(value?: string | null) {
  if (!value) {
    return "Không rõ";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return formatSessionDate(value);
  }
  const difference = Date.now() - date.getTime();
  const minutes = Math.max(0, Math.round(difference / 60_000));
  if (minutes < 1) return "Vừa xong";
  if (minutes < 60) return `${minutes} phút trước`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} giờ trước`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days} ngày trước`;
  return formatSessionDate(value);
}

function isMobileSession(session: AuthSession) {
  return /android|iphone|ipad|mobile/i.test(`${session.device_name ?? ""} ${session.user_agent ?? ""}`);
}

function sessionDeviceLabel(session: AuthSession) {
  if (isMobileSession(session)) {
    return "Thiết bị di động";
  }
  if (/windows/i.test(session.user_agent ?? "")) return "Máy tính Windows";
  if (/macintosh|mac os/i.test(session.user_agent ?? "")) return "Máy tính Mac";
  if (/linux/i.test(session.user_agent ?? "")) return "Máy tính Linux";
  return "Trình duyệt web";
}

type OrderBotResult =
  | { data: OrderWalletBalanceData; kind: "wallet" }
  | { data: OrderWalletDepositQRData; kind: "deposit" }
  | { data: OrderPaymentQRData; kind: "order-payment" }
  | { data: OrderServicesExpiringData; kind: "expiring" };

const orderServiceTypeOptions: Array<{ label: string; value: NonNullable<OrderServicesExpiringInput["service_type"]> }> = [
  { label: "Tất cả", value: "all" },
  { label: "VPS", value: "vps" },
  { label: "Proxy", value: "proxy" },
  { label: "Hosting", value: "hosting" },
  { label: "S3", value: "s3" },
  { label: "Domain", value: "domain" }
];

function BotsPage({
  canBillOrder,
  canManage,
  canUseOrder,
  channels,
  workspaceId
}: {
  canBillOrder: boolean;
  canManage: boolean;
  canUseOrder: boolean;
  channels: ChatChannel[];
  workspaceId?: string;
}) {
  const queryClient = useQueryClient();
  const [selectedBotId, setSelectedBotId] = useState("");
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createSlug, setCreateSlug] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [targetChannelId, setTargetChannelId] = useState("");
  const [testMessage, setTestMessage] = useState("");
  const [orderEmail, setOrderEmail] = useState("");
  const [orderUserId, setOrderUserId] = useState("");
  const [orderDepositAmount, setOrderDepositAmount] = useState("200000");
  const [orderIntentCode, setOrderIntentCode] = useState("");
  const [orderExpiringDays, setOrderExpiringDays] = useState("7");
  const [orderServiceType, setOrderServiceType] = useState<OrderServicesExpiringInput["service_type"]>("all");
  const [orderResult, setOrderResult] = useState<OrderBotResult | null>(null);
  const [feedback, setFeedback] = useState<{ message: string; tone: "error" | "success" } | null>(null);
  const availableChannels = useMemo(() => channels.filter((channel) => channel.isMember), [channels]);

  const orderStatusQuery = useQuery({
    enabled: Boolean(workspaceId && canUseOrder),
    queryFn: () => api.orderBot.status(workspaceId as string),
    queryKey: queryKeys.orderBot.status(workspaceId ?? "")
  });
  const botsQuery = useQuery({
    enabled: Boolean(workspaceId && canManage),
    queryFn: () => api.bots.list(workspaceId as string),
    queryKey: queryKeys.integrations.bots(workspaceId ?? "")
  });
  const bots: BotRecord[] = botsQuery.data ?? [];
  const selectedBot = bots.find((bot) => bot.id === selectedBotId) ?? bots[0];

  async function privateOrderChannel(slug: "ticket" | "gia-han" | "ke-toan") {
    const source = channels.find((channel) => channel.slug === slug);
    if (!source) {
      throw new Error(`Không tìm thấy kênh #${slug}.`);
    }
    if (!source.privateSessionMode) {
      return source.id;
    }
    const session = await api.channels.openPrivateSession(workspaceId as string, source.id);
    return session.id;
  }

  useEffect(() => {
    if (bots.length && !bots.some((bot) => bot.id === selectedBotId)) {
      setSelectedBotId(bots[0].id);
    }
  }, [bots, selectedBotId]);

  useEffect(() => {
    if (availableChannels.length && !availableChannels.some((channel) => channel.id === targetChannelId)) {
      setTargetChannelId(availableChannels[0].id);
    }
  }, [availableChannels, targetChannelId]);

  const installationsQuery = useQuery({
    enabled: Boolean(workspaceId && canManage && selectedBot?.id),
    queryFn: () => api.bots.installations(workspaceId as string, selectedBot?.id ?? ""),
    queryKey: queryKeys.integrations.botInstallations(workspaceId ?? "", selectedBot?.id ?? "")
  });
  const createBotMutation = useMutation({
    mutationFn: (input: { description?: string; name: string; slug: string }) => api.bots.create(workspaceId as string, input),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không tạo được bot."), tone: "error" }),
    onSuccess: async (bot) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.bots(workspaceId ?? "") });
      setSelectedBotId(bot.id);
      setCreateName("");
      setCreateSlug("");
      setCreateDescription("");
      setIsCreateOpen(false);
      setFeedback({ message: `Đã tạo ${bot.name}.`, tone: "success" });
    }
  });
  const installBotMutation = useMutation({
    mutationFn: ({ botId, channelId }: { botId: string; channelId: string }) =>
      api.bots.install(workspaceId as string, botId, { channel_id: channelId, config: {} }),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không cài được bot vào kênh."), tone: "error" }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: queryKeys.integrations.botInstallations(workspaceId ?? "", selectedBot?.id ?? "")
      });
      const channel = availableChannels.find((item) => item.id === targetChannelId);
      setFeedback({ message: `Đã kết nối bot với #${channel?.name ?? "kênh"}.`, tone: "success" });
    }
  });
  const sendBotMessageMutation = useMutation({
    mutationFn: ({ botId, body, channelId }: { botId: string; body: string; channelId: string }) =>
      api.bots.sendMessage(workspaceId as string, botId, {
        body,
        channel_id: channelId,
        metadata: { source: "bot-console" }
      }),
    onError: (error) => setFeedback({ message: errorMessage(error, "Bot chưa gửi được tin nhắn."), tone: "error" }),
    onSuccess: async (_, input) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.messages.channel(workspaceId ?? "", input.channelId) });
      setTestMessage("");
      setFeedback({ message: "Bot đã gửi tin nhắn thử nghiệm.", tone: "success" });
    }
  });

  const orderWalletMutation = useMutation({
    mutationFn: async () => api.orderBot.walletBalance(workspaceId as string, {
      ...buildOrderLookup(orderEmail, orderUserId),
      channel_id: await privateOrderChannel("ticket")
    }),
    onMutate: () => setFeedback(null),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không tra được ví khách hàng."), tone: "error" }),
    onSuccess: async (result) => {
      setOrderResult({ data: result.data, kind: "wallet" });
      if (result.bot_message?.channel_id) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.messages.channel(workspaceId ?? "", result.bot_message.channel_id) });
      }
      setFeedback({ message: "CSKH Bot đã tra ví và gửi kết quả vào kênh ticket.", tone: "success" });
    }
  });
  const orderDepositMutation = useMutation({
    mutationFn: async () => api.orderBot.depositQr(workspaceId as string, {
      ...buildOrderEmail(orderEmail),
      amount: parsePositiveInt(orderDepositAmount),
      expires_minutes: 1440,
      channel_id: await privateOrderChannel("ke-toan")
    }),
    onMutate: () => setFeedback(null),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không tạo được QR nạp ví."), tone: "error" }),
    onSuccess: async (result) => {
      setOrderResult({ data: result.data, kind: "deposit" });
      if (result.bot_message?.channel_id) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.messages.channel(workspaceId ?? "", result.bot_message.channel_id) });
      }
      setFeedback({ message: "Thanh Toán Bot đã tạo QR và gửi vào kênh kế toán.", tone: "success" });
    }
  });
  const orderPaymentMutation = useMutation({
    mutationFn: async () => api.orderBot.orderPaymentQr(workspaceId as string, {
      intent_code: orderIntentCode.trim(),
      channel_id: await privateOrderChannel("ke-toan")
    }),
    onMutate: () => setFeedback(null),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không tạo được QR thanh toán đơn hàng."), tone: "error" }),
    onSuccess: async (result) => {
      setOrderResult({ data: result.data, kind: "order-payment" });
      if (result.bot_message?.channel_id) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.messages.channel(workspaceId ?? "", result.bot_message.channel_id) });
      }
      setFeedback({ message: "Thanh Toán Bot đã tạo QR cho đơn hàng.", tone: "success" });
    }
  });
  const orderExpiringMutation = useMutation({
    mutationFn: async () => api.orderBot.expiringServices(workspaceId as string, {
      ...buildOrderLookup(orderEmail, orderUserId),
      days: parsePositiveInt(orderExpiringDays),
      include_expired: false,
      service_type: orderServiceType,
      channel_id: await privateOrderChannel("gia-han")
    }),
    onMutate: () => setFeedback(null),
    onError: (error) => setFeedback({ message: errorMessage(error, "Không kiểm tra được dịch vụ sắp hết hạn."), tone: "error" }),
    onSuccess: async (result) => {
      setOrderResult({ data: result.data, kind: "expiring" });
      if (result.bot_message?.channel_id) {
        await queryClient.invalidateQueries({ queryKey: queryKeys.messages.channel(workspaceId ?? "", result.bot_message.channel_id) });
      }
      setFeedback({ message: "Gia Hạn Bot đã gửi danh sách dịch vụ cần chú ý.", tone: "success" });
    }
  });

  function handleCreateBot(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = createName.trim();
    const slug = slugify(createSlug || createName);
    if (!name || slug.length < 3 || !workspaceId) {
      setFeedback({ message: "Tên bot và slug từ 3 ký tự là bắt buộc.", tone: "error" });
      return;
    }
    createBotMutation.mutate({ description: createDescription.trim() || undefined, name, slug });
  }

  const activeBots = bots.filter((bot) => bot.status === "active").length;
  const installations = installationsQuery.data ?? [];
  const orderConfigured = orderStatusQuery.data?.configured ?? false;
  const orderBusy = orderWalletMutation.isPending || orderDepositMutation.isPending || orderPaymentMutation.isPending || orderExpiringMutation.isPending;

  return (
    <div className="workspace-page bot-page">
      <header className="workspace-page__header bot-page__header">
        <div>
          <h1>Bot</h1>
        </div>
        {canManage ? (
          <Button onClick={() => setIsCreateOpen((current) => !current)} size="sm">
            {isCreateOpen ? <X size={16} /> : <Plus size={16} />}
            {isCreateOpen ? "Đóng" : "Tạo bot"}
          </Button>
        ) : null}
      </header>

      <section className="bot-hero">
        <div className="bot-hero__copy">
          <Badge tone="blue"><Sparkles size={13} /> Bot workspace</Badge>
          <h2>Tự động hóa thông báo, cảnh báo và chăm sóc nội bộ</h2>
          <div className="bot-hero__stats">
            <span><strong>{bots.length}</strong> tổng bot</span>
            <span><strong>{activeBots}</strong> đang hoạt động</span>
            <span><strong>{installations.length}</strong> kết nối đã chọn</span>
          </div>
        </div>
        <div className="bot-animation" aria-hidden="true">
          <span className="bot-animation__orbit bot-animation__orbit--one"><i /></span>
          <span className="bot-animation__orbit bot-animation__orbit--two"><i /></span>
          <span className="bot-animation__signal bot-animation__signal--one" />
          <span className="bot-animation__signal bot-animation__signal--two" />
          <span className="bot-animation__core"><Bot size={44} /><b /></span>
          <Sparkles className="bot-animation__spark bot-animation__spark--one" size={18} />
          <Zap className="bot-animation__spark bot-animation__spark--two" size={17} />
        </div>
      </section>

      {!canManage ? (
        <section className="bot-permission-state">
          <ShieldCheck size={30} />
          <div>
            <h2>Cần quyền quản lý bot</h2>
            <p>Liên hệ quản trị viên workspace để được cấp quyền <code>bot.manage</code>.</p>
          </div>
        </section>
      ) : null}

      {canManage && isCreateOpen ? (
        <form className="bot-create-form" onSubmit={handleCreateBot}>
          <header>
            <span><Bot size={20} /></span>
            <div><strong>Tạo bot mới</strong><small>Bot sẽ sẵn sàng để kết nối với kênh sau khi tạo.</small></div>
          </header>
          <label>Tên bot<input autoFocus onChange={(event) => {
            setCreateName(event.target.value);
            setCreateSlug((current) => current || slugify(event.target.value));
          }} placeholder="Ví dụ: Server Alert Bot" value={createName} /></label>
          <label>Slug<input onChange={(event) => setCreateSlug(slugify(event.target.value))} placeholder="server-alert-bot" value={createSlug} /></label>
          <label className="bot-create-form__description">Mô tả<textarea onChange={(event) => setCreateDescription(event.target.value)} placeholder="Bot dùng để làm gì?" value={createDescription} /></label>
          <Button disabled={createBotMutation.isPending} size="sm" type="submit">
            {createBotMutation.isPending ? "Đang tạo..." : "Tạo bot"}
          </Button>
        </form>
      ) : null}

      {feedback ? (
        <div className={`bot-feedback bot-feedback--${feedback.tone}`} role="status">
          <span>{feedback.tone === "success" ? <CheckCircle2 size={17} /> : <Info size={17} />}{feedback.message}</span>
          <button aria-label="Đóng thông báo" onClick={() => setFeedback(null)} type="button"><X size={15} /></button>
        </div>
      ) : null}

      {canUseOrder ? (
        <section className="order-bot-panel">
          <header>
            <div>
              <Badge tone={orderConfigured ? "blue" : "red"}>{orderConfigured ? "Đã khai báo API" : "Thiếu API key"}</Badge>
              <h2>VPSTTT Order CSKH</h2>
              <p>Tra ví, tạo QR nạp ví và kiểm tra dịch vụ sắp hết hạn từ hệ thống order.</p>
              {orderConfigured ? <small>Trạng thái này chỉ xác nhận URL và API key đã được nhập; kết nối thật được kiểm tra khi thực hiện tra cứu.</small> : null}
            </div>
            <Button disabled={orderStatusQuery.isFetching} onClick={() => void orderStatusQuery.refetch()} size="sm" variant="secondary">
              <Cloud size={15} /> Tải lại cấu hình
            </Button>
          </header>
          <div className="order-bot-grid">
            <div className="order-bot-card order-bot-card--lookup">
              <strong>Khách hàng</strong>
              <label>Email<input onChange={(event) => setOrderEmail(event.target.value)} placeholder="khach@example.com" value={orderEmail} /></label>
              <label>User ID<input onChange={(event) => setOrderUserId(event.target.value.replace(/\D/g, ""))} placeholder="8075" value={orderUserId} /></label>
              <Button disabled={!orderConfigured || orderBusy || !hasOrderLookup(orderEmail, orderUserId)} onClick={() => orderWalletMutation.mutate()} size="sm" type="button">
                <Search size={15} /> Tra ví
              </Button>
            </div>

            <div className="order-bot-card">
              <strong>Dịch vụ sắp hết hạn</strong>
              <label>Số ngày<input onChange={(event) => setOrderExpiringDays(event.target.value.replace(/\D/g, ""))} value={orderExpiringDays} /></label>
              <label>Loại dịch vụ<select onChange={(event) => setOrderServiceType(event.target.value as OrderServicesExpiringInput["service_type"])} value={orderServiceType}>
                {orderServiceTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select></label>
              <Button disabled={!orderConfigured || orderBusy || !hasOrderLookup(orderEmail, orderUserId)} onClick={() => orderExpiringMutation.mutate()} size="sm" type="button" variant="secondary">
                <Clock3 size={15} /> Gửi Gia Hạn Bot
              </Button>
            </div>

            <div className="order-bot-card">
              <strong>QR nạp ví</strong>
              <label>Số tiền<input onChange={(event) => setOrderDepositAmount(event.target.value.replace(/\D/g, ""))} value={orderDepositAmount} /></label>
              <small>QR mặc định hết hạn sau 24 giờ và gửi vào kênh kế toán.</small>
              <Button disabled={!canBillOrder || !orderConfigured || orderBusy || !orderEmail.trim() || parsePositiveInt(orderDepositAmount) < 1000} onClick={() => orderDepositMutation.mutate()} size="sm" type="button" variant="secondary">
                <FileText size={15} /> Tạo QR
              </Button>
              {!canBillOrder ? <small>Bạn cần quyền order.billing để tạo QR.</small> : null}
            </div>

            <div className="order-bot-card">
              <strong>QR đơn hàng</strong>
              <label>Intent code<input onChange={(event) => setOrderIntentCode(event.target.value)} placeholder="QOIABCD1234EFGH5678" value={orderIntentCode} /></label>
              <small>Tạo lại QR theo đúng số tiền của Quick Order; không nhận số tiền nhập tay.</small>
              <Button disabled={!canBillOrder || !orderStatusQuery.data?.quick_order_configured || orderBusy || orderIntentCode.trim().length < 6} onClick={() => orderPaymentMutation.mutate()} size="sm" type="button" variant="secondary">
                <FileText size={15} /> Tạo QR đơn hàng
              </Button>
              {!orderStatusQuery.data?.quick_order_configured ? <small>Cần cấu hình ORDER_QUICK_ORDER_KEY.</small> : null}
            </div>
          </div>
          {orderResult ? <OrderBotResultView result={orderResult} /> : null}
        </section>
      ) : null}

      {canManage ? (
        <div className="bot-workspace-grid">
          <section className="bot-catalog">
            <header>
              <div><h2>Bot trong workspace</h2><p>Chọn một bot để cấu hình và gửi thử.</p></div>
              <Badge tone={activeBots ? "green" : "slate"}>{activeBots} hoạt động</Badge>
            </header>
            {botsQuery.isLoading ? (
              <div className="bot-card-grid"><Skeleton style={{ height: 150 }} /><Skeleton style={{ height: 150 }} /></div>
            ) : botsQuery.isError ? (
              <ErrorState action={<Button onClick={() => void botsQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>} description="Không tải được danh sách bot." title="Lỗi dữ liệu bot" />
            ) : bots.length ? (
              <div className="bot-card-grid">
                {bots.map((bot) => (
                  <button className={bot.id === selectedBot?.id ? "bot-card bot-card--active" : "bot-card"} key={bot.id} onClick={() => setSelectedBotId(bot.id)} type="button">
                    <span className="bot-card__avatar">{bot.avatar_url ? <img alt="" src={bot.avatar_url} /> : <Bot size={23} />}</span>
                    <span className="bot-card__body"><strong>{bot.name}</strong><small>@{bot.slug}</small><p>{bot.description || "Chưa có mô tả cho bot này."}</p></span>
                    <span className={bot.status === "active" ? "bot-status bot-status--active" : "bot-status"}><i />{bot.status === "active" ? "Hoạt động" : bot.status}</span>
                  </button>
                ))}
              </div>
            ) : (
              <EmptyState action={<Button onClick={() => setIsCreateOpen(true)} size="sm"><Plus size={15} />Tạo bot đầu tiên</Button>} description="Tạo Ticket Bot, Server Alert Bot hoặc Gia Hạn Bot để bắt đầu." title="Chưa có bot" />
            )}
          </section>

          <aside className="bot-control-panel">
            {selectedBot ? (
              <>
                <header>
                  <span className="bot-control-panel__avatar">{selectedBot.avatar_url ? <img alt="" src={selectedBot.avatar_url} /> : <Bot size={26} />}</span>
                  <div><h2>{selectedBot.name}</h2><p>Cập nhật {formatSessionDate(selectedBot.updated_at)}</p></div>
                </header>
                <section>
                  <div className="bot-section-title"><span><Zap size={16} /></span><div><strong>Kết nối kênh</strong><small>{installations.length} cài đặt hiện có</small></div></div>
                  <div className="bot-channel-action">
                    <select aria-label="Chọn kênh cài bot" onChange={(event) => setTargetChannelId(event.target.value)} value={targetChannelId}>
                      {availableChannels.map((channel) => <option key={channel.id} value={channel.id}>#{channel.name}</option>)}
                    </select>
                    <Button disabled={!targetChannelId || installBotMutation.isPending} onClick={() => selectedBot && installBotMutation.mutate({ botId: selectedBot.id, channelId: targetChannelId })} size="sm" variant="secondary">
                      {installBotMutation.isPending ? "Đang nối..." : "Kết nối"}
                    </Button>
                  </div>
                  {!availableChannels.length ? <small>Bạn cần tham gia ít nhất một kênh trước khi cài bot.</small> : null}
                </section>
                <section>
                  <div className="bot-section-title"><span><MessageCircle size={16} /></span><div><strong>Gửi thử tin nhắn</strong><small>Kiểm tra bot trực tiếp trong kênh đã chọn.</small></div></div>
                  <form className="bot-test-form" onSubmit={(event) => {
                    event.preventDefault();
                    if (selectedBot && targetChannelId && testMessage.trim()) {
                      sendBotMessageMutation.mutate({ botId: selectedBot.id, body: testMessage.trim(), channelId: targetChannelId });
                    }
                  }}>
                    <textarea onChange={(event) => setTestMessage(event.target.value)} placeholder="Nhập nội dung bot sẽ gửi..." value={testMessage} />
                    <Button disabled={!targetChannelId || !testMessage.trim() || sendBotMessageMutation.isPending} size="sm" type="submit">
                      <Send size={15} />{sendBotMessageMutation.isPending ? "Đang gửi..." : "Gửi thử"}
                    </Button>
                  </form>
                </section>
              </>
            ) : (
              <EmptyState description="Chọn hoặc tạo một bot để bắt đầu cấu hình." title="Chưa chọn bot" />
            )}
          </aside>
        </div>
      ) : null}
    </div>
  );
}

function OrderBotResultView({ result }: { result: OrderBotResult }) {
  if (result.kind === "wallet") {
    const services = result.data.services ?? {};
    return (
      <div className="order-bot-result">
        <strong>Ví khách hàng</strong>
        <span>{orderCustomerLabel(result.data.name, result.data.email, result.data.user_id)}</span>
        <b>{formatOrderMoney(result.data.balance_vnd ?? result.data.balance ?? result.data.money ?? 0)}</b>
        <small>{formatOrderServiceMap(services) || "Chưa có thống kê dịch vụ."}</small>
      </div>
    );
  }
  if (result.kind === "deposit") {
    const bank = result.data.bank ?? {};
    return (
      <div className="order-bot-result">
        <strong>QR nạp ví</strong>
        <span>{orderCustomerLabel(result.data.name, result.data.email, result.data.user_id)}</span>
        <b>{formatOrderMoney(result.data.amount ?? 0)}</b>
        <small>{result.data.reference ? `Mã: ${result.data.reference}` : "Đã tạo yêu cầu nạp ví."}</small>
        <small>{bank.transfer_content || result.data.transfer_content ? `Nội dung CK: ${bank.transfer_content || result.data.transfer_content}` : null}</small>
        {result.data.qr_url ? <BrandedQRCode alt="Mã QR nạp ví" className="order-bot-result__qr" src={result.data.qr_url} /> : null}
        {result.data.qr_url ? <a href={result.data.qr_url} rel="noreferrer" target="_blank">Mở QR kích thước đầy đủ</a> : null}
      </div>
    );
  }
  if (result.kind === "order-payment") {
    return (
      <div className="order-bot-result">
        <strong>QR thanh toán đơn hàng</strong>
        <span>{result.data.external_order_id || `Intent #${result.data.intent_id ?? "—"}`}</span>
        <b>{formatOrderMoney(result.data.amount ?? 0)}</b>
        <small>{result.data.reference ? `Mã: ${result.data.reference}` : "QR theo số tiền được chốt từ Order."}</small>
        {result.data.qr_url ? <BrandedQRCode alt="Mã QR thanh toán đơn hàng" className="order-bot-result__qr" src={result.data.qr_url} /> : null}
        {result.data.qr_url ? <a href={result.data.qr_url} rel="noreferrer" target="_blank">Mở QR kích thước đầy đủ</a> : null}
      </div>
    );
  }
  const summary = result.data.summary ?? {};
  const items = result.data.items ?? [];
  return (
    <div className="order-bot-result">
      <strong>Dịch vụ sắp hết hạn</strong>
      <span>{orderCustomerLabel(result.data.user?.name, result.data.user?.email, result.data.user?.user_id)}</span>
      <b>{summary.total ?? items.length} dịch vụ</b>
      <small>Hết hạn: {summary.expired ?? 0} · Sắp hết hạn: {summary.expiring ?? 0} · Auto-renew tắt: {summary.auto_renew_off ?? 0}</small>
      {items.slice(0, 3).map((item) => (
        <small key={`${item.service_type_key}-${item.service_id}-${item.expires_at}`}>
          {item.service_type || item.service_type_key || "Dịch vụ"} #{item.service_id} · {item.days_remaining ?? 0} ngày · {item.expires_at || "chưa rõ hạn"}
        </small>
      ))}
    </div>
  );
}

function hasOrderLookup(email: string, userId: string) {
  return Boolean(email.trim() || parsePositiveInt(userId) > 0);
}

function buildOrderLookup(email: string, userId: string) {
  const payload: { email?: string; user_id?: number } = {};
  if (email.trim()) payload.email = email.trim();
  const parsedUserId = parsePositiveInt(userId);
  if (parsedUserId > 0) payload.user_id = parsedUserId;
  return payload;
}

function buildOrderEmail(email: string) {
  return { email: email.trim() };
}

function parsePositiveInt(value: string | number | undefined) {
  const parsed = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

function orderCustomerLabel(name?: string, email?: string, userId?: number) {
  return [name, email, userId ? `#${userId}` : ""].filter(Boolean).join(" · ") || "Không rõ khách hàng";
}

function formatOrderMoney(value: number) {
  return `${Math.round(value).toLocaleString("vi-VN")} VND`;
}

function formatOrderServiceMap(services: Record<string, number>) {
  return Object.entries(services)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key.toUpperCase()} ${value}`)
    .join(" · ");
}

function resizeAvatarFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const source = URL.createObjectURL(file);
    const image = new Image();
    image.onload = () => {
      const maxSize = 320;
      const scale = Math.min(1, maxSize / Math.max(image.naturalWidth, image.naturalHeight));
      const canvas = document.createElement("canvas");
      canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
      canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
      const context = canvas.getContext("2d");
      if (!context) {
        URL.revokeObjectURL(source);
        reject(new Error("Canvas is not available"));
        return;
      }
      context.drawImage(image, 0, 0, canvas.width, canvas.height);
      URL.revokeObjectURL(source);
      resolve(canvas.toDataURL("image/jpeg", 0.86));
    };
    image.onerror = () => {
      URL.revokeObjectURL(source);
      reject(new Error("Image could not be loaded"));
    };
    image.src = source;
  });
}

function OperationalPage({ activeRailItem }: { activeRailItem: RailItemId }) {
  const pageConfig: Partial<Record<RailItemId, { icon: typeof Ticket; title: string }>> = {
    automation: {
      icon: Workflow,
      title: "Automation"
    },
    bots: {
      icon: Bot,
      title: "Bot"
    },
    tickets: {
      icon: Ticket,
      title: "Ticket"
    }
  };
  const config = pageConfig[activeRailItem] ?? {
    icon: Archive,
    title: "Chức năng"
  };
  const Icon = config.icon;

  return (
    <div className="workspace-page">
      <section className="operational-empty">
        <Badge tone="orange">Sắp có</Badge>
        <Icon size={42} />
        <h2>{config.title} đang được hoàn thiện</h2>
        <p>WebTui Chat sẽ mở phần này khi quy trình sử dụng đã sẵn sàng cho người dùng.</p>
      </section>
    </div>
  );
}

function CreateChannelForm({
  isPending,
  onCancel,
  onSubmit
}: {
  isPending: boolean;
  onCancel: () => void;
  onSubmit: (input: CreateChannelPayload) => void;
}) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [type, setType] = useState<"public" | "private">("public");

  function handleNameChange(value: string) {
    setName(value);
    setSlug((current) => current || slugify(value));
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cleanName = name.trim();
    const cleanSlug = slugify(slug || name);

    if (!cleanName || !cleanSlug) {
      return;
    }

    onSubmit({
      description: description.trim(),
      name: cleanName,
      slug: cleanSlug,
      type
    });
  }

  return (
    <form className="channel-create-form" onSubmit={handleSubmit}>
      <Input
        aria-label="Tên kênh"
        onChange={(event) => handleNameChange(event.target.value)}
        placeholder="Tên kênh"
        required
        value={name}
      />
      <Input
        aria-label="Slug kênh"
        onChange={(event) => setSlug(slugify(event.target.value))}
        placeholder="slug-kenh"
        required
        value={slug}
      />
      <Input
        aria-label="Mô tả kênh"
        onChange={(event) => setDescription(event.target.value)}
        placeholder="Mô tả ngắn"
        value={description}
      />
      <div className="channel-create-form__footer">
        <select aria-label="Loại kênh" onChange={(event) => setType(event.target.value as "public" | "private")} value={type}>
          <option value="public">Công khai</option>
          <option value="private">Riêng tư</option>
        </select>
        <Button disabled={isPending} size="sm" type="submit">
          Tạo
        </Button>
        <Button onClick={onCancel} size="sm" variant="ghost">
          Hủy
        </Button>
      </div>
    </form>
  );
}

const quickEmojis = [
  "😀", "😃", "😄", "😁", "😆", "🥹", "😂", "🤣", "😊", "😍", "🥰", "😘",
  "😎", "🤓", "🧐", "🤩", "🥳", "😇", "🙂", "🙃", "😉", "😌", "😋", "🤗",
  "🤔", "🤭", "🫢", "😮", "😲", "😴", "🥱", "😢", "😭", "😤", "😡", "🤯",
  "👍", "👎", "👏", "🙌", "🙏", "🤝", "💪", "✌️", "👌", "🤞", "🫶", "👀",
  "❤️", "🧡", "💛", "💚", "💙", "💜", "🖤", "🤍", "💔", "💯", "✨", "🔥",
  "✅", "❌", "❓", "⚠️", "🎉", "🎊", "🎁", "🏆", "🚀", "💡", "📌", "💬"
];

function EmojiPicker({ onSelect }: { onSelect: (emoji: string) => void }) {
  return (
    <div className="emoji-picker" aria-label="Chọn biểu cảm">
      {quickEmojis.map((emoji) => (
        <button key={emoji} onClick={() => onSelect(emoji)} type="button">
          {emoji}
        </button>
      ))}
    </div>
  );
}

function TypingDots({ label }: { label: string }) {
  return (
    <div className="typing-dots" aria-label={label}>
      <small>{label}</small>
      <span />
      <span />
      <span />
    </div>
  );
}

function ChatHeader({
  callStatus = "idle",
  channel,
  isDetailPanelOpen = false,
  isFavorite = false,
  isMembersLoading = false,
  isSearchOpen,
  members = [],
  mobileFeatureMenu,
  onBack,
  onMarkUnread,
  onStartAudioCall,
  onStartVideoCall,
  onToggleDetailPanel,
  onToggleFavorite,
  onToggleSearch
}: {
  callStatus?: WebRtcCallState["status"];
  channel: ChatChannel;
  isDetailPanelOpen?: boolean;
  isFavorite?: boolean;
  isMembersLoading?: boolean;
  isSearchOpen: boolean;
  members?: ChannelMember[];
  mobileFeatureMenu?: ReactNode;
  onBack?: () => void;
  onMarkUnread?: () => void;
  onStartAudioCall?: () => void;
  onStartVideoCall?: () => void;
  onToggleDetailPanel?: () => void;
  onToggleFavorite?: () => void;
  onToggleSearch: () => void;
}) {
  const [openPopover, setOpenPopover] = useState<"members" | "more" | null>(null);
  const canShowCallActions = Boolean(onStartAudioCall || onStartVideoCall);
  const callDisabled = !onStartAudioCall || (callStatus !== "idle" && callStatus !== "ended" && callStatus !== "error");

  useEffect(() => setOpenPopover(null), [channel.id]);

  return (
    <header className="chat-header">
      {onBack ? (
        <Button aria-label="Quay lại danh sách hội thoại" className="chat-header__mobile-back" onClick={onBack} variant="icon">
          <ChevronLeft size={22} />
        </Button>
      ) : null}
      <div className="chat-title">
        {channel.type === "direct" ? (
          <Avatar name={channel.name} size="lg" src={channel.avatarUrl} status={channel.userStatus} />
        ) : (
          <span className={`channel-hash channel-hash--${channel.tone}`} style={channelHashStyle(channel)}>#</span>
        )}
        <div>
          <h1>{channel.name}</h1>
          <p>{channel.description}</p>
        </div>
      </div>
      <div className="chat-actions">
        <div className="chat-header-control">
          <Tooltip label="Thành viên">
            <Button
              aria-expanded={openPopover === "members"}
              aria-label={`Xem ${channel.memberCount} thành viên`}
              className={openPopover === "members" ? "member-count chat-action-active" : "member-count"}
              onClick={() => setOpenPopover((current) => current === "members" ? null : "members")}
              size="sm"
              variant="ghost"
            >
              <Users size={18} /> {channel.memberCount}
            </Button>
          </Tooltip>
          {openPopover === "members" ? (
            <div className="chat-header-popover chat-members-popover" role="dialog" aria-label="Danh sách thành viên">
              <header>
                <div>
                  <strong>Thành viên</strong>
                  <small>{channel.memberCount} người trong cuộc trò chuyện</small>
                </div>
                <button aria-label="Đóng danh sách thành viên" onClick={() => setOpenPopover(null)} type="button"><X size={16} /></button>
              </header>
              <div className="chat-members-list">
                {isMembersLoading ? <Skeleton style={{ height: 84 }} /> : members.length ? members.map((member) => {
                  const name = member.display_name || member.username || member.email || "Thành viên";
                  return (
                    <article key={member.user_id}>
                      <Avatar name={name} size="sm" src={member.avatar_url ?? undefined} />
                      <span>
                        <strong>{name}</strong>
                        <small>{member.status === "active" ? "Đang hoạt động" : member.status || "Thành viên"}</small>
                      </span>
                    </article>
                  );
                }) : <p>Chưa tải được danh sách thành viên.</p>}
              </div>
            </div>
          ) : null}
        </div>
        <Tooltip label="Tìm kiếm">
          <Button
            aria-label={isSearchOpen ? "Đóng tìm kiếm" : "Tìm kiếm"}
            className={isSearchOpen ? "chat-action-active" : undefined}
            onClick={() => {
              setOpenPopover(null);
              onToggleSearch();
            }}
            type="button"
            variant="icon"
          >
            <Search size={19} />
          </Button>
        </Tooltip>
        {canShowCallActions ? (
          <>
            <Tooltip className="chat-call-action" label={callDisabled ? "Cuộc gọi đang diễn ra" : "Gọi thoại"}>
              <Button
                aria-label="Gọi thoại"
                disabled={callDisabled}
                onClick={() => {
                  setOpenPopover(null);
                  onStartAudioCall?.();
                }}
                type="button"
                variant="icon"
              >
                <Phone size={19} />
              </Button>
            </Tooltip>
            <Tooltip className="chat-call-action" label={callDisabled ? "Cuộc gọi đang diễn ra" : "Gọi video"}>
              <Button
                aria-label="Gọi video"
                disabled={callDisabled || !onStartVideoCall}
                onClick={() => {
                  setOpenPopover(null);
                  onStartVideoCall?.();
                }}
                type="button"
                variant="icon"
              >
                <Video size={19} />
              </Button>
            </Tooltip>
          </>
        ) : null}
        <Tooltip className="chat-detail-action" label={isDetailPanelOpen ? "Ẩn thông tin cuộc trò chuyện" : "Hiện thông tin cuộc trò chuyện"}>
          <Button
            aria-label={isDetailPanelOpen ? "Ẩn thông tin cuộc trò chuyện" : "Hiện thông tin cuộc trò chuyện"}
            className={isDetailPanelOpen ? "chat-action-active" : undefined}
            disabled={!onToggleDetailPanel}
            onClick={() => {
              setOpenPopover(null);
              onToggleDetailPanel?.();
            }}
            variant="icon"
          >
            <Info size={19} />
          </Button>
        </Tooltip>
        <div className="chat-header-control">
          <Tooltip label="Tùy chọn khác">
            <Button
              aria-expanded={openPopover === "more"}
              aria-label="Tùy chọn khác"
              className={openPopover === "more" ? "chat-action-active" : undefined}
              onClick={() => setOpenPopover((current) => current === "more" ? null : "more")}
              variant="icon"
            >
              <MoreVertical size={19} />
            </Button>
          </Tooltip>
          {openPopover === "more" ? (
            <div className="chat-header-popover chat-more-menu" role="menu">
              {onToggleDetailPanel ? (
                <button
                  className="chat-more-menu__mobile-action"
                  onClick={() => { onToggleDetailPanel(); setOpenPopover(null); }}
                  role="menuitem"
                  type="button"
                >
                  <Info size={17} /> {isDetailPanelOpen ? "Ẩn thông tin" : "Thông tin cuộc trò chuyện"}
                </button>
              ) : null}
              <button onClick={() => { onToggleFavorite?.(); setOpenPopover(null); }} role="menuitem" type="button">
                <Star fill={isFavorite ? "currentColor" : "none"} size={17} />
                {isFavorite ? "Bỏ khỏi yêu thích" : "Thêm vào yêu thích"}
              </button>
              <button onClick={() => { onMarkUnread?.(); setOpenPopover(null); }} role="menuitem" type="button">
                <MessageCircle size={17} />
                Đánh dấu chưa đọc
              </button>
            </div>
          ) : null}
        </div>
        {!isDetailPanelOpen && onToggleDetailPanel ? (
          <Tooltip label="Mở bảng thông tin">
            <Button
              aria-label="Mở bảng thông tin cuộc trò chuyện"
              className="chat-panel-open-button"
              onClick={() => {
                setOpenPopover(null);
                onToggleDetailPanel();
              }}
              variant="icon"
            >
              <PanelRightOpen size={19} />
            </Button>
          </Tooltip>
        ) : null}
        {mobileFeatureMenu}
      </div>
    </header>
  );
}

function useIncomingCallRingtone(active: boolean) {
  const audioContextRef = useRef<AudioContext | null>(null);

  useEffect(() => {
    if (!active || typeof window === "undefined") {
      return undefined;
    }

    const audioWindow = window as Window & {
      AudioContext?: typeof AudioContext;
      webkitAudioContext?: typeof AudioContext;
    };
    const AudioContextCtor = audioWindow.AudioContext ?? audioWindow.webkitAudioContext;
    if (!AudioContextCtor) {
      return undefined;
    }

    const context = audioContextRef.current ?? new AudioContextCtor();
    audioContextRef.current = context;
    let disposed = false;

    const ring = () => {
      if (disposed) {
        return;
      }
      void context.resume().then(() => {
        const oscillator = context.createOscillator();
        const gain = context.createGain();
        oscillator.type = "sine";
        oscillator.frequency.setValueAtTime(880, context.currentTime);
        oscillator.frequency.setValueAtTime(660, context.currentTime + 0.18);
        gain.gain.setValueAtTime(0.0001, context.currentTime);
        gain.gain.exponentialRampToValueAtTime(0.16, context.currentTime + 0.04);
        gain.gain.exponentialRampToValueAtTime(0.0001, context.currentTime + 0.48);
        oscillator.connect(gain);
        gain.connect(context.destination);
        oscillator.start();
        oscillator.stop(context.currentTime + 0.5);
      }).catch(() => undefined);
    };

    ring();
    const interval = window.setInterval(ring, 1600);
    return () => {
      disposed = true;
      window.clearInterval(interval);
    };
  }, [active]);

  useEffect(() => () => {
    void audioContextRef.current?.close().catch(() => undefined);
    audioContextRef.current = null;
  }, []);
}

function CallPanel({
  callState,
  hasMediaSession,
  mediaContainerRef,
  onAccept,
  onEnd,
  onReject
}: {
  callState: WebRtcCallState;
  hasMediaSession: boolean;
  mediaContainerRef: RefObject<HTMLDivElement | null>;
  onAccept: () => void;
  onEnd: () => void;
  onReject: () => void;
}) {
  const [minimized, setMinimized] = useState(false);

  useEffect(() => {
    if (callState.status === "incoming" || callState.status === "idle") {
      setMinimized(false);
    }
  }, [callState.status]);

  if (callState.status === "idle") {
    return null;
  }

  const canControlCall = callState.status === "outgoing" || callState.status === "connecting" || callState.status === "active";
  const isVideo = callState.mode === "video";
  const showMediaSession = canControlCall;

  return (
    <section className={`call-panel call-panel--floating call-panel--${callState.status}${minimized ? " call-panel--minimized" : ""}`} role="status">
      <div className="call-panel__summary">
        <span className="call-panel__badge">
          {isVideo ? <Video size={18} /> : <Phone size={18} />}
        </span>
        <div>
          <strong>{callPanelTitle(callState)}</strong>
          <small>{callPanelSubtitle(callState)}</small>
        </div>
      </div>
      {showMediaSession && !minimized ? (
        <div className={`call-panel__media call-panel__media--${callState.mode}`}>
          <div
            className={hasMediaSession ? "webtui-webrtc-call" : "webtui-webrtc-call webtui-webrtc-call--loading"}
            ref={mediaContainerRef}
          />
        </div>
      ) : null}
      <div className="call-panel__actions">
        {showMediaSession ? (
          <Tooltip label={minimized ? "Mở cửa sổ cuộc gọi" : "Thu nhỏ cuộc gọi"}>
            <Button
              aria-label={minimized ? "Mở cửa sổ cuộc gọi" : "Thu nhỏ cuộc gọi"}
              className="call-panel__icon-button"
              onClick={() => setMinimized((value) => !value)}
              size="sm"
              variant="secondary"
            >
              <Minimize2 size={16} />
            </Button>
          </Tooltip>
        ) : null}
        {callState.status === "incoming" ? (
          <>
            <Button className="call-panel__button call-panel__button--accept" onClick={onAccept} size="sm" variant="secondary">
              <Phone size={16} />
              Nhận
            </Button>
            <Button className="call-panel__button call-panel__button--end" onClick={onReject} size="sm" variant="secondary">
              <PhoneOff size={16} />
              Từ chối
            </Button>
          </>
        ) : null}
        {showMediaSession ? (
          <Button className="call-panel__button call-panel__button--end" onClick={onEnd} size="sm" variant="secondary">
            <PhoneOff size={16} />
            Kết thúc
          </Button>
        ) : null}
      </div>
    </section>
  );
}

function callPanelTitle(callState: WebRtcCallState): string {
  if (callState.status === "incoming") {
    return "Cuộc gọi đến";
  }
  if (callState.status === "outgoing") {
    return "Đang gọi";
  }
  if (callState.status === "connecting") {
    return "Đang kết nối";
  }
  if (callState.status === "active") {
    return "Đang trong cuộc gọi";
  }
  if (callState.status === "error") {
    return "Không thể thực hiện cuộc gọi";
  }
  return "Cuộc gọi đã kết thúc";
}

function callPanelSubtitle(callState: WebRtcCallState): string {
  if (callState.error) {
    return callState.error;
  }
  const modeLabel = callState.mode === "video" ? "Video" : "Thoại";
  return [modeLabel, callState.peerName].filter(Boolean).join(" - ");
}

function ChannelAccessView({
  channel,
  isPending,
  mobileFeatureMenu,
  onBack,
  onRequestJoin
}: {
  channel: ChatChannel;
  isPending: boolean;
  mobileFeatureMenu?: ReactNode;
  onBack?: () => void;
  onRequestJoin: () => void;
}) {
  const isWaiting = channel.membershipStatus === "invited";

  return (
    <div className="channel-access-view">
      <ChatHeader channel={channel} isSearchOpen={false} mobileFeatureMenu={mobileFeatureMenu} onBack={onBack} onToggleSearch={() => undefined} />
      <section>
        <span><ShieldCheck size={30} /></span>
        <h2>{isWaiting ? "Yêu cầu đang chờ phê duyệt" : "Bạn chưa tham gia kênh này"}</h2>
        <p>
          {isWaiting
            ? "Chủ kênh cần phê duyệt trước khi bạn có thể xem và gửi tin nhắn."
            : "Nội dung kênh chỉ hiển thị cho thành viên đã được chủ kênh phê duyệt."}
        </p>
        <Button disabled={isPending || isWaiting} onClick={onRequestJoin}>
          {isPending ? "Đang gửi..." : isWaiting ? "Đang chờ duyệt" : "Yêu cầu tham gia kênh"}
        </Button>
      </section>
    </div>
  );
}

function NotificationDropdown({
  contactRequests,
  isLoading,
  isMutatingContactRequest,
  isMarkingAllRead,
  notifications,
  onAcceptContactRequest,
  onMarkAllRead,
  onOpenContacts,
  onOpenNotification,
  onRejectContactRequest
}: {
  contactRequests: ContactRequest[];
  isLoading: boolean;
  isMutatingContactRequest: boolean;
  isMarkingAllRead: boolean;
  notifications: NotificationItem[];
  onAcceptContactRequest: (request: ContactRequest) => void;
  onMarkAllRead: () => void;
  onOpenContacts: () => void;
  onOpenNotification: (notification: NotificationItem) => void;
  onRejectContactRequest: (request: ContactRequest) => void;
}) {
  const unreadNotificationCount = notifications.filter((item) => !item.isRead).length;
  const totalUnreadCount = unreadNotificationCount + contactRequests.length;

  return (
    <section className="notification-dropdown" aria-label="Thông báo">
      <header className="notification-dropdown__header">
        <div className="notification-dropdown__title">
          <span><Bell size={18} /></span>
          <div>
            <strong>Thông báo</strong>
          </div>
          {totalUnreadCount ? <b>{totalUnreadCount}</b> : null}
        </div>
        <Button disabled={isMarkingAllRead || !unreadNotificationCount} onClick={onMarkAllRead} size="sm" variant="ghost">
          {isMarkingAllRead ? "Đang xử lý..." : "Đọc tất cả"}
        </Button>
      </header>
      <div className="notification-dropdown__body">
        {contactRequests.length ? (
          <section className="notification-group">
            <header><span><Users size={15} /> Lời mời kết bạn</span><b>{contactRequests.length}</b></header>
            <div className="notification-list">
              {contactRequests.map((request) => (
                <article className="notification-row notification-row--unread contact-request-row" key={request.id}>
                  <Avatar
                    name={request.user.display_name || request.user.username || request.user.email || "Người dùng"}
                    size="sm"
                    src={request.user.avatar_url ?? undefined}
                  />
                  <div>
                    <strong>{request.user.display_name || request.user.username || request.user.email || "Người dùng"}</strong>
                    <p>Muốn kết nối và nhắn tin với bạn.</p>
                    <small>{request.user.email}</small>
                    <div className="contact-request-row__actions">
                      <Button disabled={isMutatingContactRequest} onClick={() => onAcceptContactRequest(request)} size="sm" variant="primary">
                        Đồng ý
                      </Button>
                      <Button disabled={isMutatingContactRequest} onClick={() => onRejectContactRequest(request)} size="sm" variant="ghost">
                        Từ chối
                      </Button>
                    </div>
                  </div>
                </article>
              ))}
            </div>
            <button className="notification-group__link" onClick={onOpenContacts} type="button">Mở danh bạ</button>
          </section>
        ) : null}
        {isLoading ? (
          <PanelSkeleton />
        ) : notifications.length ? (
          <section className="notification-group">
            <header><span><Bell size={15} /> Hoạt động gần đây</span></header>
            <div className="notification-list">
              {notifications.map((notification) => (
                <button
                  className={notification.isRead ? "notification-row" : "notification-row notification-row--unread"}
                  key={notification.id}
                  onClick={() => onOpenNotification(notification)}
                  type="button"
                >
                  <span className="notification-row__icon"><Bell size={16} /></span>
                  <div>
                    <strong>{notification.title}</strong>
                    <p>{notification.body}</p>
                    <small>{notification.createdAt}</small>
                  </div>
                  {!notification.isRead ? <i aria-label="Chưa đọc" /> : null}
                </button>
              ))}
            </div>
          </section>
        ) : !contactRequests.length ? (
          <EmptyState description="Thông báo mới sẽ xuất hiện tại đây." title="Bạn đã xem hết" />
        ) : null}
      </div>
    </section>
  );
}

function MessageNotificationToast({
  notification,
  onClose,
  onOpen
}: {
  notification: NotificationItem;
  onClose: () => void;
  onOpen: (notification: NotificationItem) => void;
}) {
  return (
    <section aria-live="polite" className="message-notice-toast" role="status">
      <button className="message-notice-toast__body" onClick={() => onOpen(notification)} type="button">
        <span className="message-notice-toast__icon"><MessageCircle size={18} /></span>
        <span className="message-notice-toast__copy">
          <strong>{notification.title}</strong>
          <small>{notification.body}</small>
        </span>
      </button>
      <button aria-label="Đóng thông báo" className="message-notice-toast__close" onClick={onClose} type="button">
        <X size={14} />
      </button>
    </section>
  );
}

function ReplyComposerPreview({
  onCancel,
  preview
}: {
  onCancel: () => void;
  preview: MessageReplyPreview;
}) {
  return (
    <section className="composer-reply-preview" role="status">
      <span className="composer-reply-preview__mark"><Reply size={16} /></span>
      <span className="composer-reply-preview__copy">
        <strong>Trả lời {preview.authorName}</strong>
        <small>{preview.body}</small>
      </span>
      <button aria-label="Hủy trả lời" onClick={onCancel} type="button">
        <X size={15} />
      </button>
    </section>
  );
}

function ReplyQuotePreview({
  onOpen,
  preview
}: {
  onOpen?: () => void;
  preview: MessageReplyPreview;
}) {
  const content = (
    <>
      <strong>{preview.authorName}</strong>
      <small>{preview.body}</small>
    </>
  );

  return onOpen ? (
    <button className="message-reply-preview" onClick={onOpen} type="button">
      {content}
    </button>
  ) : (
    <div className="message-reply-preview">
      {content}
    </div>
  );
}

function UploadQueue({
  disabled,
  items,
  onRemove,
  onRetry
}: {
  disabled: boolean;
  items: UploadQueueItem[];
  onRemove: (id: string) => void;
  onRetry: (id: string) => void;
}) {
  const labels: Record<UploadQueueItem["status"], string> = {
    attached: "Đã gắn",
    failed: "Lỗi",
    queued: "Chờ gửi",
    uploading: "Đang tải"
  };

  const hasImages = items.some((item) => item.isImage);

  return (
    <div className={hasImages ? "upload-queue upload-queue--media" : "upload-queue"} aria-label="Hàng đợi upload">
      {items.map((item) => {
        const kindClassName = item.isImage
          ? "upload-queue__item--image"
          : item.isAudio
            ? "upload-queue__item--audio"
            : "upload-queue__item--file";
        const statusLabel = item.error ?? labels[item.status];

        return (
          <article
            aria-busy={item.status === "uploading"}
            className={`upload-queue__item upload-queue__item--${item.status} ${kindClassName}`}
            key={item.id}
          >
            {item.isImage && item.previewUrl ? (
              <div className="upload-queue__preview upload-queue__preview--image">
                <img alt={item.name} className="upload-queue__thumb" src={item.previewUrl} />
                <span className="upload-queue__status visually-hidden" role="status">{statusLabel}</span>
              </div>
            ) : item.isAudio && item.previewUrl ? (
              <div className="upload-queue__preview upload-queue__preview--audio">
                <VoiceMessagePlayer id={`preview-${item.id}`} source={item.previewUrl} />
              </div>
            ) : (
              <span className="upload-queue__icon upload-queue__file-preview" aria-hidden="true">
                <FileText size={22} />
                <small className="upload-queue__file-extension">{fileExtensionLabel(item.name, item.file.type)}</small>
              </span>
            )}
            {!item.isImage ? (
              <div className="upload-queue__details">
                <strong className="upload-queue__name" title={item.name}>{item.isAudio ? "Tin nhắn thoại" : item.name}</strong>
                <small className="upload-queue__meta">
                  {item.durationSeconds ? `${formatVoiceTime(item.durationSeconds)} · ` : ""}{statusLabel}
                </small>
                {item.status === "uploading" ? <i aria-hidden="true" className="upload-queue__progress" /> : null}
              </div>
            ) : null}
            {!item.isImage && item.status === "failed" ? (
              <button className="upload-queue__retry" disabled={disabled} onClick={() => onRetry(item.id)} type="button">
                Thử lại
              </button>
            ) : null}
            {item.isImage || item.status === "queued" || item.status === "failed" ? (
              <button
                aria-label={`Xóa ${item.name}`}
                className="upload-queue__remove"
                disabled={disabled}
                onClick={() => onRemove(item.id)}
                title="Xóa file"
                type="button"
              >
                <X size={15} />
              </button>
            ) : null}
          </article>
        );
      })}
    </div>
  );
}

function ForwardMessageDialog({
  channels,
  isPending,
  onCancel,
  onSubmit
}: {
  channels: Array<{ id: string; name: string }>;
  isPending: boolean;
  onCancel: () => void;
  onSubmit: (channelId: string) => void;
}) {
  const [channelId, setChannelId] = useState(channels[0]?.id ?? "");

  return (
    <div className="forward-dialog-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onCancel()} role="presentation">
      <section aria-labelledby="forward-dialog-title" aria-modal="true" className="forward-dialog" role="dialog">
        <div>
          <h2 id="forward-dialog-title">Chuyển tiếp tin nhắn</h2>
          <p>Chọn cuộc trò chuyện hoặc kênh mà bạn đang là thành viên.</p>
        </div>
        <label>
          Nơi nhận
          <select autoFocus onChange={(event) => setChannelId(event.target.value)} value={channelId}>
            {channels.map((channel) => <option key={channel.id} value={channel.id}>{channel.name}</option>)}
          </select>
        </label>
        {!channels.length ? <p>Bạn chưa có kênh đích phù hợp.</p> : null}
        <div className="forward-dialog__actions">
          <Button disabled={isPending} onClick={onCancel} variant="secondary">Hủy</Button>
          <Button disabled={isPending || !channelId} onClick={() => onSubmit(channelId)}>
            {isPending ? "Đang chuyển..." : "Chuyển tiếp"}
          </Button>
        </div>
      </section>
    </div>
  );
}

function ImagePreviewDialog({
  attachment,
  onClose,
  onDownload,
  source
}: {
  attachment: MessageAttachmentItem;
  onClose: () => void;
  onDownload: () => void;
  source: string;
}) {
  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [onClose]);

  return (
    <div className="image-preview-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onClose()} role="presentation">
      <section aria-label={`Xem ảnh ${attachment.name}`} aria-modal="true" className="image-preview-dialog" role="dialog">
        <header>
          <div>
            <strong>{attachment.name}</strong>
            {attachment.size ? <small>{attachment.size}</small> : null}
          </div>
          <div>
            <Button onClick={onDownload} size="sm" type="button" variant="secondary">
              <Cloud size={15} /> Tải ảnh
            </Button>
            <Button aria-label="Đóng xem ảnh" onClick={onClose} type="button" variant="icon">
              <X size={19} />
            </Button>
          </div>
        </header>
        <div className="image-preview-dialog__stage">
          <img alt={attachment.name} src={source} />
        </div>
      </section>
    </div>
  );
}

function MessageTimeline({
  currentUserId,
  editingBody,
  editingMessageId,
  focusedMessageId,
  hasOlderMessages,
  isEditingPending,
  isLoadingOlderMessages,
  messages,
  onCancelEdit,
  onChangeEditingBody,
  onDeleteMessage,
  onDownloadAttachment,
  onForwardMessage,
  onPreviewAttachment,
  onResolveAttachment,
  onLoadOlderMessages,
  onOpenThread,
  onReplyMessage,
  onFocusedMessageSettled,
  onRetryCall,
  onSearchResultSelect,
  onStartEdit,
  onSubmitEdit,
  onTogglePin,
  onToggleReaction,
  pinnedMessageIds,
  readMembers,
  searchQuery,
  searchResults,
  showAuthorName,
  timelineId,
  workspaceMembers
}: {
  currentUserId: string;
  editingBody: string;
  editingMessageId: string | null;
  focusedMessageId: string | null;
  hasOlderMessages: boolean;
  isEditingPending: boolean;
  isLoadingOlderMessages: boolean;
  messages: ChatMessage[];
  onCancelEdit: () => void;
  onChangeEditingBody: (value: string) => void;
  onDeleteMessage: (message: ChatMessage) => void;
  onDownloadAttachment: (attachment: MessageAttachmentItem) => void;
  onForwardMessage: (messageId: string) => void;
  onPreviewAttachment: (attachment: MessageAttachmentItem, source?: string) => void;
  onResolveAttachment: (fileId: string) => Promise<Blob>;
  onLoadOlderMessages: () => Promise<unknown> | void;
  onOpenThread: (messageId: string) => void;
  onReplyMessage: (message: ChatMessage, author: ChatUser) => void;
  onFocusedMessageSettled: () => void;
  onRetryCall: (mode: "audio" | "video") => void;
  onSearchResultSelect: (message: ChatMessage) => void;
  onStartEdit: (message: ChatMessage) => void;
  onSubmitEdit: (event: FormEvent<HTMLFormElement>) => void;
  onTogglePin: (message: ChatMessage, isPinned: boolean) => void;
  onToggleReaction: (message: ChatMessage, emoji: string) => void;
  pinnedMessageIds: Set<string>;
  readMembers: ChannelMember[];
  searchQuery: string;
  searchResults: ChatMessage[];
  showAuthorName: boolean;
  timelineId: string;
  workspaceMembers: WorkspaceMember[];
}) {
  const lastMessageId = messages.at(-1)?.id;
  const timelineRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const olderLoadRequestedRef = useRef(false);
  const olderScrollAnchorRef = useRef<number | null>(null);
  const lastAutoScrollMessageIdRef = useRef<string | undefined>(undefined);
  const timelineIdRef = useRef<string | undefined>(undefined);
  const [reactionPickerMessageId, setReactionPickerMessageId] = useState<string | null>(null);
  const [actionMenuMessageId, setActionMenuMessageId] = useState<string | null>(null);
  const memberByUserId = useMemo(
    () => buildMessageAuthorLookup(workspaceMembers, readMembers),
    [readMembers, workspaceMembers]
  );
  const replyPreviewById = useMemo(
    () =>
      new Map(
        messages.map((message) => [
          message.id,
          createMessageReplyPreview(message, resolveRenderedMessageAuthor(message, memberByUserId))
        ])
      ),
    [memberByUserId, messages]
  );
  const messageOrderById = useMemo(() => new Map(messages.map((message, index) => [message.id, index])), [messages]);
  const lastOwnReceipt = useMemo(
    () => resolveLastOwnMessageReceipt(messages, readMembers, currentUserId, messageOrderById),
    [currentUserId, messageOrderById, messages, readMembers]
  );

  const jumpTimelineToBottom = useCallback((timeline: HTMLDivElement) => {
    const previousScrollBehavior = timeline.style.scrollBehavior;
    timeline.style.scrollBehavior = "auto";
    timeline.scrollTop = timeline.scrollHeight;
    timeline.style.scrollBehavior = previousScrollBehavior;
  }, []);

  useLayoutEffect(() => {
    const timeline = timelineRef.current;
    if (!timeline || !lastMessageId || timelineIdRef.current === timelineId) {
      return;
    }

    timelineIdRef.current = timelineId;
    lastAutoScrollMessageIdRef.current = lastMessageId;
    jumpTimelineToBottom(timeline);
    const frame = window.requestAnimationFrame(() => jumpTimelineToBottom(timeline));
    return () => window.cancelAnimationFrame(frame);
  }, [jumpTimelineToBottom, lastMessageId, timelineId]);

  useEffect(() => {
    if (!actionMenuMessageId && !reactionPickerMessageId) {
      return undefined;
    }

    const closeOverlaysOnOutsidePress = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest(".message-actions, .message-reaction-control")) {
        return;
      }
      setActionMenuMessageId(null);
      setReactionPickerMessageId(null);
    };
    const closeOverlaysOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setActionMenuMessageId(null);
        setReactionPickerMessageId(null);
      }
    };

    document.addEventListener("pointerdown", closeOverlaysOnOutsidePress);
    document.addEventListener("keydown", closeOverlaysOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOverlaysOnOutsidePress);
      document.removeEventListener("keydown", closeOverlaysOnEscape);
    };
  }, [actionMenuMessageId, reactionPickerMessageId]);

  useEffect(() => {
    const isNewLastMessage = Boolean(lastMessageId && lastAutoScrollMessageIdRef.current !== lastMessageId);
    lastAutoScrollMessageIdRef.current = lastMessageId;
    if (!lastMessageId || !isNewLastMessage || focusedMessageId) {
      return;
    }

    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
    const timeout = window.setTimeout(() => {
      const timeline = timelineRef.current;
      if (timeline) {
        timeline.scrollTop = timeline.scrollHeight;
      }
    }, 120);

    return () => window.clearTimeout(timeout);
  }, [focusedMessageId, lastMessageId]);

  useEffect(() => {
    if (!isLoadingOlderMessages) {
      olderLoadRequestedRef.current = false;
    }
  }, [isLoadingOlderMessages]);

  useLayoutEffect(() => {
    const scrollAnchor = olderScrollAnchorRef.current;
    const timeline = timelineRef.current;

    if (isLoadingOlderMessages || scrollAnchor === null || !timeline) {
      return;
    }

    timeline.scrollTop = Math.max(0, timeline.scrollHeight - scrollAnchor);
    olderScrollAnchorRef.current = null;
  }, [isLoadingOlderMessages, messages.length]);

  useEffect(() => {
    if (!focusedMessageId) {
      return;
    }
    const target = timelineRef.current?.querySelector<HTMLElement>(`[data-message-id="${cssEscape(focusedMessageId)}"]`);
    if (!target) {
      return;
    }
    target.scrollIntoView({ behavior: "smooth", block: "center" });
    const timeout = window.setTimeout(onFocusedMessageSettled, 1_600);
    return () => window.clearTimeout(timeout);
  }, [focusedMessageId, messages, onFocusedMessageSettled]);

  function requestOlderMessages(source?: HTMLDivElement | null) {
    if (!hasOlderMessages || isLoadingOlderMessages || olderLoadRequestedRef.current) {
      return;
    }

    const timeline = source ?? timelineRef.current;
    olderLoadRequestedRef.current = true;
    olderScrollAnchorRef.current = timeline ? timeline.scrollHeight - timeline.scrollTop : null;
    const result = onLoadOlderMessages();
    if (result && typeof (result as PromiseLike<unknown>).then === "function") {
      void (result as PromiseLike<unknown>).then(
        () => undefined,
        () => {
          olderLoadRequestedRef.current = false;
          olderScrollAnchorRef.current = null;
        }
      );
    }
  }

  function handleTimelineScroll(event: UIEvent<HTMLDivElement>) {
    if (event.currentTarget.scrollTop <= 96) {
      requestOlderMessages(event.currentTarget);
    }
  }

  if (!messages.length) {
    return (
      <div className="message-timeline" onScroll={handleTimelineScroll} ref={timelineRef}>
        {hasOlderMessages ? (
          <Button disabled={isLoadingOlderMessages} onClick={() => requestOlderMessages()} size="sm" variant="secondary">
            {isLoadingOlderMessages ? "Đang tải..." : "Tải tin nhắn cũ"}
          </Button>
        ) : null}
        <EmptyState description="Tin nhắn mới sẽ xuất hiện tại đây." title="Kênh này chưa có tin nhắn" />
      </div>
    );
  }

  return (
    <div className="message-timeline" onScroll={handleTimelineScroll} ref={timelineRef}>
      {hasOlderMessages ? (
        <Button className="load-older-button" disabled={isLoadingOlderMessages} onClick={() => requestOlderMessages()} size="sm" variant="secondary">
          {isLoadingOlderMessages ? "Đang tải..." : "Tải tin nhắn cũ"}
        </Button>
      ) : null}

      {searchQuery.trim().length >= 2 ? (
        <section className="message-search-results">
          <header>
            <strong>Kết quả tìm kiếm</strong>
            <span>{searchResults.length} tin nhắn</span>
          </header>
          {searchResults.length ? (
            searchResults.map((message) => {
              const messageAuthor = resolveRenderedMessageAuthor(message, memberByUserId);
              return (
                <button key={message.id} onClick={() => onSearchResultSelect(message)} type="button">
                  <Avatar name={messageAuthor.name} size="sm" src={messageAuthor.avatarUrl} />
                  <span>
                    <strong>{messageAuthor.name}</strong>
                    <small>{message.body}</small>
                  </span>
                </button>
              );
            })
          ) : (
            <EmptyState description="Không có tin nhắn nào khớp từ khóa hiện tại." title="Không tìm thấy" />
          )}
        </section>
      ) : null}

      {messages.map((message) => {
        const hasReactions = Boolean(message.reactions?.length);
        const messageAuthor = resolveRenderedMessageAuthor(message, memberByUserId);
        const replyPreview = message.parentId ? replyPreviewById.get(message.parentId) ?? message.replyTo : message.replyTo;
        if (message.isSystem) {
          const SystemIcon = message.systemTone === "announcement" ? Bell : Info;
          return (
            <article
              className={`message-system-row message-system-row--${message.systemTone ?? "system"}${focusedMessageId === message.id ? " message-system-row--focused" : ""}`}
              data-message-id={message.id}
              key={message.id}
            >
              <span className="message-system-row__icon"><SystemIcon size={15} /></span>
              <div className="message-system-row__body">
                <strong>{message.systemTone === "announcement" ? "Thông báo" : "Hệ thống"}</strong>
                <MessageBody body={message.body} />
              </div>
              <time>{message.sentAt}</time>
            </article>
          );
        }
        if (message.callEvent) {
          return (
            <CallMessageRow
              focused={focusedMessageId === message.id}
              key={message.id}
              message={message}
              messageAuthor={messageAuthor}
              onRetryCall={onRetryCall}
            />
          );
        }
        const showMessageHeader = showAuthorName || message.isBot || message.isForwarded || Boolean(message.editedAt) || message.isPending;
        return (
        <article
          className={`${message.isMine ? "message-row message-row--local" : "message-row"}${isImageOnlyMessage(message) ? " message-row--media-only" : ""}${isFileOnlyMessage(message) ? " message-row--file-only" : ""}${hasReactions ? " message-row--has-reactions" : ""}${focusedMessageId === message.id ? " message-row--focused" : ""}`}
          data-message-id={message.id}
          key={message.id}
          tabIndex={0}
        >
          {!message.isMine ? <Avatar name={messageAuthor.name} src={messageAuthor.avatarUrl} status={messageAuthor.status} /> : null}
          <div className="message-row__stack">
            <div className="message-row__content">
              {showMessageHeader ? (
                <header>
                  {showAuthorName ? <strong>{messageAuthor.name}</strong> : null}
                  {message.isBot ? <Badge tone="blue">BOT</Badge> : null}
                  {message.isForwarded ? <span>Đã chuyển tiếp</span> : null}
                  {message.editedAt ? <span>Đã sửa {message.editedAt}</span> : null}
                  {message.isPending ? <Badge tone="blue">Đang gửi</Badge> : null}
                </header>
              ) : null}
              <div className={actionMenuMessageId === message.id ? "message-actions message-actions--open" : "message-actions"}>
                <Tooltip label={actionMenuMessageId === message.id ? "Đóng thao tác" : "Thao tác tin nhắn"}>
                  <button
                    aria-expanded={actionMenuMessageId === message.id}
                    aria-haspopup="menu"
                    aria-label="Thao tác tin nhắn"
                    className="message-actions__trigger"
                    onClick={() => {
                      setReactionPickerMessageId(null);
                      setActionMenuMessageId((current) => current === message.id ? null : message.id);
                    }}
                    type="button"
                  >
                    <MoreVertical size={17} />
                  </button>
                </Tooltip>
                {actionMenuMessageId === message.id ? (
                  <div aria-label="Các thao tác với tin nhắn" className="message-actions__menu" role="menu">
                    {!message.isDeleted ? (
                      <Tooltip label={pinnedMessageIds.has(message.id) ? "Bỏ ghim tin nhắn" : "Ghim tin nhắn"}>
                        <button
                          aria-label={pinnedMessageIds.has(message.id) ? "Bỏ ghim tin nhắn" : "Ghim tin nhắn"}
                          className={pinnedMessageIds.has(message.id) ? "message-pin-button message-pin-button--active" : "message-pin-button"}
                          onClick={() => {
                            setActionMenuMessageId(null);
                            onTogglePin(message, pinnedMessageIds.has(message.id));
                          }}
                          role="menuitem"
                          type="button"
                        >
                          <Pin size={15} />
                        </button>
                      </Tooltip>
                    ) : null}
                    {message.canEdit ? (
                      <Tooltip label="Sửa tin nhắn">
                        <button
                          aria-label="Sửa tin nhắn"
                          onClick={() => {
                            setActionMenuMessageId(null);
                            onStartEdit(message);
                          }}
                          role="menuitem"
                          type="button"
                        >
                          <Edit3 size={15} />
                        </button>
                      </Tooltip>
                    ) : null}
                    {message.canDelete ? (
                      <Tooltip label="Xóa tin nhắn">
                        <button
                          aria-label="Xóa tin nhắn"
                          onClick={() => {
                            setActionMenuMessageId(null);
                            onDeleteMessage(message);
                          }}
                          role="menuitem"
                          type="button"
                        >
                          <Trash2 size={15} />
                        </button>
                      </Tooltip>
                    ) : null}
                    {!message.isDeleted && !message.isPending ? (
                      <Tooltip label="Chuyển tiếp tin nhắn">
                        <button
                          aria-label="Chuyển tiếp tin nhắn"
                          onClick={() => {
                            setActionMenuMessageId(null);
                            onForwardMessage(message.id);
                          }}
                          role="menuitem"
                          type="button"
                        >
                          <Share2 size={15} />
                        </button>
                      </Tooltip>
                    ) : null}
                    <Tooltip label="Mở luồng trả lời">
                      <button
                        aria-label="Mở luồng trả lời"
                        onClick={() => {
                          setActionMenuMessageId(null);
                          onOpenThread(message.id);
                        }}
                        role="menuitem"
                        type="button"
                      >
                        <MessageCircle size={15} />
                      </button>
                    </Tooltip>
                    <Tooltip label="Trả lời">
                      <button
                        aria-label="Trả lời"
                        onClick={() => {
                          setActionMenuMessageId(null);
                          onReplyMessage(message, messageAuthor);
                        }}
                        role="menuitem"
                        type="button"
                      >
                        <Reply size={15} />
                      </button>
                    </Tooltip>
                  </div>
                ) : null}
              </div>
            {replyPreview ? (
              <ReplyQuotePreview
                onOpen={
                  replyPreview.messageId
                    ? () => {
                        const target = timelineRef.current?.querySelector<HTMLElement>(
                          `[data-message-id="${cssEscape(replyPreview.messageId)}"]`
                        );
                        target?.scrollIntoView({ behavior: "smooth", block: "center" });
                      }
                    : undefined
                }
                preview={replyPreview}
              />
            ) : null}
            {editingMessageId === message.id ? (
              <form className="message-edit-form" onSubmit={onSubmitEdit}>
                <input
                  aria-label="Nội dung sửa"
                  autoFocus
                  onChange={(event) => onChangeEditingBody(event.target.value)}
                  value={editingBody}
                />
                <Button disabled={isEditingPending || !editingBody.trim()} size="sm" type="submit">
                  Lưu
                </Button>
                <Button disabled={isEditingPending} onClick={onCancelEdit} size="sm" variant="ghost">
                  Hủy
                </Button>
              </form>
            ) : shouldRenderMessageBody(message) ? (
              <MessageBody body={message.qrImageUrl ? stripDisplayedQRURL(message.body) : message.body} />
            ) : null}
            {message.isVoice && !message.attachments?.some((attachment) => attachment.isAudio) ? (
              <div className="attachment-audio"><span className="attachment-media-loading">Đang tải tin nhắn thoại...</span></div>
            ) : null}
            {message.qrImageUrl ? (
              <a
                aria-label="Mở mã QR thanh toán"
                className="message-payment-qr"
                href={message.qrImageUrl}
                rel="noreferrer"
                target="_blank"
              >
                <BrandedQRCode
                  alt={`Mã QR thanh toán${message.qrReference ? ` ${message.qrReference}` : ""}`}
                  src={message.qrImageUrl}
                />
                <span>
                  <strong>Quét mã QR để thanh toán</strong>
                  <small>{message.qrReference ? `Mã tham chiếu: ${message.qrReference}` : "Nhấn để mở ảnh QR kích thước đầy đủ"}</small>
                </span>
              </a>
            ) : null}
            {message.attachments?.length ? (
              <div className={attachmentListClassName(message.attachments)}>
                {message.attachments.map((attachment) =>
                  attachment.isAudio || attachment.isImage || attachment.isVideo ? (
                    <AttachmentMedia
                      attachment={attachment}
                      key={attachment.id}
                      onDownload={onDownloadAttachment}
                      onPreview={onPreviewAttachment}
                      onResolve={onResolveAttachment}
                    />
                  ) : (
                    <AttachmentFileCard
                      attachment={attachment}
                      key={attachment.id}
                      onDownload={onDownloadAttachment}
                    />
                  )
                )}
              </div>
            ) : null}
            {message.reactions?.length ? (
              <div className="reaction-pill">
                {message.reactions.map((reaction) => (
                  <button
                    className={reaction.reactedByMe ? "reaction-pill__item reaction-pill__item--active" : "reaction-pill__item"}
                    key={reaction.emoji}
                    onClick={() => onToggleReaction(message, reaction.emoji)}
                    type="button"
                  >
                    <ReactionGlyph emoji={reaction.emoji} />
                    <span>{reaction.count}</span>
                  </button>
                ))}
              </div>
            ) : null}
            {!message.isDeleted ? (
              <div className="message-reaction-control">
                <button
                  aria-expanded={reactionPickerMessageId === message.id}
                  aria-label="Thả cảm xúc"
                  className="reaction-add-button"
                  onClick={() => {
                    setActionMenuMessageId(null);
                    setReactionPickerMessageId((current) => current === message.id ? null : message.id);
                  }}
                  type="button"
                >
                  <Smile size={15} />
                </button>
                {reactionPickerMessageId === message.id ? (
                  <div className="message-reaction-picker" role="menu" aria-label="Chọn cảm xúc">
                    {quickReactions.map((reaction) => {
                      const ReactionIcon = reaction.icon;
                      return (
                        <button
                          aria-label={`Thả cảm xúc ${reaction.label}`}
                          className={`message-reaction-picker__choice ${reaction.className}`}
                          key={reaction.emoji}
                          onClick={() => {
                            onToggleReaction(message, reaction.emoji);
                            setReactionPickerMessageId(null);
                          }}
                          role="menuitem"
                          type="button"
                        >
                          <ReactionIcon size={20} strokeWidth={2.2} />
                        </button>
                      );
                    })}
                  </div>
                ) : null}
              </div>
            ) : null}
            </div>
            <footer className="message-row__meta">
              <time>{message.sentAt}</time>
              {lastOwnReceipt?.messageId === message.id ? (
                <span className={`message-read-status message-read-status--${lastOwnReceipt.tone}`}>
                  <span aria-hidden="true">✓✓</span>
                  {lastOwnReceipt.label}
                </span>
              ) : null}
            </footer>
          </div>
        </article>
        );
      })}
      <div ref={bottomRef} aria-hidden="true" />
    </div>
  );
}

function CallMessageRow({
  focused,
  message,
  messageAuthor,
  onRetryCall
}: {
  focused: boolean;
  message: ChatMessage;
  messageAuthor: ChatUser;
  onRetryCall: (mode: "audio" | "video") => void;
}) {
  const callEvent = message.callEvent;
  if (!callEvent) {
    return null;
  }

  const isMissed = callEvent.status === "missed";
  const isLocalCallRow = message.isMine;
  const CallIcon = isMissed ? PhoneOff : callEvent.mode === "video" ? Video : Phone;

  return (
    <article
      className={`${isLocalCallRow ? "message-row message-row--local" : "message-row"} message-row--call${isMissed ? " message-row--call-missed" : ""}${focused ? " message-row--focused" : ""}`}
      data-message-id={message.id}
    >
      {!isLocalCallRow ? <Avatar name={messageAuthor.name} src={messageAuthor.avatarUrl} status={messageAuthor.status} /> : null}
      <div className="message-row__stack">
        <div className={isMissed ? "message-call-card message-call-card--missed" : "message-call-card"}>
          <strong>{callMessageTitle(callEvent)}</strong>
          <span>
            <CallIcon size={17} />
            {callMessageDetail(callEvent)}
          </span>
          <button onClick={() => onRetryCall(callEvent.mode)} type="button">
            Gọi lại
          </button>
        </div>
        <time className="message-call-time">{message.sentAt}</time>
      </div>
    </article>
  );
}

function callMessageTitle(callEvent: NonNullable<ChatMessage["callEvent"]>): string {
  if (callEvent.status === "missed") {
    return callEvent.direction === "incoming" ? "Bạn bị nhỡ" : "Cuộc gọi nhỡ";
  }
  const modeLabel = callEvent.mode === "video" ? "video" : "thoại";
  const directionLabel = callEvent.direction === "incoming" ? "đến" : "đi";
  return `Cuộc gọi ${modeLabel} ${directionLabel}`;
}

function callMessageDetail(callEvent: NonNullable<ChatMessage["callEvent"]>): string {
  const modeLabel = callEvent.mode === "video" ? "Cuộc gọi video" : "Cuộc gọi thoại";
  if (callEvent.status === "missed") {
    return modeLabel;
  }
  return formatCallDurationLabel(callEvent.durationSeconds ?? 0);
}

function formatCallDurationLabel(durationSeconds: number): string {
  const safeSeconds = Math.max(0, Math.round(durationSeconds));
  const minutes = Math.floor(safeSeconds / 60);
  const seconds = safeSeconds % 60;
  return `${minutes} phút ${seconds} giây`;
}

type MessageReceipt = {
  label: "Đã nhận" | "Đã xem";
  messageId: string;
  tone: "delivered" | "seen";
};

function resolveLastOwnMessageReceipt(
  messages: ChatMessage[],
  readMembers: ChannelMember[],
  currentUserId: string,
  messageOrderById: Map<string, number>
): MessageReceipt | null {
  const message = [...messages].reverse().find((item) => item.isMine && !item.callEvent && !item.isDeleted && !item.isPending && !item.isLocal);
  if (!message) {
    return null;
  }

  const messageIndex = messageOrderById.get(message.id) ?? -1;
  const messageCreatedAt = parseDateMs(message.rawCreatedAt);
  const hasReader = readMembers.some((member) => {
    if (!member.user_id || member.user_id === currentUserId) {
      return false;
    }
    return memberHasReadMessage(member, messageIndex, messageCreatedAt, messageOrderById);
  });

  return {
    label: hasReader ? "Đã xem" : "Đã nhận",
    messageId: message.id,
    tone: hasReader ? "seen" : "delivered"
  };
}

function memberHasReadMessage(
  member: ChannelMember,
  messageIndex: number,
  messageCreatedAt: number | null,
  messageOrderById: Map<string, number>
) {
  const readMessageId = member.last_read_message_id ?? "";
  const readMessageIndex = readMessageId ? messageOrderById.get(readMessageId) : undefined;
  if (typeof readMessageIndex === "number" && messageIndex >= 0) {
    return readMessageIndex >= messageIndex;
  }

  const readAt = parseDateMs(member.last_read_at);
  return Boolean(readAt && messageCreatedAt && readAt >= messageCreatedAt);
}

function parseDateMs(value?: string | null) {
  if (!value) {
    return null;
  }
  const timestamp = new Date(value).getTime();
  return Number.isNaN(timestamp) ? null : timestamp;
}

type DesktopDeepLinkTarget = {
  channelId?: string;
  kind?: "channel" | "dm";
  messageId?: string;
  section?: string;
  workspaceId?: string;
};

function parseDesktopDeepLink(value: string): DesktopDeepLinkTarget | null {
  try {
    const url = new URL(value);
    if (url.protocol !== "webtui:") {
      return null;
    }

    const host = url.host.toLowerCase();
    const segments = url.pathname.split("/").filter(Boolean);
    const workspaceId =
      url.searchParams.get("workspaceId") ||
      url.searchParams.get("workspace") ||
      url.searchParams.get("workspace_id") ||
      (host === "chat" && (segments[0] === "conversations" || segments[0] === "channels") ? "" : segments[0]) ||
      "";
    const section = url.searchParams.get("section") || (host === "section" ? segments[1] : "");
    const kindParam = url.searchParams.get("kind");
    const kind = kindParam === "dm" || host === "dm" || segments[0] === "conversations" ? "dm" : "channel";
    const channelId =
      url.searchParams.get("target") ||
      url.searchParams.get("channel") ||
      url.searchParams.get("channel_id") ||
      (host === "chat" ? segments[1] : segments[0]) ||
      "";
    const messageId = url.searchParams.get("message") || url.searchParams.get("message_id") || "";

    return {
      channelId,
      kind,
      messageId,
      section,
      workspaceId
    };
  } catch {
    return null;
  }
}

function notificationPreferencesStorageKey(workspaceId: string): string {
  return `vpsttt:notification-preferences:${workspaceId}`;
}

function desktopVersionBadgeTone(status: DesktopVersionStatus["status"]): "green" | "orange" | "red" | "slate" {
  if (status === "unsupported") {
    return "red";
  }
  if (status === "update_available") {
    return "orange";
  }
  if (status === "current") {
    return "green";
  }
  return "slate";
}

function parseNotificationPreferences(value: string | null): NotificationPreferences {
  if (!value) {
    return defaultNotificationPreferences;
  }

  try {
    const parsed = JSON.parse(value) as Partial<NotificationPreferences>;
    return normalizeNotificationPreferences(parsed);
  } catch {
    return defaultNotificationPreferences;
  }
}

function mapNotificationPreference(preference: NotificationPreference): NotificationPreferences {
  return normalizeNotificationPreferences({
    mode: preference.mode,
    preview: preference.preview,
    quietEnd: preference.quiet_end,
    quietHours: preference.quiet_hours,
    quietStart: preference.quiet_start
  });
}

function toNotificationPreferenceInput(workspaceId: string, preference: NotificationPreferences): NotificationPreferenceInput {
  return {
    mode: preference.mode,
    preview: preference.preview,
    quiet_end: preference.quietEnd,
    quiet_hours: preference.quietHours,
    quiet_start: preference.quietStart,
    workspace_id: workspaceId
  };
}

function normalizeNotificationPreferences(preferences: Partial<NotificationPreferences>): NotificationPreferences {
  return {
    mode: preferences.mode === "mentions" || preferences.mode === "muted" ? preferences.mode : "all",
    preview: typeof preferences.preview === "boolean" ? preferences.preview : defaultNotificationPreferences.preview,
    quietEnd: isTimeValue(preferences.quietEnd) ? preferences.quietEnd : defaultNotificationPreferences.quietEnd,
    quietHours: typeof preferences.quietHours === "boolean" ? preferences.quietHours : defaultNotificationPreferences.quietHours,
    quietStart: isTimeValue(preferences.quietStart) ? preferences.quietStart : defaultNotificationPreferences.quietStart
  };
}

function nativeNotificationData(notification: NotificationItem, workspaceId: string): Record<string, unknown> {
  return {
    ...(notification.data ?? {}),
    callId: notification.callId,
    call_id: notification.callId,
    channelId: notification.channelId,
    channel_id: notification.channelId,
    initiatorUserId: notification.initiatorUserId,
    initiator_user_id: notification.initiatorUserId,
    messageId: notification.messageId,
    message_id: notification.messageId,
    mode: notification.callMode,
    status: notification.callStatus,
    targetUserId: notification.targetUserId,
    target_user_id: notification.targetUserId,
    workspaceId,
    workspace_id: workspaceId
  };
}

function isIncomingCallNotification(notification: NotificationItem, currentUserId: string): boolean {
  const eventType = typeof notification.data?.event_type === "string" ? notification.data.event_type : "";
  const status = notification.callStatus?.toLowerCase() || "";
  const targetsCurrentUser = !notification.targetUserId || notification.targetUserId === currentUserId;
  return Boolean(
    notification.callId &&
      targetsCurrentUser &&
      (status === "" || status === "ringing") &&
      (eventType === "call_invite" || notification.type === "invite")
  );
}

function shouldShowDesktopNotification(notification: NotificationItem, preferences: NotificationPreferences): boolean {
  if (preferences.mode === "muted") {
    return false;
  }
  if (preferences.mode === "mentions" && !isMentionNotification(notification)) {
    return false;
  }
  if (preferences.quietHours && isWithinQuietHours(preferences.quietStart, preferences.quietEnd)) {
    return false;
  }
  return true;
}

function isMessageNotification(notification: NotificationItem): boolean {
  const type = notification.type.toLowerCase();
  return Boolean(notification.channelId && notification.messageId) || type.includes("message") || type.includes("tin_nhan");
}

function isMentionNotification(notification: NotificationItem): boolean {
  const type = notification.type.toLowerCase();
  return type.includes("mention") || type.includes("tag");
}

function isWithinQuietHours(start: string, end: string, now = new Date()): boolean {
  const startMinutes = timeValueToMinutes(start);
  const endMinutes = timeValueToMinutes(end);
  if (startMinutes === null || endMinutes === null || startMinutes === endMinutes) {
    return false;
  }

  const currentMinutes = now.getHours() * 60 + now.getMinutes();
  return startMinutes < endMinutes
    ? currentMinutes >= startMinutes && currentMinutes < endMinutes
    : currentMinutes >= startMinutes || currentMinutes < endMinutes;
}

function isTimeValue(value: unknown): value is string {
  return typeof value === "string" && /^\d{2}:\d{2}$/.test(value) && timeValueToMinutes(value) !== null;
}

function timeValueToMinutes(value: string): number | null {
  const [hourValue, minuteValue] = value.split(":");
  const hour = Number(hourValue);
  const minute = Number(minuteValue);
  if (!Number.isInteger(hour) || !Number.isInteger(minute) || hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return null;
  }
  return hour * 60 + minute;
}

function cssEscape(value: string): string {
  if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
    return CSS.escape(value);
  }
  return value.replace(/["\\]/g, "\\$&");
}

function resolveMentionToken(value: string): { query: string; start: number } | null {
  const match = value.match(/(^|\s)@([^\s@]{0,32})$/u);
  if (!match || typeof match.index !== "number") {
    return null;
  }

  return {
    query: match[2] ?? "",
    start: match.index + (match[1]?.length ?? 0)
  };
}

function buildMentionSuggestions(
  members: Array<ChannelMember | WorkspaceMember>,
  currentUserId: string,
  query: string
): Array<ChannelMember | WorkspaceMember> {
  const normalizedQuery = query.trim().toLowerCase();
  const seen = new Set<string>();

  return members
    .filter((member) => {
      if (!member.user_id || member.user_id === currentUserId || seen.has(member.user_id)) {
        return false;
      }
      seen.add(member.user_id);
      if (!normalizedQuery) {
        return true;
      }
      return mentionMemberSearchText(member).includes(normalizedQuery);
    })
    .slice(0, 6);
}

function mentionMemberName(member: ChannelMember | WorkspaceMember): string {
  return member.display_name || member.username || member.email || member.user_id || "Thành viên";
}

function mentionMemberSearchText(member: ChannelMember | WorkspaceMember): string {
  return [member.display_name, member.username, member.email, member.user_id].filter(Boolean).join(" ").toLowerCase();
}

function collectMentionedUserIds(body: string, members: Array<ChannelMember | WorkspaceMember>): string[] {
  const normalizedBody = body.toLowerCase();
  const seen = new Set<string>();
  const mentionedUserIds: string[] = [];

  for (const member of members) {
    if (!member.user_id || seen.has(member.user_id)) {
      continue;
    }

    const candidates = [
      mentionMemberName(member),
      member.username,
      member.email
    ].filter(Boolean) as string[];
    const isMentioned = candidates.some((candidate) => normalizedBody.includes(`@${candidate.toLowerCase()}`));
    if (!isMentioned) {
      continue;
    }

    seen.add(member.user_id);
    mentionedUserIds.push(member.user_id);
  }

  return mentionedUserIds;
}

function validateUploadFiles(files: File[], imageOnly = false): {
  accepted: File[];
  rejected: Array<{ file: File; reason: string }>;
} {
  const accepted: File[] = [];
  const rejected: Array<{ file: File; reason: string }> = [];

  for (const file of files) {
    const reason = validateUploadFile(file, imageOnly);
    if (reason) {
      rejected.push({ file, reason });
    } else {
      accepted.push(file);
    }
  }

  return { accepted, rejected };
}

function validateUploadFile(file: File, imageOnly: boolean): string | null {
  const name = file.name.trim();
  const mimeType = normalizedUploadMimeType(file);

  if (!name) {
    return "Tên file không được để trống.";
  }
  if ([...name].length > 255) {
    return `${name} có tên dài quá 255 ký tự.`;
  }
  if (file.size <= 0) {
    return `${name} không có dữ liệu.`;
  }
  if (file.size > maxUploadSizeBytes) {
    return `${name} vượt quá giới hạn ${formatFileSize(maxUploadSizeBytes)}.`;
  }
  if (imageOnly && !mimeType.startsWith("image/")) {
    return `${name} không phải là ảnh.`;
  }
  if (!isAllowedUploadMimeType(mimeType)) {
    return `${name} có định dạng không được hỗ trợ.`;
  }

  return null;
}

function normalizedUploadMimeType(file: File): string {
  const browserMimeType = file.type.toLowerCase().split(";")[0]?.trim();
  return browserMimeType || uploadMimeTypeFromName(file.name);
}

function fileExtensionLabel(name: string, mimeType?: string): string {
  const extension = name.trim().split(".").at(-1)?.toUpperCase() ?? "";
  if (/^[A-Z0-9]{1,5}$/.test(extension) && extension !== name.toUpperCase()) {
    return extension;
  }

  if (mimeType?.includes("pdf")) {
    return "PDF";
  }
  if (mimeType?.startsWith("text/")) {
    return "TXT";
  }
  return "FILE";
}

function uploadMimeTypeFromName(name: string): string {
  const extension = name.split(".").at(-1)?.toLowerCase() ?? "";
  const map: Record<string, string> = {
    doc: "application/msword",
    docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    gif: "image/gif",
    jpeg: "image/jpeg",
    jpg: "image/jpeg",
    json: "application/json",
    m4a: "audio/mp4",
    mp3: "audio/mpeg",
    ogg: "audio/ogg",
    pdf: "application/pdf",
    png: "image/png",
    ppt: "application/vnd.ms-powerpoint",
    pptx: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    txt: "text/plain",
    wav: "audio/wav",
    webm: "audio/webm",
    webp: "image/webp",
    xls: "application/vnd.ms-excel",
    xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    zip: "application/zip"
  };

  return map[extension] ?? "application/octet-stream";
}

function isAllowedUploadMimeType(mimeType: string): boolean {
  if (mimeType.startsWith("image/") || mimeType.startsWith("text/")) {
    return true;
  }

  return uploadAcceptList.some((accept) => accept === mimeType);
}

function profilePresenceStatus(
  status: string | undefined,
  fallback: ChatUser["status"] | undefined
): ChatUser["status"] {
  if (status === "online" || status === "offline" || status === "busy") {
    return status;
  }
  return fallback ?? "offline";
}

function enrichMemberProfiles<T extends ChannelMember | WorkspaceMember>(
  members: T[],
  profiles: ReadonlyMap<string, ChatUser>
): T[] {
  return members.map((member) => {
    const profile = profiles.get(member.user_id);
    if (!profile) {
      return member;
    }

    return {
      ...member,
      avatar_url: member.avatar_url || profile.avatarUrl,
      display_name: member.display_name || profile.name,
      email: member.email || profile.email,
      username: member.username || profile.username
    };
  });
}

type MessageAuthorLookupMember = Pick<ChannelMember | WorkspaceMember, "avatar_url" | "display_name" | "email" | "status" | "user_id" | "username">;

function buildMessageAuthorLookup(
  workspaceMembers: WorkspaceMember[],
  channelMembers: ChannelMember[]
): Map<MessageAuthorLookupMember["user_id"], MessageAuthorLookupMember> {
  const memberByUserId = new Map<MessageAuthorLookupMember["user_id"], MessageAuthorLookupMember>();
  for (const member of [...workspaceMembers, ...channelMembers]) {
    const current = memberByUserId.get(member.user_id);
    const nextStatus = member.status === "online" || member.status === "offline" || member.status === "busy"
      ? member.status
      : current?.status ?? member.status;
    memberByUserId.set(member.user_id, {
      avatar_url: member.avatar_url || current?.avatar_url,
      display_name: member.display_name || current?.display_name,
      email: member.email || current?.email,
      status: nextStatus,
      user_id: member.user_id,
      username: member.username || current?.username
    });
  }
  return memberByUserId;
}

function resolveRenderedMessageAuthor(message: ChatMessage, memberByUserId: Map<string, MessageAuthorLookupMember>): ChatUser {
  if (message.isBot || !message.rawSenderId) {
    return message.author;
  }

  const member = memberByUserId.get(message.rawSenderId);
  if (!member) {
    return message.author;
  }

  const memberDisplayName = member.display_name || member.username || member.email || "";
  const memberPresence = member.status === "online" || member.status === "offline" || member.status === "busy"
    ? member.status
    : message.author.status;

  return {
    avatarUrl: member.avatar_url || message.author.avatarUrl,
    id: member.user_id,
    name: memberDisplayName || message.author.name,
    status: memberPresence
  };
}

function UnreadBadge({ count }: { count: number }) {
  if (count <= 0) {
    return null;
  }

  return (
    <span aria-label={`${count} tin nhắn chưa đọc`} className="conversation-row__unread-badge">
      <span>{count > 99 ? "99+" : count}</span>
    </span>
  );
}

function createMessageReplyPreview(message: ChatMessage, author: ChatUser): MessageReplyPreview {
  return {
    authorName: author.name,
    body: replyPreviewBody(message),
    messageId: message.id
  };
}

function replyPreviewBody(message: ChatMessage): string {
  const text = message.isDeleted ? "Tin nhắn đã bị xóa" : stripDisplayedQRURL(message.body).trim();
  if (text) {
    return text.length > 160 ? `${text.slice(0, 157).trimEnd()}...` : text;
  }

  if (message.isVoice || message.attachments?.some((attachment) => attachment.isAudio)) {
    return "Tin nhắn thoại";
  }
  if (message.attachments?.some((attachment) => attachment.isImage)) {
    return "Hình ảnh";
  }
  if (message.attachments?.some((attachment) => attachment.isVideo)) {
    return "Video";
  }
  if (message.attachments?.length) {
    return message.attachments.length === 1 ? message.attachments[0].name : `${message.attachments.length} file đính kèm`;
  }
  if (message.callEvent) {
    return callMessageTitle(message.callEvent);
  }
  return "Tin nhắn";
}

function ReactionGlyph({ emoji, size = 15 }: { emoji: string; size?: number }) {
  const reaction = quickReactions.find((item) => item.emoji === emoji);
  if (!reaction) {
    return <span aria-hidden="true" className="reaction-glyph__emoji">{emoji}</span>;
  }

  const ReactionIcon = reaction.icon;
  return (
    <span aria-hidden="true" className={`reaction-glyph ${reaction.className}`}>
      <ReactionIcon size={size} strokeWidth={2.35} />
    </span>
  );
}

function MessageBody({ body }: { body: string }) {
  if (body.trim() === "👍") {
    return <div aria-label="Thích" className="message-body message-body--like"><ComposerLikeIcon /></div>;
  }

  const blocks = body.split("```");
  return (
    <div className="message-body">
      {blocks.map((block, index) =>
        index % 2 === 1 ? (
          <pre key={`${index}-${block.slice(0, 12)}`}><code>{stripCodeLanguage(block)}</code></pre>
        ) : (
          <Fragment key={`${index}-${block.slice(0, 12)}`}>{renderInlineMarkdown(block)}</Fragment>
        )
      )}
    </div>
  );
}

function renderInlineMarkdown(value: string): ReactNode[] {
  const pattern = /(\*\*[^*\n]+\*\*|`[^`\n]+`|\[[^\]\n]+\]\(https?:\/\/[^\s)]+\))/gi;
  const nodes: ReactNode[] = [];
  let cursor = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(value)) !== null) {
    if (match.index > cursor) {
      nodes.push(value.slice(cursor, match.index));
    }
    const token = match[0];
    if (token.startsWith("**")) {
      nodes.push(<strong key={`${match.index}-${token}`}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("`")) {
      nodes.push(<code key={`${match.index}-${token}`}>{token.slice(1, -1)}</code>);
    } else {
      const link = /^\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)$/i.exec(token);
      nodes.push(link ? <a href={link[2]} key={`${match.index}-${token}`} rel="noreferrer noopener" target="_blank">{link[1]}</a> : token);
    }
    cursor = match.index + token.length;
  }
  if (cursor < value.length) {
    nodes.push(value.slice(cursor));
  }
  return nodes;
}

function stripCodeLanguage(block: string) {
  const normalized = block.replace(/^\r?\n/, "");
  const firstNewline = normalized.indexOf("\n");
  if (firstNewline > 0 && /^[a-z0-9_+-]{1,20}\r?$/i.test(normalized.slice(0, firstNewline))) {
    return normalized.slice(firstNewline + 1).replace(/\r?\n$/, "");
  }
  return normalized.replace(/\r?\n$/, "");
}

function AttachmentFileCard({
  attachment,
  onDownload
}: {
  attachment: MessageAttachmentItem;
  onDownload: (attachment: MessageAttachmentItem) => void;
}) {
  return (
    <button
      aria-label={`Tải xuống ${attachment.name}`}
      className={`attachment-card attachment-file-card attachment-file-card--${attachment.tone}`}
      onClick={() => onDownload(attachment)}
      title={attachment.name}
      type="button"
    >
      <span className="attachment-file-card__preview" aria-hidden="true">
        <FileText size={28} />
      </span>
      <span className="attachment-file-card__content">
        <strong className="attachment-file-card__name">{attachment.name}</strong>
      </span>
    </button>
  );
}

function AttachmentMedia({
  attachment,
  onDownload,
  onPreview,
  onResolve
}: {
  attachment: MessageAttachmentItem;
  onDownload: (attachment: MessageAttachmentItem) => void;
  onPreview: (attachment: MessageAttachmentItem, source?: string) => void;
  onResolve: (fileId: string) => Promise<Blob>;
}) {
  const directSource = directEmbeddableMediaSource(attachment.previewUrl) ?? directEmbeddableMediaSource(attachment.url);
  const [resolvedSource, setResolvedSource] = useState<string | undefined>(directSource ?? getCachedMediaUrl(attachment.fileId));

  useEffect(() => {
    if (directSource) {
      setResolvedSource(directSource);
      return undefined;
    }

    let disposed = false;
    void resolveCachedMediaUrl(attachment.fileId, () => onResolve(attachment.fileId))
      .then((url) => {
        if (disposed) {
          return;
        }
        setResolvedSource(url);
      })
      .catch(() => setResolvedSource(undefined));

    return () => {
      disposed = true;
    };
  }, [attachment.fileId, directSource, onResolve]);

  if (attachment.isAudio) {
    return resolvedSource
      ? <VoiceMessagePlayer id={attachment.id} source={resolvedSource} />
      : <div className="attachment-audio"><span className="attachment-media-loading">Đang tải tin nhắn thoại...</span></div>;
  }

  if (attachment.isVideo) {
    return (
      <div className="attachment-card attachment-media-card attachment-media-card--video attachment-video">
        <div className="attachment-media-card__preview">
          {resolvedSource ? (
            <video controls playsInline preload="metadata" src={resolvedSource}>
              Trình duyệt của bạn không hỗ trợ phát video.
            </video>
          ) : <span className="attachment-media-loading">Đang tải video...</span>}
        </div>
        <button className="attachment-media-card__caption" disabled={!resolvedSource} onClick={() => onDownload(attachment)} title={attachment.name} type="button">
          {attachment.name}
        </button>
      </div>
    );
  }

  return (
    <button
      aria-label={`Mở ảnh ${attachment.name}`}
      className="attachment-card attachment-media-card attachment-media-card--image attachment-image"
      disabled={!resolvedSource}
      onClick={() => onPreview(attachment, resolvedSource)}
      title={attachment.name}
      type="button"
    >
      <span className="attachment-media-card__preview">
        {resolvedSource ? <img alt={attachment.name} decoding="async" loading="lazy" src={resolvedSource} /> : <span className="attachment-media-loading">Đang tải ảnh...</span>}
      </span>
    </button>
  );
}

function directEmbeddableMediaSource(value?: string): string | undefined {
  const source = value?.trim();
  if (!source) {
    return undefined;
  }
  if (source.startsWith("blob:") || source.startsWith("data:")) {
    return source;
  }
  try {
    const url = new URL(source);
    return url.protocol === "http:" || url.protocol === "https:" ? url.toString() : undefined;
  } catch {
    return undefined;
  }
}

const voiceWaveformHeights = [8, 14, 10, 20, 13, 24, 16, 10, 21, 14, 26, 18, 10, 20, 13, 24, 16, 9, 18, 12, 23, 16, 10, 21, 14, 25, 17, 10, 20, 13, 22, 15];

function VoiceMessagePlayer({ id, source }: { id: string; source: string }) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const progress = duration > 0 ? currentTime / duration : 0;

  useEffect(() => {
    setCurrentTime(0);
    setDuration(0);
    setIsPlaying(false);
    setPlaybackRate(1);
  }, [source]);

  useEffect(() => {
    const pauseOtherPlayer = (event: Event) => {
      const activeId = (event as CustomEvent<string>).detail;
      if (activeId !== id) {
        audioRef.current?.pause();
      }
    };
    window.addEventListener("webtui:voice-play", pauseOtherPlayer);
    return () => window.removeEventListener("webtui:voice-play", pauseOtherPlayer);
  }, [id]);

  async function togglePlayback() {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.paused) {
      window.dispatchEvent(new CustomEvent("webtui:voice-play", { detail: id }));
      try {
        await audio.play();
      } catch {
        setIsPlaying(false);
      }
    } else {
      audio.pause();
    }
  }

  function cyclePlaybackRate() {
    const nextRate = playbackRate === 1 ? 1.5 : playbackRate === 1.5 ? 2 : 1;
    setPlaybackRate(nextRate);
    if (audioRef.current) audioRef.current.playbackRate = nextRate;
  }

  return (
    <div className={isPlaying ? "voice-message voice-message--playing" : "voice-message"}>
      <button
        aria-label={isPlaying ? "Tạm dừng tin nhắn thoại" : "Phát tin nhắn thoại"}
        className="voice-message__play"
        onClick={() => void togglePlayback()}
        type="button"
      >
        <span className={isPlaying ? "voice-message__pause-icon" : "voice-message__play-icon"} />
      </button>
      <div className="voice-message__timeline">
        <div className="voice-message__waveform" aria-hidden="true">
          {voiceWaveformHeights.map((height, index) => (
            <i
              className={index / voiceWaveformHeights.length <= progress ? "voice-message__bar voice-message__bar--played" : "voice-message__bar"}
              key={`${id}-${index}`}
              style={{ height }}
            />
          ))}
        </div>
        <input
          aria-label="Tua tin nhắn thoại"
          max={duration || 1}
          min="0"
          onChange={(event) => {
            const nextTime = Number(event.target.value);
            setCurrentTime(nextTime);
            if (audioRef.current) audioRef.current.currentTime = nextTime;
          }}
          step="0.01"
          type="range"
          value={currentTime}
        />
      </div>
      <time>{formatVoiceTime(isPlaying ? currentTime : duration)}</time>
      <button aria-label="Đổi tốc độ phát" className="voice-message__rate" onClick={cyclePlaybackRate} type="button">
        {playbackRate}x
      </button>
      <audio
        onDurationChange={(event) => setDuration(Number.isFinite(event.currentTarget.duration) ? event.currentTarget.duration : 0)}
        onEnded={() => {
          setCurrentTime(0);
          setIsPlaying(false);
        }}
        onLoadedMetadata={(event) => setDuration(Number.isFinite(event.currentTarget.duration) ? event.currentTarget.duration : 0)}
        onPause={() => setIsPlaying(false)}
        onPlay={() => setIsPlaying(true)}
        onTimeUpdate={(event) => setCurrentTime(event.currentTarget.currentTime)}
        preload="metadata"
        ref={audioRef}
        src={source}
      >
        Trình duyệt của bạn không hỗ trợ phát tin nhắn thoại.
      </audio>
    </div>
  );
}

function formatVoiceTime(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const rounded = Math.floor(seconds);
  return `${Math.floor(rounded / 60)}:${String(rounded % 60).padStart(2, "0")}`;
}

function shouldRenderMessageBody(message: ChatMessage): boolean {
  const body = message.body.trim();
  const normalizedBody = normalizeAttachmentMessageBody(body);
  if (!body) {
    return false;
  }

  if (message.isVoice) {
    return false;
  }

  const attachments = message.attachments ?? [];
  if (isGeneratedAttachmentBody(normalizedBody, attachments)) {
    return false;
  }
  const hasOnlyImages = attachments.length > 0 && attachments.every((attachment) => attachment.isImage);
  const hasOnlyAudio = attachments.length > 0 && attachments.every((attachment) => attachment.isAudio);
  if (hasOnlyAudio && /^Đã gửi(?: \d+)? tin nhắn thoại$/.test(body)) {
    return false;
  }
  if (attachments.length === 1 && body === `Đã gửi file ${attachments[0].name}`) {
    return false;
  }
  if (attachments.length > 1 && body === `Đã gửi ${attachments.length} file`) {
    return false;
  }
  if (!hasOnlyImages) {
    return true;
  }

  return !/^Đã gửi(?: \d+)? ảnh$/.test(body);
}

function isGeneratedAttachmentBody(normalizedBody: string, attachments: MessageAttachmentItem[]): boolean {
  if (!attachments.length || !normalizedBody) {
    return false;
  }
  if (normalizedBody === "da gui tep dinh kem") {
    return true;
  }

  const hasOnlyImages = attachments.every((attachment) => attachment.isImage);
  const hasOnlyAudio = attachments.every((attachment) => attachment.isAudio);
  if (hasOnlyImages && /^da gui(?: \d+)? anh$/.test(normalizedBody)) {
    return true;
  }
  if (hasOnlyAudio && /^da gui(?: \d+)? tin nhan thoai$/.test(normalizedBody)) {
    return true;
  }
  if (attachments.length === 1) {
    const attachmentName = normalizeAttachmentMessageBody(attachments[0].name);
    return normalizedBody === `da gui file ${attachmentName}` || (hasOnlyImages && normalizedBody === attachmentName);
  }
  return normalizedBody === `da gui ${attachments.length} file`;
}

function normalizeAttachmentMessageBody(value: string): string {
  return value
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .replace(/đ/g, "d")
    .replace(/Đ/g, "d")
    .toLowerCase()
    .replace(/\s+/g, " ")
    .trim();
}

function isImageOnlyMessage(message: ChatMessage): boolean {
  const attachments = message.attachments ?? [];
  return attachments.length > 0 && attachments.every((attachment) => attachment.isImage) && !shouldRenderMessageBody(message);
}

function attachmentListClassName(attachments: MessageAttachmentItem[]): string {
  if (!attachments.length || !attachments.every((attachment) => attachment.isImage)) {
    return "attachment-list";
  }
  return `attachment-list attachment-list--images attachment-list--images-${Math.min(attachments.length, 4)}`;
}

function isFileOnlyMessage(message: ChatMessage): boolean {
  const attachments = message.attachments ?? [];
  return attachments.length > 0 && attachments.every((attachment) => !attachment.isAudio && !attachment.isImage && !attachment.isVideo) && !shouldRenderMessageBody(message);
}

function formatRecordingTime(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60).toString().padStart(2, "0");
  const seconds = (totalSeconds % 60).toString().padStart(2, "0");
  return `${minutes}:${seconds}`;
}

function preferredVoiceMimeType() {
  return getPlatformServices().media.getSupportedAudioMimeType([
    "audio/webm;codecs=opus",
    "audio/ogg;codecs=opus",
    "audio/mp4",
    "audio/webm"
  ]);
}

function voiceFileExtension(mimeType: string) {
  const normalized = mimeType.toLowerCase();
  if (normalized.includes("ogg")) return "ogg";
  if (normalized.includes("mp4") || normalized.includes("m4a")) return "m4a";
  if (normalized.includes("mpeg")) return "mp3";
  if (normalized.includes("wav")) return "wav";
  return "webm";
}

function MediaGalleryThumbnail({
  item,
  onOpen,
  onResolve
}: {
  item: MediaItem;
  onOpen: (item: MediaItem, source?: string) => void;
  onResolve: (fileId: string) => Promise<Blob>;
}) {
  const directSource = directEmbeddableMediaSource(item.url);
  const [source, setSource] = useState(directSource ?? getCachedMediaUrl(item.id));

  useEffect(() => {
    if (directSource) {
      setSource(directSource);
      return undefined;
    }
    let disposed = false;
    void resolveCachedMediaUrl(item.id, () => onResolve(item.id))
      .then((url) => {
        if (!disposed) {
          setSource(url);
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, [directSource, item.id, onResolve]);

  return (
    <button
      aria-label={item.label}
      className={source ? "media-file-thumb media-file-thumb--loaded" : "media-file-thumb"}
      disabled={!source}
      onClick={() => onOpen(item, source)}
      style={source ? { backgroundImage: `url(${source})` } : undefined}
      type="button"
    >
      {!source ? <ImageIcon size={18} /> : null}
    </button>
  );
}

function RightDetailPanel({
  activeTab,
  channelMembers,
  files,
  isDirectChat = false,
  isLoading,
  isSendingThread,
  isThreadLoading,
  mediaItems,
  onClose,
  onCloseThread,
  onFileSelect,
  onMediaSelect,
  onResolveMedia,
  onSendThread,
  onTabChange,
  pinnedMessages,
  threadMessageId,
  threadMessages,
  workspaceMembers
}: {
  activeTab: DetailTab;
  channelMembers: ChannelMember[];
  files: FileItem[];
  isDirectChat?: boolean;
  isLoading: boolean;
  isSendingThread: boolean;
  isThreadLoading: boolean;
  mediaItems: MediaItem[];
  onClose: () => void;
  onCloseThread: () => void;
  onFileSelect: (file: FileItem) => void;
  onMediaSelect: (item: MediaItem, source?: string) => void;
  onResolveMedia: (fileId: string) => Promise<Blob>;
  onSendThread: (body: string) => Promise<boolean>;
  onTabChange: (tab: DetailTab) => void;
  pinnedMessages: PinnedMessage[];
  threadMessageId: string | null;
  threadMessages: ChatMessage[];
  workspaceMembers: WorkspaceMember[];
}) {
  const [threadDraft, setThreadDraft] = useState("");
  const currentChatLabel = isDirectChat ? "Hội thoại này" : "Kênh này";
  const memberByUserId = useMemo(
    () => buildMessageAuthorLookup(workspaceMembers, channelMembers),
    [channelMembers, workspaceMembers]
  );

  useEffect(() => {
    setThreadDraft("");
  }, [threadMessageId]);

  async function handleThreadSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = threadDraft.trim();
    if (!body || isSendingThread) {
      return;
    }
    if (await onSendThread(body)) {
      setThreadDraft("");
    }
  }

  return (
    <aside className={threadMessageId ? "detail-panel detail-panel--thread-open" : "detail-panel"} aria-label={isDirectChat ? "Thông tin hội thoại" : "Thông tin kênh"}>
      {threadMessageId ? (
        <section className="thread-panel">
          <header>
            <div>
              <h2>Luồng trả lời</h2>
              <span>{threadMessages.length} tin nhắn</span>
            </div>
            <Button aria-label="Đóng luồng trả lời" onClick={onCloseThread} size="sm" variant="icon">
              <X size={18} />
            </Button>
          </header>
          {isThreadLoading ? (
            <PanelSkeleton />
          ) : threadMessages.length ? (
            <div className="thread-list">
              {threadMessages.map((message) => {
                const messageAuthor = resolveRenderedMessageAuthor(message, memberByUserId);
                return (
                  <article key={message.id}>
                    <Avatar name={messageAuthor.name} size="sm" src={messageAuthor.avatarUrl} />
                    <div>
                      <strong>{messageAuthor.name}</strong>
                      <small>{message.sentAt}</small>
                      <MessageBody body={message.body} />
                    </div>
                  </article>
                );
              })}
            </div>
          ) : (
            <EmptyState description="Luồng này chưa có tin nhắn." title="Luồng trống" />
          )}
          <form className="thread-composer" onSubmit={handleThreadSubmit}>
            <input
              aria-label="Trả lời trong luồng"
              onChange={(event) => setThreadDraft(event.target.value)}
              placeholder="Trả lời trong luồng..."
              value={threadDraft}
            />
            <Button aria-label="Gửi trả lời" disabled={isSendingThread || !threadDraft.trim()} size="sm" type="submit" variant="icon">
              <Send size={17} />
            </Button>
          </form>
        </section>
      ) : null}

      <div className="detail-tabs">
        <SegmentedControl aria-label="Tab thông tin kênh" onValueChange={onTabChange} options={detailTabs} value={activeTab} />
        <Tooltip label="Ẩn bảng thông tin">
          <Button aria-label="Ẩn bảng thông tin" onClick={onClose} variant="icon">
            <PanelRightClose size={18} />
          </Button>
        </Tooltip>
      </div>

      {activeTab === "pinned" ? (
        <section className="detail-section">
          <header>
            <h2>Tin nhắn đã ghim</h2>
          </header>
          {pinnedMessages.length ? (
            pinnedMessages.map((item) => (
              <article className="pinned-card" key={item.id}>
                <Avatar name={item.author.name} size="sm" src={item.author.avatarUrl} />
                <div>
                  <strong>{item.author.name}</strong>
                  <small>{item.date}</small>
                  <p>{item.text}</p>
                </div>
                <Pin size={16} />
              </article>
            ))
          ) : (
            <EmptyState description={`${currentChatLabel} chưa có tin nhắn được ghim.`} title="Chưa có tin ghim" />
          )}
        </section>
      ) : null}

      {activeTab === "media" ? (
        <section className="detail-section">
          <header>
            <h2>Ảnh & Hình ảnh</h2>
          </header>
          {isLoading ? (
            <PanelSkeleton />
          ) : mediaItems.length ? (
            <div className="media-grid">
              {mediaItems.map((item) => (
                <MediaGalleryThumbnail item={item} key={item.id} onOpen={onMediaSelect} onResolve={onResolveMedia} />
              ))}
            </div>
          ) : (
            <EmptyState description={`${currentChatLabel} chưa có ảnh nào được chia sẻ.`} title="Chưa có ảnh" />
          )}
        </section>
      ) : null}

      {activeTab === "files" ? (
        <section className="detail-section">
          <header>
            <h2>File gần đây</h2>
          </header>
          {isLoading ? (
            <PanelSkeleton />
          ) : files.length ? (
            files.map((file) => (
              <button className="file-row" key={file.id} onClick={() => onFileSelect(file)} type="button">
                <span className={`file-icon file-icon--${file.tone}`}>
                  <FileText size={18} />
                </span>
                <span>
                  <strong>{file.name}</strong>
                  <small>
                    {fileSecurityLabel(file.status) ?? `${file.size} - ${file.updatedAt}`}
                  </small>
                </span>
              </button>
            ))
          ) : (
            <EmptyState description={`${currentChatLabel} chưa có file nào được chia sẻ.`} title="Chưa có file" />
          )}
        </section>
      ) : null}
    </aside>
  );
}

function PanelSkeleton() {
  return (
    <div className="panel-skeleton">
      <Skeleton />
      <Skeleton />
      <Skeleton />
    </div>
  );
}

function TimelineSkeleton() {
  return (
    <div className="message-timeline">
      <Skeleton style={{ height: 58 }} />
      <Skeleton style={{ height: 72 }} />
      <Skeleton style={{ height: 58 }} />
    </div>
  );
}

function formatFileSize(size: number): string {
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }

  if (size < 1024 * 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }

  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function fileSecurityLabel(status?: string): string | null {
  switch (status?.trim().toLowerCase()) {
    case "scanning":
    case "pending_scan":
    case "pending":
      return "Đang quét bảo mật";
    case "quarantine":
    case "quarantined":
    case "blocked":
      return "File đang bị cách ly";
    case "infected":
      return "File không an toàn";
    default:
      return null;
  }
}

function fileSecurityTone(status?: string): "danger" | "warning" {
  const normalized = status?.trim().toLowerCase();
  return normalized === "infected" || normalized === "blocked" || normalized === "quarantine" || normalized === "quarantined"
    ? "danger"
    : "warning";
}

async function verifyBlobChecksum(blob: Blob, expectedSha256: string): Promise<boolean> {
  if (typeof crypto === "undefined" || !crypto.subtle) {
    return true;
  }

  const expected = expectedSha256.trim().toLowerCase();
  if (!expected) {
    return true;
  }

  const digest = await crypto.subtle.digest("SHA-256", await blob.arrayBuffer());
  const actual = Array.from(new Uint8Array(digest))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
  return actual === expected;
}

function contactResultFromRequest(request: ContactRequest, data: ChatWorkspaceData): ContactResult {
  const member = data.members.find((item) => item.user_id === request.user.id);
  const name = request.user.display_name || member?.display_name || request.user.username || member?.username || request.user.email || member?.email || request.user.id;

  return {
    avatarUrl: request.user.avatar_url || member?.avatar_url,
    contactDirection: request.direction,
    contactRequestId: request.id,
    contactStatus: "accepted",
    email: request.user.email ?? member?.email,
    hasConversation: data.directConversations.some((conversation) => conversation.user.id === request.user.id),
    isWorkspaceMember: Boolean(member),
    name,
    phoneNumber: request.user.phone_number || memberPhone(member),
    status: member?.status ?? request.user.status,
    userId: request.user.id,
    username: request.user.username ?? member?.username
  };
}

function buildContactResults({
  contacts,
  contactRequests,
  currentUserId,
  directConversations,
  members,
  query,
  searchUsers
}: {
  contacts: ContactRequest[];
  contactRequests: ContactRequest[];
  currentUserId?: string;
  directConversations: ChatWorkspaceData["directConversations"];
  members: WorkspaceMember[];
  query: string;
  searchUsers: AuthUser[];
}): ContactResult[] {
  const conversationUserIds = new Set(directConversations.map((conversation) => conversation.user.id));
  const memberByUserId = new Map(members.map((member) => [member.user_id, member]));
  const acceptedByUserId = new Map(contacts.map((contact) => [contact.user.id, contact]));
  const requestByUserId = new Map(contactRequests.map((request) => [request.user.id, request]));
  const statusFor = (userId: string) => {
    const accepted = acceptedByUserId.get(userId);
    const request = requestByUserId.get(userId);
    const activeRequest = accepted ?? request;

    return {
      contactDirection: activeRequest?.direction,
      contactRequestId: activeRequest?.id,
      contactStatus:
        accepted?.status === "accepted"
          ? ("accepted" as const)
          : request?.status === "pending"
            ? ("pending" as const)
            : request?.status === "rejected"
              ? ("rejected" as const)
              : ("none" as const)
    };
  };

  if (query.trim().length >= 2) {
    return searchUsers.filter((user) => user.id !== currentUserId).map((user) => {
      const member = memberByUserId.get(user.id);
      const contactState = statusFor(user.id);

      return {
        avatarUrl: user.avatar_url || member?.avatar_url,
        ...contactState,
        email: user.email ?? member?.email,
        hasConversation: conversationUserIds.has(user.id),
        isWorkspaceMember: Boolean(member),
        name: user.display_name || member?.display_name || user.username || member?.username || user.email || member?.email || user.id,
        phoneNumber: user.phone_number || memberPhone(member),
        status: member?.status ?? user.status,
        userId: user.id,
        username: user.username ?? member?.username
      };
    });
  }

  const results: ContactResult[] = [];
  const seenUserIds = new Set<string>();

  const pushResult = (result: ContactResult) => {
    if (!result.userId || result.userId === currentUserId || seenUserIds.has(result.userId)) {
      return;
    }
    seenUserIds.add(result.userId);
    results.push(result);
  };

  for (const request of [...contacts, ...contactRequests]) {
    const user = request.user;
    const member = memberByUserId.get(user.id);
    pushResult({
      avatarUrl: user.avatar_url || member?.avatar_url,
      ...statusFor(user.id),
      email: user.email ?? member?.email,
      hasConversation: conversationUserIds.has(user.id),
      isWorkspaceMember: Boolean(member),
      name: user.display_name || member?.display_name || user.username || member?.username || user.email || member?.email || user.id,
      phoneNumber: user.phone_number || memberPhone(member),
      status: member?.status ?? user.status,
      userId: user.id,
      username: user.username ?? member?.username
    });
  }

  for (const member of members) {
    pushResult({
      avatarUrl: member.avatar_url,
      ...statusFor(member.user_id),
      email: member.email,
      hasConversation: conversationUserIds.has(member.user_id),
      isWorkspaceMember: true,
      name: memberName(member),
      phoneNumber: memberPhone(member),
      status: member.status,
      userId: member.user_id,
      username: member.username
    });
  }

  return results;
}

function memberName(member?: WorkspaceMember): string {
  return member?.display_name || member?.username || member?.email || member?.user_id || "";
}

function formValue(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

function memberPhone(member?: WorkspaceMember): string {
  if (!member) {
    return "";
  }

  const extendedMember = member as WorkspaceMember & {
    mobile?: string | null;
    phone?: string | null;
    phone_number?: string | null;
  };

  return extendedMember.phone || extendedMember.phone_number || extendedMember.mobile || "";
}

function slugify(value: string): string {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);
}

function railItemFromRoute(pathname: string, searchParams?: { get: (name: string) => string | null }): RailItemId {
  const section = parseChatRoute(pathname, searchParams)?.sectionRef;
  return railItems.some((item) => item.id === section) ? section as RailItemId : "messages";
}

function messageSidebarTabFromRoute(
  pathname: string,
  searchParams?: { get: (name: string) => string | null }
): MessageSidebarTab {
  const route = parseChatRoute(pathname, searchParams);
  if (route?.kind === "channel" || route?.messageView === "channels") {
    return "channels";
  }
  return "conversations";
}

function shouldOpenExternally(link: HTMLAnchorElement): boolean {
  try {
    const url = new URL(link.href);
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      return false;
    }
    return link.target === "_blank" || url.origin !== window.location.origin;
  } catch {
    return false;
  }
}

function BrandedQRCode({ alt, className = "", src }: { alt: string; className?: string; src: string }) {
  return (
    <span className={`branded-qr${className ? ` ${className}` : ""}`}>
      <img alt={alt} className="branded-qr__image" src={src} />
      <span className="branded-qr__logo" aria-hidden="true">
        <img alt="" src="/brand/vpsttt-logo.png" />
      </span>
    </span>
  );
}

function adminPanelUrl(): string {
  try {
    return `${new URL(runtimeEnvironment.apiBaseUrl).origin}/admin`;
  } catch {
    return "https://chat.vpsttt.com/admin";
  }
}

function stripDisplayedQRURL(body: string) {
  return body
    .split("\n")
    .filter((line) => !/^\s*QR\s*:\s*https?:\/\/\S+\s*$/i.test(line))
    .join("\n")
    .trim();
}

function canAccessRailItem(itemId: RailItemId, can: (permission: string) => boolean) {
  switch (itemId) {
    case "departments":
      return can("workspace.manage");
    case "tickets":
      return can("ticket.view");
    case "files":
      return can("admin.view");
    case "bots":
      return can("bot.manage");
    case "automation":
      return can("webhook.manage") || can("cronjob.manage") || can("module.manage");
    default:
      return true;
  }
}
