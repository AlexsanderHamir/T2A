import type { ComponentPropsWithoutRef } from "react";

type HamixWordmarkProps = ComponentPropsWithoutRef<"span">;

/** Scalable product wordmark — gradient text rendered in CSS, not a raster asset. */
export function HamixWordmark({ className, ...props }: HamixWordmarkProps) {
  return (
    <span className={className} {...props}>
      Hamix
    </span>
  );
}
