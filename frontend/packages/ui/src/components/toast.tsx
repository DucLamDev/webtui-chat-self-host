import type { HTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type ToastProps = HTMLAttributes<HTMLDivElement> & {
  tone?: "info" | "success" | "danger";
};

export function Toast({ className, tone = "info", ...props }: ToastProps) {
  return <div className={cn("ui-toast", `ui-toast--${tone}`, className)} role="status" {...props} />;
}
