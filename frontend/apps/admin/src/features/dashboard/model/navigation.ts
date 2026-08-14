export const adminPageMeta = {
  overview: {
    description: "Theo dõi sức khỏe, hoạt động và các chỉ số quan trọng của workspace.",
    label: "Tổng quan",
    title: "Tổng quan hệ thống"
  },
  push: {
    description: "Theo dõi hàng đợi, tỷ lệ giao thành công và xử lý dead-letter push.",
    label: "Push notification",
    title: "Vận hành push notification"
  },
  messages: {
    description: "Tìm kiếm và giám sát luồng tin nhắn trong toàn workspace.",
    label: "Tin nhắn",
    title: "Quản trị tin nhắn"
  },
  channels: {
    description: "Quản lý kênh nhóm, kênh riêng và các phiên bot riêng tư.",
    label: "Kênh",
    title: "Quản trị kênh"
  },
  moderation: {
    description: "Tiếp nhận, xem xét và lưu kết quả xử lý báo cáo nội dung do người dùng tạo.",
    label: "Kiểm duyệt",
    title: "Hàng đợi kiểm duyệt"
  },
  users: {
    description: "Quản lý tài khoản, lời mời và trạng thái truy cập workspace.",
    label: "Người dùng",
    title: "Quản trị người dùng"
  },
  roles: {
    description: "Thiết lập vai trò và quyền hạn theo nguyên tắc đặc quyền tối thiểu.",
    label: "Vai trò & quyền",
    title: "Vai trò và phân quyền"
  },
  integrations: {
    description: "Quản lý API token, webhook và các kết nối dịch vụ bên ngoài.",
    label: "Tích hợp",
    title: "Tích hợp hệ thống"
  },
  automations: {
    description: "Cài đặt workflow, connector và bot theo cấu hình của zone hiện tại.",
    label: "Automation",
    title: "Automation theo zone"
  },
  bots: {
    description: "Quản lý bot, phạm vi cài đặt và hoạt động trong workspace.",
    label: "Bot",
    title: "Quản trị bot"
  },
  cronjobs: {
    description: "Lập lịch, theo dõi và vận hành các tác vụ tự động.",
    label: "Tác vụ định kỳ",
    title: "Tác vụ định kỳ"
  },
  backups: {
    description: "Quản lý lịch sao lưu, lần chạy và trạng thái bảo vệ dữ liệu.",
    label: "Sao lưu",
    title: "Sao lưu dữ liệu"
  },
  settings: {
    description: "Cấu hình workspace, zone, quota, domain và đăng nhập doanh nghiệp.",
    label: "Cài đặt",
    title: "Cài đặt hệ thống"
  }
} as const;

export type AdminNavId = keyof typeof adminPageMeta;

export const enabledAdminSections = [
  "overview",
  "push",
  "moderation",
  "messages",
  "channels",
  "users",
  "roles",
  "cronjobs",
  "backups",
  "settings"
] as const satisfies readonly AdminNavId[];

export const adminNavigationGroups: ReadonlyArray<{
  id: string;
  label: string;
  items: readonly AdminNavId[];
}> = [
  { id: "monitoring", label: "Giám sát", items: ["overview", "push"] },
  { id: "workspace", label: "Workspace", items: ["moderation", "messages", "channels", "users", "roles"] },
  { id: "operations", label: "Vận hành", items: ["cronjobs", "backups", "settings"] }
];

export const adminSectionRequiredPermissions: Partial<Record<AdminNavId, string>> = {
  moderation: "moderation.manage"
};

export function canAccessAdminSection(
  section: AdminNavId,
  hasPermission: (permission: string) => boolean
): boolean {
  const permission = adminSectionRequiredPermissions[section];
  return !permission || hasPermission(permission);
}

export function isAdminSectionEnabled(section: AdminNavId): boolean {
  return (enabledAdminSections as readonly AdminNavId[]).includes(section);
}

export function resolveAdminSection(value: string | null | undefined): AdminNavId {
  if (!value || !Object.hasOwn(adminPageMeta, value)) {
    return "overview";
  }
  const section = value as AdminNavId;
  return isAdminSectionEnabled(section) ? section : "overview";
}

export function adminSectionGroup(section: AdminNavId): string {
  return adminNavigationGroups.find((group) => group.items.includes(section))?.label ?? "Quản trị";
}
