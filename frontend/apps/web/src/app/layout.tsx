import type { Metadata } from "next";
import "@webtui/ui/styles.css";
import "./globals.css";
import { AppProviders } from "./providers";

const instanceName = process.env.NEXT_PUBLIC_APP_NAME?.trim() || "Ứng dụng chat";

export const metadata: Metadata = {
  title: instanceName,
  description: "Nền tảng chat nội bộ tự host cho doanh nghiệp Việt."
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="vi" suppressHydrationWarning>
      <body>
        <AppProviders>{children}</AppProviders>
      </body>
    </html>
  );
}
