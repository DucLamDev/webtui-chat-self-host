export type AdminUserStatus = "active" | "blocked";

export type AdminUserFilter = "all" | "active" | "blocked";

export type AdminUser = {
  id: string;
  name: string;
  email: string;
  department: string;
  role: string;
  status: AdminUserStatus;
};

export type DashboardMetric = {
  label: string;
  value: string;
  delta: string;
  tone: "blue" | "green" | "orange" | "purple";
};

export type ChannelRank = {
  id: string;
  name: string;
  count: string;
  tone: "blue" | "green" | "red" | "orange" | "purple";
};

export type CacheSetting = {
  key: string;
  label: string;
  value: string;
};
