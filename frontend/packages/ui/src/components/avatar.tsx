"use client";

import { type HTMLAttributes, useState } from "react";
import { cn } from "../lib/cn";

export type AvatarProps = HTMLAttributes<HTMLDivElement> & {
  name: string;
  src?: string;
  status?: "online" | "offline" | "busy";
  size?: "sm" | "md" | "lg";
};

export function Avatar({
  className,
  name,
  src,
  status,
  size = "md",
  ...props
}: AvatarProps) {
  const [failedSrc, setFailedSrc] = useState<string>();
  const initials = name
    .split(" ")
    .filter(Boolean)
    .slice(-2)
    .map((part) => part[0])
    .join("")
    .toUpperCase();

  return (
    <div className={cn("ui-avatar", `ui-avatar--${size}`, className)} {...props}>
      {src && failedSrc !== src ? <img alt={name} onError={() => setFailedSrc(src)} src={src} /> : <span>{initials}</span>}
      {status ? <i className={`ui-avatar__status ui-avatar__status--${status}`} /> : null}
    </div>
  );
}
