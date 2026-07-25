import type { HTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type MetricCardProps = HTMLAttributes<HTMLDivElement> & {
  label: string;
  value: string;
  delta?: string;
  tone?: "blue" | "green" | "orange" | "purple";
};

export function MetricCard({
  className,
  label,
  value,
  delta,
  tone = "blue",
  ...props
}: MetricCardProps) {
  return (
    <div className={cn("ui-metric", `ui-metric--${tone}`, className)} {...props}>
      <span>{label}</span>
      <strong>{value}</strong>
      {delta ? <small>{delta}</small> : null}
    </div>
  );
}
