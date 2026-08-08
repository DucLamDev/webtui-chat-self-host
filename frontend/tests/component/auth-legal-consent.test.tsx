import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { AuthScreen } from "@webtui/ui";

describe("registration legal consent", () => {
  it("renders both immutable policy versions and keeps registration disabled before consent", () => {
    const html = renderToStaticMarkup(
      <AuthScreen
        mode="register"
        onLogin={vi.fn()}
        onModeChange={vi.fn()}
        onRegister={vi.fn()}
        registrationLegal={{
          privacyUrl: "https://download.vpsttt.com/privacy",
          privacyVersion: "2026-08-07",
          termsUrl: "https://download.vpsttt.com/terms",
          termsVersion: "2026-08-07"
        }}
      />
    );

    expect(html).toContain("Điều khoản &amp; Quy tắc sử dụng");
    expect(html).toContain("Chính sách quyền riêng tư");
    expect(html.match(/bản 2026-08-07/g)).toHaveLength(2);
    expect(html.match(/type="checkbox"/g)).toHaveLength(2);
    expect(html).toMatch(/auth-submit[^>]*disabled/);
  });

  it("shows discovery failure and a retry action without fabricating versions", () => {
    const html = renderToStaticMarkup(
      <AuthScreen
        mode="register"
        onLogin={vi.fn()}
        onModeChange={vi.fn()}
        onRegister={vi.fn()}
        registrationLegal={{
          error: "Không tải được tài liệu pháp lý từ máy chủ.",
          onRetry: vi.fn(),
          privacyUrl: "",
          termsUrl: ""
        }}
      />
    );

    expect(html).toContain("Không tải được tài liệu pháp lý từ máy chủ.");
    expect(html).toContain("Thử tải lại");
    expect(html).not.toContain("bản 2026-08-07");
  });
});
