"use client";

import {
  type FormEvent,
  type ReactNode,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore
} from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  isCompleteLegalAcceptance,
  legalAcceptanceCompatibilityError,
  legalDocumentsCompatibilityError,
  queryKeys,
  resolveCurrentLegalDocuments
} from "@webtui/api-client";
import { Button } from "@webtui/ui";
import { getPlatformServices } from "@webtui/chat-core";
import { api, runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "./auth-store";
import {
  createLegalAcceptanceScope,
  legalAcceptanceGate,
  type LegalGateSnapshot
} from "./legal-acceptance-gate";
import { legalPolicyConfig } from "./legal-policy-config";

export function LegalAcceptanceProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const userId = useAuthStore((state) => state.user?.id ?? null);
  const workspaceId = useAuthStore((state) => state.workspaceId);
  const server = useAuthStore((state) => state.zoneRuntime?.api_base_url)
    ?? runtimeEnvironment.apiBaseUrl;
  const scope = createLegalAcceptanceScope(server, userId, workspaceId);
  const gateSnapshot = useSyncExternalStore(
    legalAcceptanceGate.subscribe,
    legalAcceptanceGate.getSnapshot,
    legalAcceptanceGate.getSnapshot
  );
  const handledServerRevision = useRef(0);
  const [privacyAccepted, setPrivacyAccepted] = useState(false);
  const [serverRefreshPending, setServerRefreshPending] = useState(false);
  const [termsAccepted, setTermsAccepted] = useState(false);

  const documentsQuery = useQuery({
    enabled: Boolean(scope),
    queryFn: () => api.auth.legalDocuments(),
    queryKey: queryKeys.auth.legalDocuments(server),
    refetchOnWindowFocus: "always",
    retry: false
  });
  const acceptanceQuery = useQuery({
    enabled: Boolean(scope),
    queryFn: () => api.auth.legalAcceptance(workspaceId as string),
    queryKey: queryKeys.auth.legalAcceptance(scope ?? "unresolved"),
    refetchOnWindowFocus: "always",
    retry: false
  });
  const documentsResolution = useMemo(
    () => resolveCurrentLegalDocuments(documentsQuery.data),
    [documentsQuery.data]
  );
  const documents = documentsResolution.documents;
  const validationError = legalPolicyConfig.configurationError
    ?? (!documentsQuery.isLoading && !documentsQuery.isError ? documentsResolution.error : null)
    ?? (documents ? legalDocumentsCompatibilityError(documents, legalPolicyConfig) : null)
    ?? (documents && acceptanceQuery.data && workspaceId
      ? legalAcceptanceCompatibilityError(acceptanceQuery.data, documents, legalPolicyConfig, workspaceId)
      : null);
  const queryError = documentsQuery.error ?? acceptanceQuery.error;
  const isChecking = !scope
    || serverRefreshPending
    || documentsQuery.isLoading
    || documentsQuery.isFetching
    || acceptanceQuery.isLoading
    || acceptanceQuery.isFetching;

  useEffect(() => {
    setPrivacyAccepted(false);
    setTermsAccepted(false);
  }, [documents?.privacy.version, documents?.terms.version, scope]);

  useEffect(() => {
    setServerRefreshPending(false);
    if (!scope) {
      legalAcceptanceGate.reset();
      return;
    }
    legalAcceptanceGate.markChecking(scope);
  }, [scope]);

  useEffect(() => {
    if (
      !scope
      || gateSnapshot.scope !== scope
      || gateSnapshot.reason !== "server"
      || gateSnapshot.revision === handledServerRevision.current
    ) {
      return;
    }
    handledServerRevision.current = gateSnapshot.revision;
    const revision = gateSnapshot.revision;
    setServerRefreshPending(true);
    legalAcceptanceGate.markChecking(scope);
    void Promise.all([documentsQuery.refetch(), acceptanceQuery.refetch()]).finally(() => {
      if (handledServerRevision.current === revision) setServerRefreshPending(false);
    });
  }, [acceptanceQuery, documentsQuery, gateSnapshot, scope]);

  useEffect(() => {
    if (!scope) {
      return;
    }
    if (gateSnapshot.scope === scope && gateSnapshot.reason === "server") {
      return;
    }
    if (gateSnapshot.scope === scope && gateSnapshot.kind === "unavailable") {
      return;
    }
    if (isChecking) {
      legalAcceptanceGate.markChecking(scope);
      return;
    }
    if (queryError) {
      legalAcceptanceGate.markUnavailable(
        scope,
        queryError instanceof Error ? queryError.message : "Không thể kiểm tra trạng thái pháp lý."
      );
      return;
    }
    if (validationError) {
      legalAcceptanceGate.markMismatch(scope, validationError);
      return;
    }
    if (acceptanceQuery.data && isCompleteLegalAcceptance(acceptanceQuery.data)) {
      legalAcceptanceGate.markComplete(scope);
      return;
    }
    legalAcceptanceGate.markRequired(scope);
  }, [acceptanceQuery.data, gateSnapshot.kind, gateSnapshot.reason, gateSnapshot.scope, isChecking, queryError, scope, validationError]);

  const acceptanceMutation = useMutation({
    mutationFn: async () => {
      if (!scope || !documents || validationError || !privacyAccepted || !termsAccepted) {
        throw new Error("Hãy chấp nhận đầy đủ tài liệu pháp lý hiện hành trước khi tiếp tục.");
      }
      return api.auth.acceptLegalDocuments({
        privacy_accepted: true,
        privacy_version: documents.privacy.version,
        terms_accepted: true,
        terms_version: documents.terms.version,
        workspace_id: workspaceId as string
      });
    },
    onSuccess: (acceptance) => {
      if (!scope) return;
      queryClient.setQueryData(queryKeys.auth.legalAcceptance(scope), acceptance);
      void queryClient.invalidateQueries({ queryKey: queryKeys.auth.legalAcceptance(scope) });
    }
  });

  const visibleSnapshot = gateSnapshot.scope === scope
    ? gateSnapshot
    : ({ kind: "checking", revision: gateSnapshot.revision, scope } satisfies LegalGateSnapshot);

  function retry() {
    if (!scope) return;
    setServerRefreshPending(true);
    legalAcceptanceGate.markChecking(scope);
    void Promise.all([documentsQuery.refetch(), acceptanceQuery.refetch()]).finally(() => {
      setServerRefreshPending(false);
    });
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    acceptanceMutation.mutate();
  }

  return (
    <>
      {visibleSnapshot.kind !== "complete" ? (
        <aside className={`legal-gate legal-gate--${visibleSnapshot.kind}`} aria-live="polite" aria-label="Xác nhận tài liệu pháp lý">
          <div className="legal-gate__header">
            <strong>{gateTitle(visibleSnapshot)}</strong>
            <span>Việc đọc, báo cáo, chặn người dùng, cài đặt, xóa tài khoản và đăng xuất vẫn khả dụng.</span>
          </div>
          {visibleSnapshot.detail ? <p className="legal-gate__error" role="alert">{visibleSnapshot.detail}</p> : null}
          {visibleSnapshot.kind === "required" && documents && !validationError ? (
            <form className="legal-gate__form" onSubmit={submit}>
              <label>
                <input checked={termsAccepted} disabled={acceptanceMutation.isPending} onChange={(event) => setTermsAccepted(event.target.checked)} type="checkbox" />
                <span>Tôi đã đọc và chấp nhận <LegalPolicyLink label="Điều khoản & Quy tắc sử dụng" url={legalPolicyConfig.termsUrl} version={documents.terms.version} />.</span>
              </label>
              <label>
                <input checked={privacyAccepted} disabled={acceptanceMutation.isPending} onChange={(event) => setPrivacyAccepted(event.target.checked)} type="checkbox" />
                <span>Tôi đã đọc và chấp nhận <LegalPolicyLink label="Chính sách quyền riêng tư" url={legalPolicyConfig.privacyUrl} version={documents.privacy.version} />.</span>
              </label>
              {acceptanceMutation.error ? <p className="legal-gate__error" role="alert">{acceptanceMutation.error.message}</p> : null}
              <Button disabled={!privacyAccepted || !termsAccepted || acceptanceMutation.isPending} type="submit">
                {acceptanceMutation.isPending ? "Đang ghi nhận..." : "Chấp nhận và tiếp tục tạo nội dung"}
              </Button>
            </form>
          ) : null}
          {visibleSnapshot.kind === "unavailable" || visibleSnapshot.kind === "mismatch" ? (
            <Button onClick={retry} size="sm" type="button" variant="secondary">Thử kiểm tra lại</Button>
          ) : null}
        </aside>
      ) : null}
      {children}
    </>
  );
}

function LegalPolicyLink({ label, url, version }: { label: string; url: string; version: string }) {
  const isDesktop = getPlatformServices().lifecycle.isDesktop;
  return (
    <a
      href={url}
      onClick={isDesktop ? (event) => {
        event.preventDefault();
        void getPlatformServices().links.openExternal(url);
      } : undefined}
      rel="noreferrer"
      target="_blank"
    >
      {label} (bản {version})
    </a>
  );
}

function gateTitle(snapshot: LegalGateSnapshot): string {
  switch (snapshot.kind) {
    case "required":
      return "Cần chấp nhận tài liệu pháp lý trước khi tạo nội dung";
    case "unavailable":
      return "Chưa thể kiểm tra trạng thái pháp lý";
    case "mismatch":
      return "Cấu hình tài liệu pháp lý chưa đồng bộ";
    default:
      return "Đang kiểm tra trạng thái pháp lý...";
  }
}
