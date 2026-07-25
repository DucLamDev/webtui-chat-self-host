import type { HTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type TooltipProps = HTMLAttributes<HTMLSpanElement> & {
  label: string;
};

export function Tooltip({ children, className, label, ...props }: TooltipProps) {
  return (
    <span className={cn("ui-tooltip", className)} {...props}>
      {children}
      <span className="ui-tooltip__content" role="tooltip">
        {label}
      </span>
    </span>
  );
}
