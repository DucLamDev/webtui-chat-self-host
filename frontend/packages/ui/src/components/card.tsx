import type { HTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type CardProps = HTMLAttributes<HTMLDivElement>;

export function Card({ className, ...props }: CardProps) {
  return <div className={cn("ui-card", className)} {...props} />;
}

export function CardHeader({ className, ...props }: CardProps) {
  return <div className={cn("ui-card__header", className)} {...props} />;
}

export function CardContent({ className, ...props }: CardProps) {
  return <div className={cn("ui-card__content", className)} {...props} />;
}
