import js from "@eslint/js";
import tseslint from "typescript-eslint";

const nextConfig = [
  {
    ignores: [
      ".next/**",
      "dist/**",
      "out/**",
      "coverage/**",
      "node_modules/**"
    ]
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "warn"
    }
  }
];

export default nextConfig;
