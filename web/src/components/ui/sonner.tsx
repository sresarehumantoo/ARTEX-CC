import type { CSSProperties } from "react"
import { Toaster as Sonner, type ToasterProps } from "sonner"

import { usePreferences } from "../../stores/preferences"

function Toaster(props: ToasterProps) {
  const { theme_mode } = usePreferences()
  return (
    <Sonner
      theme={theme_mode}
      className="toaster group"
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
