import type { InputHTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  leftAddon?: React.ReactNode;
  rightAddon?: React.ReactNode;
};

export function Input({
  className,
  leftAddon,
  rightAddon,
  type = "text",
  ...props
}: InputProps) {
  return (
    <label className={cn("ui-input-shell", className)}>
      {leftAddon ? <span className="ui-input-shell__addon">{leftAddon}</span> : null}
      <input type={type} {...props} />
      {rightAddon ? <span className="ui-input-shell__addon">{rightAddon}</span> : null}
    </label>
  );
}
