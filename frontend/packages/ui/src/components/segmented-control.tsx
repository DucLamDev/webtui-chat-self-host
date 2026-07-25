import { cn } from "../lib/cn";

export type SegmentedOption<TValue extends string> = {
  label: string;
  value: TValue;
  disabled?: boolean;
};

export type SegmentedControlProps<TValue extends string> = {
  "aria-label": string;
  className?: string;
  options: Array<SegmentedOption<TValue>>;
  value: TValue;
  onValueChange: (value: TValue) => void;
};

export function SegmentedControl<TValue extends string>({
  "aria-label": ariaLabel,
  className,
  options,
  value,
  onValueChange
}: SegmentedControlProps<TValue>) {
  return (
    <div aria-label={ariaLabel} className={cn("ui-segmented", className)} role="tablist">
      {options.map((option) => (
        <button
          aria-selected={value === option.value}
          className={cn(
            "ui-segmented__item",
            value === option.value && "ui-segmented__item--active"
          )}
          disabled={option.disabled}
          key={option.value}
          onClick={() => onValueChange(option.value)}
          role="tab"
          type="button"
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
