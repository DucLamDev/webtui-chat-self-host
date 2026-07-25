import type { HTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: "blue" | "green" | "red" | "orange" | "slate";
};

export function Badge({
  className,
  tone = "blue",
  ...props
}: BadgeProps) {
  return <span className={cn("ui-badge", `ui-badge--${tone}`, className)} {...props} />;
}
