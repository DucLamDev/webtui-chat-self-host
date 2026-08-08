import type {
  CurrentLegalAcceptance,
  CurrentLegalDocuments,
  LegalDocumentVersion
} from "@webtui/types";

export type LegalPolicySource = {
  NEXT_PUBLIC_PRIVACY_URL?: string;
  NEXT_PUBLIC_PRIVACY_VERSION?: string;
  NEXT_PUBLIC_TERMS_URL?: string;
  NEXT_PUBLIC_TERMS_VERSION?: string;
};

export type LegalPolicyConfig = {
  configurationError: string | null;
  privacyUrl: string;
  privacyVersion: string;
  termsUrl: string;
  termsVersion: string;
};

export type LegalDocumentsResolution = {
  documents: CurrentLegalDocuments | null;
  error: string | null;
};

/**
 * Policy metadata is intentionally never defaulted. A build without explicit
 * published URLs and versions stays usable for sign-in/read-only access, but
 * cannot collect consent or create an account.
 */
export function createLegalPolicyConfig(source: LegalPolicySource): LegalPolicyConfig {
  const termsUrl = source.NEXT_PUBLIC_TERMS_URL?.trim() ?? "";
  const privacyUrl = source.NEXT_PUBLIC_PRIVACY_URL?.trim() ?? "";
  const termsVersion = source.NEXT_PUBLIC_TERMS_VERSION?.trim() ?? "";
  const privacyVersion = source.NEXT_PUBLIC_PRIVACY_VERSION?.trim() ?? "";
  const problems = [
    validatePolicyUrl("NEXT_PUBLIC_TERMS_URL", termsUrl),
    validatePolicyUrl("NEXT_PUBLIC_PRIVACY_URL", privacyUrl),
    validatePolicyVersion("NEXT_PUBLIC_TERMS_VERSION", termsVersion),
    validatePolicyVersion("NEXT_PUBLIC_PRIVACY_VERSION", privacyVersion)
  ].filter((problem): problem is string => Boolean(problem));

  return {
    configurationError: problems.length ? problems.join(" ") : null,
    privacyUrl,
    privacyVersion,
    termsUrl,
    termsVersion
  };
}

export function resolveCurrentLegalDocuments(
  values: readonly LegalDocumentVersion[] | null | undefined
): LegalDocumentsResolution {
  if (!values) {
    return { documents: null, error: "Chưa tải được phiên bản tài liệu pháp lý từ máy chủ." };
  }
  const terms = values.filter((value) => value.document_type === "terms");
  const privacy = values.filter((value) => value.document_type === "privacy");
  if (terms.length !== 1 || privacy.length !== 1) {
    return {
      documents: null,
      error: "Máy chủ phải công bố đúng một phiên bản Điều khoản và một phiên bản Chính sách quyền riêng tư."
    };
  }
  if (!isValidDocument(terms[0]) || !isValidDocument(privacy[0])) {
    return { documents: null, error: "Máy chủ trả về tài liệu pháp lý không hợp lệ." };
  }
  if (
    !terms[0].includes.includes("terms_of_use")
    || !terms[0].includes.includes("acceptable_use_policy")
    || !privacy[0].includes.includes("privacy_policy")
  ) {
    return {
      documents: null,
      error: "Tài liệu pháp lý trên máy chủ không công bố đủ Điều khoản, Quy tắc sử dụng và Chính sách quyền riêng tư."
    };
  }
  return { documents: { privacy: privacy[0], terms: terms[0] }, error: null };
}

export function legalDocumentsCompatibilityError(
  documents: CurrentLegalDocuments,
  config: LegalPolicyConfig
): string | null {
  if (config.configurationError) {
    return config.configurationError;
  }
  if (documents.terms.version !== config.termsVersion) {
    return `Phiên bản Điều khoản trên máy chủ (${documents.terms.version}) không khớp bản đã công bố (${config.termsVersion}).`;
  }
  if (documents.privacy.version !== config.privacyVersion) {
    return `Phiên bản Chính sách quyền riêng tư trên máy chủ (${documents.privacy.version}) không khớp bản đã công bố (${config.privacyVersion}).`;
  }
  return null;
}

