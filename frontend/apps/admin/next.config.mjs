import process from "node:process";

const adminBasePath = process.env.ADMIN_BASE_PATH?.trim();

const nextConfig = {
  allowedDevOrigins: ["127.0.0.1"],
  reactStrictMode: true,
  ...(adminBasePath ? { basePath: adminBasePath } : {}),
  transpilePackages: [
    "@webtui/api-client",
    "@webtui/icons",
    "@webtui/types",
    "@webtui/ui"
  ]
};

export default nextConfig;
