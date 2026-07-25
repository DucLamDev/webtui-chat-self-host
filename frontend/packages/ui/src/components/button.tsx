import type { ButtonHTMLAttributes } from "react";
import { cn } from "../lib/cn";

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "icon";
  size?: "sm" | "md" | "lg";
};

export function Button({
  className,
  variant = "primary",
  size = "md",
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn("ui-button", `ui-button--${variant}`, `ui-button--${size}`, className)}
      type={type}
      {...props}
    />
  );
}
