"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import { createPermissionSet, hasPermission, type PermissionCode } from "@webtui/types";
import { api } from "@/lib/api";
import { buildChatRoute, parseChatRoute } from "@/lib/chat-route";
import {
  isLikelyOfflineError,
  readWorkspaceShellCache,
  writeWorkspaceShellCache,
  type WorkspaceShellCache
} from "@/features/chat/model/offline-cache";

export type PermissionValue = PermissionCode | string;

export function useWorkspaceContext() {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const parsedRoute = parseChatRoute(pathname, searchParams);
  const legacyWorkspaceId = searchParams.get("workspace") ?? "";
  const requestedWorkspaceRef = parsedRoute?.workspaceRef || legacyWorkspaceId;
  const [cachedShell, setCachedShell] = useState<WorkspaceShellCache | null>(null);

  const workspacesQuery = useQuery({
    queryFn: () => api.workspaces.listMine(),
    queryKey: queryKeys.workspaces.all
  });

  useEffect(() => {
    let disposed = false;
    void readWorkspaceShellCache(requestedWorkspaceRef)
      .then((cache) => cache ?? readWorkspaceShellCache(""))
      .then((cache) => {
        if (!disposed) {
          setCachedShell(cache);
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, [requestedWorkspaceRef]);

  const workspaces = workspacesQuery.data?.length ? workspacesQuery.data : cachedShell?.workspaces ?? [];
  const requestedWorkspace = workspaces.find(
    (workspace) => workspace.id === requestedWorkspaceRef || workspace.slug.toLowerCase() === requestedWorkspaceRef.toLowerCase()
  );
  const resolvedWorkspaceId = requestedWorkspace?.id || (!parsedRoute ? legacyWorkspaceId : "") || workspaces[0]?.id || "";

  const workspaceQuery = useQuery({
    enabled: Boolean(resolvedWorkspaceId),
    queryFn: () => api.workspaces.get(resolvedWorkspaceId),
    queryKey: queryKeys.workspaces.detail(resolvedWorkspaceId),
    retry: false
  });

  const selectedWorkspace =
    workspaceQuery.data ??
    workspaces.find((workspace) => workspace.id === resolvedWorkspaceId) ??
    (cachedShell?.selectedWorkspace?.id === resolvedWorkspaceId ? cachedShell.selectedWorkspace : null);
  const workspaceId = selectedWorkspace?.id ?? resolvedWorkspaceId;

  const permissionsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.rbac.myPermissions(workspaceId),
    queryKey: queryKeys.rbac.me(workspaceId),
    retry: false
  });

  const membersQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.workspaces.members(workspaceId),
    queryKey: queryKeys.workspaces.members(workspaceId)
  });

  const settingsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.workspaces.settings(workspaceId),
    queryKey: queryKeys.workspaces.settings(workspaceId)
  });

  const permissions = permissionsQuery.data ?? cachedShell?.permissions ?? [];
  const members = membersQuery.data ?? cachedShell?.members ?? [];
  const settings = settingsQuery.data ?? cachedShell?.settings ?? [];
  const offlineReadMode = Boolean(cachedShell && (
    (workspacesQuery.isError && isLikelyOfflineError(workspacesQuery.error)) ||
    (workspaceQuery.isError && isLikelyOfflineError(workspaceQuery.error)) ||
    (permissionsQuery.isError && isLikelyOfflineError(permissionsQuery.error)) ||
    (membersQuery.isError && isLikelyOfflineError(membersQuery.error))
  ));

  useEffect(() => {
    if (!workspacesQuery.data?.length) {
      return;
    }
    const snapshot: WorkspaceShellCache = {
      members: membersQuery.data ?? cachedShell?.members ?? [],
      permissions: permissionsQuery.data ?? cachedShell?.permissions ?? [],
      selectedWorkspace,
      settings: settingsQuery.data ?? cachedShell?.settings ?? [],
      workspaces: workspacesQuery.data
    };
    void Promise.all([
      writeWorkspaceShellCache(requestedWorkspaceRef, snapshot),
      writeWorkspaceShellCache(selectedWorkspace?.slug || selectedWorkspace?.id || "", snapshot),
      writeWorkspaceShellCache("", snapshot)
    ]).catch(() => undefined);
  }, [
    cachedShell?.members,
    cachedShell?.permissions,
    cachedShell?.settings,
    membersQuery.data,
    permissionsQuery.data,
    requestedWorkspaceRef,
    selectedWorkspace,
    settingsQuery.data,
    workspacesQuery.data
  ]);

  const permissionCodes = useMemo(
    () => createPermissionSet(permissions),
    [permissions]
  );

  const can = useCallback(
    (permission: PermissionValue) => hasPermission(permissionCodes, permission),
    [permissionCodes]
  );

  const setWorkspaceId = useCallback(
    (nextWorkspaceId: string) => {
      const workspace = workspaces.find((item) => item.id === nextWorkspaceId);
      router.replace(nextWorkspaceId ? buildChatRoute(workspace?.slug || nextWorkspaceId) : "/", { scroll: false });
    },
    [router, workspaces]
  );

  useEffect(() => {
    if (!parsedRoute && !searchParams.get("channel") && workspaceId) {
      setWorkspaceId(workspaceId);
    }
  }, [parsedRoute, searchParams, setWorkspaceId, workspaceId]);

  return {
    can,
    members,
    membersQuery,
    offlineReadMode,
    permissionCodes,
    permissions,
    permissionsQuery,
    selectedWorkspace,
    setWorkspaceId,
    settings,
    settingsQuery,
    workspaceQuery,
    workspaceId,
    workspaces,
    workspacesQuery
  };
}
