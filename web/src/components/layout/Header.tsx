import { Menu, Monitor, Moon, Search, Sun } from 'lucide-react'
import { Button } from '../ui/button'
import { Separator } from '../ui/separator'
import { Kbd } from '../ui/kbd'
import { usePreferences, type ThemeMode } from '../../stores/preferences'

const nextTheme: Record<ThemeMode, ThemeMode> = { light: 'dark', dark: 'system', system: 'light' }
const themeIcon = { light: Sun, dark: Moon, system: Monitor }

interface HeaderProps {
  onToggleSidebar: () => void
  onOpenPalette: () => void
}

export function Header({ onToggleSidebar, onOpenPalette }: HeaderProps) {
  const { theme_mode, setThemeMode } = usePreferences()
  const ThemeIcon = themeIcon[theme_mode]

  return (
    <header className="sticky top-0 z-30 flex h-12 shrink-0 items-center justify-between gap-2 border-b bg-background/50 px-4 backdrop-blur-md">
      <div className="flex items-center gap-1">
        <Button variant="ghost" size="icon" onClick={onToggleSidebar} aria-label="Toggle sidebar">
          <Menu className="size-4" />
        </Button>
        <Separator orientation="vertical" className="mx-2 data-vertical:h-4" />
        <Button
          variant="ghost"
          size="sm"
          onClick={onOpenPalette}
          aria-label="Open command palette"
          className="gap-2 text-muted-foreground"
        >
          <Search className="size-4" />
          <span className="hidden sm:inline">Search</span>
          <Kbd className="hidden sm:inline-flex">Ctrl K</Kbd>
        </Button>
      </div>
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          onClick={() => setThemeMode(nextTheme[theme_mode])}
          aria-label={`Theme: ${theme_mode}. Switch to ${nextTheme[theme_mode]}`}
        >
          <ThemeIcon className="size-4" />
        </Button>
        <span className="text-xs text-muted-foreground">ARTEX-CC</span>
      </div>
    </header>
  )
}
