import process from "node:process";
import { fileURLToPath, URL } from "node:url";
import { assertLegalPolicyBuildEnvironment } from "../../scripts/legal-policy-env.mjs";

const adminBasePath = process.env.ADMIN_BASE_PATH?.trim();

if (process.env.NODE_ENV === "production") {
  assertLegalPolicyBuildEnvironment(process.env, "admin");
}

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