export function legalAcceptanceCompatibilityError(
  acceptance: CurrentLegalAcceptance,
  documents: CurrentLegalDocuments,
  config: LegalPolicyConfig,
  expectedWorkspaceId: string
): string | null {
  if (!expectedWorkspaceId.trim() || acceptance.workspace_id !== expectedWorkspaceId) {
    return `Trạng thái pháp lý thuộc workspace không mong đợi (${acceptance.workspace_id || "không xác định"}); cần workspace ${expectedWorkspaceId || "không xác định"}.`;
  }
  const documentError = legalDocumentsCompatibilityError(documents, config);
  if (documentError) {
    return documentError;
  }
  if (acceptance.terms.version !== documents.terms.version) {
    return `Phiên bản Điều khoản trong trạng thái chấp nhận (${acceptance.terms.version}) không khớp tài liệu hiện hành (${documents.terms.version}).`;
  }
  if (acceptance.privacy.version !== documents.privacy.version) {
    return `Phiên bản Chính sách quyền riêng tư trong trạng thái chấp nhận (${acceptance.privacy.version}) không khớp tài liệu hiện hành (${documents.privacy.version}).`;
  }
  if (acceptance.complete && (!acceptance.terms.accepted || !acceptance.privacy.accepted)) {
    return "Máy chủ báo trạng thái pháp lý hoàn tất nhưng thiếu một xác nhận bắt buộc.";
  }
  if (acceptance.terms.accepted && !isValidAcceptanceTimestamp(acceptance.terms.accepted_at)) {
    return "Xác nhận Điều khoản thiếu thời điểm ghi nhận hợp lệ.";
  }
  if (acceptance.privacy.accepted && !isValidAcceptanceTimestamp(acceptance.privacy.accepted_at)) {
    return "Xác nhận Chính sách quyền riêng tư thiếu thời điểm ghi nhận hợp lệ.";
  }
  if (acceptance.complete && (!acceptance.terms.accepted_at || !acceptance.privacy.accepted_at)) {
    return "Trạng thái pháp lý hoàn tất nhưng thiếu bằng chứng thời gian chấp nhận.";
  }
  return null;
}

export function isCompleteLegalAcceptance(acceptance: CurrentLegalAcceptance): boolean {
  return Boolean(
    acceptance.complete
      && acceptance.terms.accepted
      && acceptance.privacy.accepted
      && isValidAcceptanceTimestamp(acceptance.terms.accepted_at)
      && isValidAcceptanceTimestamp(acceptance.privacy.accepted_at)
  );
}

function isValidDocument(document: LegalDocumentVersion): boolean {
  return (
    validatePolicyVersion("version", document.version) === null
    && Array.isArray(document.includes)
    && document.includes.every((item) => typeof item === "string" && Boolean(item.trim()))
  );
}

function isValidAcceptanceTimestamp(value: string | null | undefined): boolean {
  return typeof value === "string" && Boolean(value.trim()) && Number.isFinite(Date.parse(value));
}

function validatePolicyUrl(name: string, value: string): string | null {
  try {
    const url = new URL(value);
    if (url.protocol !== "https:" || url.username || url.password || url.search || url.hash) {
      throw new Error("unsafe URL");
    }
    return null;
  } catch {
    return `${name} phải là URL HTTPS công khai, không chứa credential, query hoặc fragment.`;
  }
}

function validatePolicyVersion(name: string, value: string): string | null {
  const normalized = value.toLowerCase();
  const placeholder = ["change_me", "changeme", "todo", "latest", "dev", "development", "unknown"].includes(normalized);
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(value) || placeholder) {
    return `${name} phải là version policy hợp lệ, không dùng placeholder.`;
  }
  return null;
}
