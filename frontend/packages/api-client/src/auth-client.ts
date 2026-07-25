import type {
  AuthResult,
  AuthSession,
  AuthUser,
  GoogleLoginInput,
  LoginInput,
  LogoutInput,
  OIDCCompleteInput,
  OIDCProviderSummary,
  OIDCStartInput,
  OIDCStartResult,
  RefreshInput,
  RegisterInput
} from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export function createAuthClient(http: HttpClient) {
  return {
    login(input: LoginInput) {
      return http.post<AuthResult>("/api/v1/auth/login", input, { auth: false });
    },
    google(input: GoogleLoginInput) {
      return http.post<AuthResult>("/api/v1/auth/google", input, { auth: false });
    },
    async oidcProviders(domain: string) {
      const query = new URLSearchParams({ domain });
      const data = await http.get<unknown>(`/api/v1/auth/oidc/providers?${query.toString()}`, { auth: false });
      return collectionFrom<OIDCProviderSummary>(data, "oidc_providers");
    },
    oidcStart(input: OIDCStartInput) {
      return http.post<OIDCStartResult>("/api/v1/auth/oidc/start", input, { auth: false });
    },
    oidcComplete(input: OIDCCompleteInput) {
      return http.post<AuthResult>("/api/v1/auth/oidc/complete", input, { auth: false });
    },
    register(input: RegisterInput) {
      return http.post<AuthResult>("/api/v1/auth/register", input, { auth: false });
    },
    refresh(input: RefreshInput) {
      return http.post<AuthResult>("/api/v1/auth/refresh", input, { auth: false });
    },
    logout(input: LogoutInput) {
      return http.post<{ status?: string }>("/api/v1/auth/logout", input);
    },
    async me() {
      const data = await http.get<unknown>("/api/v1/auth/me");
      return itemFrom<AuthUser>(data, "user");
    },
    async sessions() {
      const data = await http.get<unknown>("/api/v1/auth/sessions");
      return collectionFrom<AuthSession>(data, "sessions");
    },
    revokeSession(sessionId: string) {
      return http.delete<void>(`/api/v1/auth/sessions/${encodeURIComponent(sessionId)}`);
    },
    revokeAllSessions() {
      return http.delete<void>("/api/v1/auth/sessions");
    }
  };
}
