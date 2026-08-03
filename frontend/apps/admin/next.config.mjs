import process from "node:process";
import { fileURLToPath, URL } from "node:url";

const adminBasePath = process.env.ADMIN_BASE_PATH?.trim();

const nextConfig = {
  allowedDevOrigins: ["127.0.0.1"],
  reactStrictMode: true,
  turbopack: { root: fileURLToPath(new URL("../..", import.meta.url)) },
  ...(adminBasePath ? { basePath: adminBasePath } : {}),
  transpilePackages: [
    "@webtui/api-client",
    "@webtui/icons",
    "@webtui/types",
    "@webtui/ui"
  ]
};

export default nextConfig;
