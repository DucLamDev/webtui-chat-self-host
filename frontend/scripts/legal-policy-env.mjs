const versionPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const versionPlaceholders = new Set([
  "change_me",
  "changeme",
  "todo",
  "latest",
  "dev",
  "development",
  "unknown"
]);

export function assertLegalPolicyBuildEnvironment(environment, appName) {
  const termsUrl = required(environment, "NEXT_PUBLIC_TERMS_URL", appName);
  const privacyUrl = required(environment, "NEXT_PUBLIC_PRIVACY_URL", appName);
  const termsVersion = required(environment, "NEXT_PUBLIC_TERMS_VERSION", appName);
  const privacyVersion = required(environment, "NEXT_PUBLIC_PRIVACY_VERSION", appName);

  validateUrl(termsUrl, "NEXT_PUBLIC_TERMS_URL", appName);
  validateUrl(privacyUrl, "NEXT_PUBLIC_PRIVACY_URL", appName);
  validateVersion(termsVersion, "NEXT_PUBLIC_TERMS_VERSION", appName);
  validateVersion(privacyVersion, "NEXT_PUBLIC_PRIVACY_VERSION", appName);
}

function required(environment, name, appName) {
  const value = environment[name]?.trim();
  if (!value) {
    throw new Error(`[${appName}] ${name} is required for a production build; legal consent cannot use a fallback.`);
  }
  return value;
}

function validateUrl(value, name, appName) {
  try {
    const url = new URL(value);
    if (url.protocol !== "https:" || url.username || url.password || url.search || url.hash) {
      throw new Error("unsafe URL");
    }
  } catch {
    throw new Error(`[${appName}] ${name} must be a public HTTPS URL without credentials, query, or fragment.`);
  }
}

function validateVersion(value, name, appName) {
  if (!versionPattern.test(value) || versionPlaceholders.has(value.toLowerCase())) {
    throw new Error(`[${appName}] ${name} must be an explicit, non-placeholder policy version.`);
  }
}
