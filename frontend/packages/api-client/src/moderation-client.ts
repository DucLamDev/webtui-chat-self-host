import type {
  ListModerationReportsParams,
  ModerationReport,
  UpdateModerationReportInput
} from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom } from "./response-utils";

export function createModerationClient(http: HttpClient) {
  return {
    async listReports(workspaceId: string, params: ListModerationReportsParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/moderation/reports`,
        { query: params }
      );
      return collectionFrom<ModerationReport>(data, "reports");
    },
    updateReport(workspaceId: string, reportId: string, input: UpdateModerationReportInput) {
      return http.patch<ModerationReport>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/moderation/reports/${encodeURIComponent(reportId)}`,
        input
      );
    }
  };
}
