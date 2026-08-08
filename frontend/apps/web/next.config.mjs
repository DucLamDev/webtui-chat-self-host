/* global process */

import { fileURLToPath, URL } from "node:url";
import { assertLegalPolicyBuildEnvironment } from "../../scripts/legal-policy-env.mjs";

const isTauriBuild = process.env.TAURI_BUILD === "1";

if (process.env.NODE_ENV === "production") {
  assertLegalPolicyBuildEnvironment(process.env, "web");
}

const nextConfig = {
  allowedDevOrigins: ["127.0.0.1"],
  reactStrictMode: true,
  turbopack: { root: fileURLToPath(new URL("../..", import.meta.url)) },
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
