import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { PermissionGate } from "../../apps/web/src/features/workspace/components/permission-gate";

describe("PermissionGate", () => {
  it("renders children when permission is allowed", () => {
    const html = renderToStaticMarkup(
      <PermissionGate allowed permission="message.send">
        <button>Gửi tin nhắn</button>
      </PermissionGate>
    );

    expect(html).toContain("Gửi tin nhắn");
  });

  it("renders fallback when permission is denied", () => {
    const html = renderToStaticMarkup(
      <PermissionGate allowed={false} fallback={<p>Chưa đủ quyền</p>} permission="admin.view">
        <button>Quản trị</button>
      </PermissionGate>
    );

    expect(html).toContain("Chưa đủ quyền");
    expect(html).not.toContain("Quản trị");
  });
});
