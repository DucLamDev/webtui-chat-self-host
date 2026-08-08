import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AdminSidebar } from "../../apps/admin/src/features/dashboard/components/admin-sidebar";

const commonProps = {
  activeId: "overview" as const,
  collapsed: false,
  mobileOpen: false,
  onCloseMobile: () => undefined,
  onPrefetch: () => undefined,
  onSelect: () => undefined,
  onToggleCollapsed: () => undefined,
  organization: { name: "WebTUI" },
  profile: { name: "Moderator", status: "online" as const }
};

describe("admin moderation navigation", () => {
  it("hides moderation when moderation.manage is absent", () => {
    const html = renderToStaticMarkup(
      <AdminSidebar {...commonProps} isItemVisible={(section) => section !== "moderation"} />
    );

    expect(html).not.toContain("Kiểm duyệt");
    expect(html).toContain("Tin nhắn");
  });

  it("shows moderation when moderation.manage is present", () => {
    const html = renderToStaticMarkup(<AdminSidebar {...commonProps} />);

    expect(html).toContain("Kiểm duyệt");
  });
});
