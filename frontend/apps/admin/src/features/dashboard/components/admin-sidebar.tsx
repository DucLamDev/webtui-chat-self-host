"use client";

import type { ComponentType } from "react";
import { Avatar } from "@webtui/ui";
import {
  Activity,
  Bell,
  Bot,
  CalendarClock,
  Database,
  Hash,
  MessageCircle,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  ShieldCheck,
  Users,
  Workflow,
  X,
  Zap
} from "@webtui/icons";
import {
  adminNavigationGroups,
  adminPageMeta,
  type AdminNavId
} from "../model/navigation";

type NavigationIcon = ComponentType<{ "aria-hidden"?: boolean; size?: number }>;

const navigationIcons: Record<AdminNavId, NavigationIcon> = {
  automations: Zap,
  backups: Database,
  bots: Bot,
  channels: Hash,
  cronjobs: CalendarClock,
  integrations: Workflow,
  messages: MessageCircle,
  overview: Activity,
  push: Bell,
  roles: ShieldCheck,
  settings: Settings,
  users: Users
};

export function AdminSidebar({
  activeId,
  collapsed,
  mobileOpen,
  onCloseMobile,
  onPrefetch,
  onSelect,
  onToggleCollapsed,
  organization,
  profile
}: {
  activeId: AdminNavId;
  collapsed: boolean;
  mobileOpen: boolean;
  onCloseMobile: () => void;
  onPrefetch: (id: AdminNavId) => void;
  onSelect: (id: AdminNavId) => void;
  onToggleCollapsed: () => void;
  organization: { logo?: string; name: string };
  profile: { name: string; src?: string; status: "online" | "offline" | "busy" };
}) {
  return (
    <>
      <button
        aria-label="Đóng menu quản trị"
        className={`admin-sidebar-backdrop${mobileOpen ? " admin-sidebar-backdrop--visible" : ""}`}
        onClick={onCloseMobile}
        type="button"
      />
      <aside
        className={`admin-sidebar${collapsed ? " admin-sidebar--collapsed" : ""}${mobileOpen ? " admin-sidebar--mobile-open" : ""}`}
        id="admin-navigation"
      >
        <div className="admin-sidebar__brand">
          <span aria-hidden="true" className="admin-sidebar__brand-mark">
            {organization.logo ? (
              <img alt="" src={organization.logo} />
            ) : (
              organization.name.slice(0, 1).toUpperCase()
            )}
          </span>
          <span className="admin-sidebar__brand-copy">
            <strong>{organization.name}</strong>
            <small>Trang quản trị</small>
          </span>
          <button
            aria-label="Đóng menu"
            className="admin-sidebar__mobile-close"
            onClick={onCloseMobile}
            type="button"
          >
            <X aria-hidden size={18} />
          </button>
        </div>

        <nav aria-label="Điều hướng quản trị" className="admin-sidebar__navigation">
          {adminNavigationGroups.map((group) => (
            <section className="admin-sidebar__group" key={group.id}>
              <h2>{group.label}</h2>
              <div>
                {group.items.map((id) => {
                  const Icon = navigationIcons[id];
                  const active = id === activeId;
                  return (
                    <button
                      aria-current={active ? "page" : undefined}
                      className={`admin-sidebar__item${active ? " admin-sidebar__item--active" : ""}`}
                      key={id}
                      onFocus={() => onPrefetch(id)}
                      onClick={() => onSelect(id)}
                      onPointerEnter={() => onPrefetch(id)}
                      title={collapsed ? adminPageMeta[id].label : undefined}
                      type="button"
                    >
                      <Icon aria-hidden size={19} />
                      <span>{adminPageMeta[id].label}</span>
                    </button>
                  );
                })}
              </div>
            </section>
          ))}
        </nav>

        <footer className="admin-sidebar__footer">
          <div className="admin-sidebar__profile">
            <Avatar name={profile.name} size="sm" src={profile.src} status={profile.status} />
            <span>
              <strong>{profile.name}</strong>
              <small>Quản trị viên</small>
            </span>
          </div>
          <button
            aria-label={collapsed ? "Mở rộng thanh điều hướng" : "Thu gọn thanh điều hướng"}
            className="admin-sidebar__collapse"
            onClick={onToggleCollapsed}
            title={collapsed ? "Mở rộng" : "Thu gọn"}
            type="button"
          >
            {collapsed ? <PanelLeftOpen aria-hidden size={18} /> : <PanelLeftClose aria-hidden size={18} />}
          </button>
        </footer>
      </aside>
    </>
  );
}
