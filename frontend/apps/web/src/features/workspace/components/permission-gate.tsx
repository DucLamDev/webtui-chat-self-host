"use client";

import type { ReactNode } from "react";
import { EmptyState } from "@webtui/ui";
import type { PermissionValue } from "../hooks/use-workspace-context";

export type PermissionGateProps = {
  allowed: boolean;
  children: ReactNode;
  description?: string;
  fallback?: ReactNode;
  permission: PermissionValue;
  title?: string;
};

export function PermissionGate({
  allowed,
  children,
  description,
  fallback,
  permission,
  title
}: PermissionGateProps) {
  if (allowed) {
    return <>{children}</>;
  }

  return (
    <>
      {fallback ?? (
        <EmptyState
          description={description ?? `Tài khoản hiện tại chưa có quyền ${permission}.`}
          title={title ?? "Chưa đủ quyền"}
        />
      )}
    </>
  );
}
