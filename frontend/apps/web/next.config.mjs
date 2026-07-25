/* global process */

const isTauriBuild = process.env.TAURI_BUILD === "1";

const nextConfig = {
  allowedDevOrigins: ["127.0.0.1"],
  reactStrictMode: true,
  transpilePackages: [
    "@webtui/api-client",
    "@webtui/chat-core",
    "@webtui/icons",
    "@webtui/types",
    "@webtui/ui"
  ]
};

if (isTauriBuild) {
  nextConfig.output = "export";
  nextConfig.images = { unoptimized: true };
  nextConfig.trailingSlash = true;
}

export default nextConfig;
