import type { Metadata } from "next";
import "@webtui/ui/styles.css";
import "./globals.css";
import { AppProviders } from "./providers";

export const metadata: Metadata = {
  title: "Quản trị WebTui Chat",
  description: "Bảng quản trị hệ thống chat nội bộ WebTui."
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
