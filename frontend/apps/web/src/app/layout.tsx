import type { Metadata, Viewport } from "next";
import "@webtui/ui/styles.css";
import "./globals.css";
import { AppProviders } from "./providers";

const instanceName = process.env.NEXT_PUBLIC_APP_NAME?.trim() || "Ứng dụng chat";
const appIcon = "/brand/logo_webtui.png";

export const metadata: Metadata = {
  applicationName: instanceName,
  manifest: "/manifest.webmanifest",
  title: instanceName,
  description: "Nền tảng chat nội bộ tự host cho doanh nghiệp Việt.",
  formatDetection: {
    telephone: false
  },
  icons: {
    apple: [{ url: appIcon, sizes: "512x512", type: "image/png" }],
    icon: [{ url: appIcon, sizes: "512x512", type: "image/png" }]
  },
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: instanceName
  }
};

export const viewport: Viewport = {
  initialScale: 1,
  themeColor: "#0b74de",
  viewportFit: "cover",
  width: "device-width"
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
