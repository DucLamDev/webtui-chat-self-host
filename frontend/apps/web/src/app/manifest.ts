import type { MetadataRoute } from "next";

const appName = process.env.NEXT_PUBLIC_APP_NAME?.trim() || "WebTui Chat";
const shortName = appName.length > 18 ? "WebTui Chat" : appName;

export default function manifest(): MetadataRoute.Manifest {
  return {
    background_color: "#f4f7fb",
    description: "Nền tảng chat nội bộ tự host.",
    display: "standalone",
    icons: [
      {
        src: "/brand/logo_webtui.png",
        sizes: "512x512",
        type: "image/png"
      },
      {
        purpose: "maskable",
        src: "/brand/logo_webtui.png",
        sizes: "512x512",
        type: "image/png"
      }
    ],
    id: "/",
    lang: "vi",
    name: appName,
    orientation: "portrait",
    scope: "/",
    short_name: shortName,
    start_url: "/",
    theme_color: "#0b74de"
  };
}
