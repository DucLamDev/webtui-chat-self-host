import type { Permission } from "./rbac";

export type PermissionInput = Permission | string;

export function normalizePermissionCode(permission: PermissionInput): string {
  return typeof permission === "string" ? permission : permission.code;
}

export function createPermissionSet(permissions: PermissionInput[]): Set<string> {
  return new Set(permissions.map(normalizePermissionCode).filter(Boolean));
}

export function hasPermission(permissionCodes: ReadonlySet<string>, permission: string): boolean {
  return permissionCodes.has(permission) || permissionCodes.has("*");
}
