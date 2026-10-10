import * as React from "react"
import { cn } from "@/lib/utils"

/**
 * A filled, borderless text field.
 *
 * A hard 1px box around an input is the default in most component libraries,
 * but it makes the field the loudest thing on the page and reads as a form from
 * the previous decade. Apple's treatment is a filled surface that recedes until
 * focused, which keeps the label above it as the thing you actually read.
 *
 * The field stays 44px tall, which is the minimum comfortable touch target.
 */
function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-11 w-full min-w-0 rounded-lg border-0 bg-muted px-3.5 text-base text-foreground",
        "placeholder:text-muted-foreground/70",
        "transition-shadow outline-none",
        // Focus fills the field back to the surface colour and adds a soft ring.
        "focus-visible:bg-card focus-visible:ring-2 focus-visible:ring-primary/40",
        "aria-invalid:ring-2 aria-invalid:ring-destructive/40",
        "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        "file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground",
        "selection:bg-primary selection:text-primary-foreground",
        className
      )}
      {...props}
    />
  )
}

export { Input }
