"use client";

import { Children, Fragment, isValidElement, type ReactElement, type ReactNode } from "react";
import { Select } from "@/components/operator/ui";

type Opt = { value: string; label: string };

const text = (n: ReactNode): string => Children.toArray(n).map((c) => (typeof c === "string" || typeof c === "number" ? String(c) : isValidElement(c) ? text((c.props as { children?: ReactNode }).children) : "")).join("");

/** The `<option>`s a native select would have been given, found in its children (also inside groups and fragments). */
function options(children: ReactNode, out: Opt[] = []): Opt[] {
  for (const c of Children.toArray(children)) {
    if (!isValidElement(c)) continue;
    const el = c as ReactElement<{ value?: string | number; children?: ReactNode }>;
    if (el.type === "option") out.push({ value: String(el.props.value ?? text(el.props.children)), label: text(el.props.children) });
    else if (el.type === "optgroup" || el.type === Fragment) options(el.props.children, out);
  }
  return out;
}

/**
 * A drop-in for the browser's `<select>` in the platform's own style: the same `value`, `onChange` (an event with
 * `target.value`) and `<option>` children, so a form is moved over by changing the tag. The list animates, follows the
 * keyboard and looks like every other control of the product, which the browser's own list never could.
 */
export function SelectField({ value, onChange, children, className, disabled, ...rest }: {
  value?: string | number; onChange?: (e: { target: { value: string }; currentTarget: { value: string } }) => void; children: ReactNode; className?: string;
  disabled?: boolean; "aria-label"?: string; id?: string; name?: string; title?: string; required?: boolean;
}) {
  const opts = options(children);
  const v = value === undefined ? "" : String(value);
  return (
    <Select label={rest["aria-label"] ?? rest.title ?? "Choose"} className={className} disabled={disabled} value={v}
      options={opts} placeholder={opts.find((o) => o.value === v)?.label ?? ""} onChange={(next) => onChange?.({ target: { value: next }, currentTarget: { value: next } })} />
  );
}
