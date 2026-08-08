import { createLegalPolicyConfig } from "@webtui/api-client";

export const legalPolicyConfig = createLegalPolicyConfig({
  NEXT_PUBLIC_PRIVACY_URL: process.env.NEXT_PUBLIC_PRIVACY_URL,
  NEXT_PUBLIC_PRIVACY_VERSION: process.env.NEXT_PUBLIC_PRIVACY_VERSION,
  NEXT_PUBLIC_TERMS_URL: process.env.NEXT_PUBLIC_TERMS_URL,
  NEXT_PUBLIC_TERMS_VERSION: process.env.NEXT_PUBLIC_TERMS_VERSION
});
