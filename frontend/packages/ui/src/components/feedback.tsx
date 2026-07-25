import type { HTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type FeedbackStateProps = HTMLAttributes<HTMLDivElement> & {
  title: string;
  description?: string;
  action?: React.ReactNode;
};

export function EmptyState({
  action,
  className,
  description,
  title,
  ...props
}: FeedbackStateProps) {
  return (
    <div className={cn("ui-feedback ui-feedback--empty", className)} {...props}>
      <strong>{title}</strong>
      {description ? <p>{description}</p> : null}
      {action}
    </div>
  );
}

export function ErrorState({
  action,
  className,
  description,
  title,
  ...props
}: FeedbackStateProps) {
  return (
    <div className={cn("ui-feedback ui-feedback--error", className)} {...props}>
      <strong>{title}</strong>
      {description ? <p>{description}</p> : null}
      {action}
    </div>
  );
}
