import { MonitorIcon, MoonIcon, SunIcon } from 'lucide-react'
import { useTheme } from 'next-themes'

import { Button } from '@/components/ui/button'

const themes = ['system', 'light', 'dark'] as const

export function ThemeToggle() {
  const { setTheme, theme = 'system' } = useTheme()
  const currentIndex = Math.max(0, themes.indexOf(theme as (typeof themes)[number]))
  const nextTheme = themes[(currentIndex + 1) % themes.length] ?? 'system'
  const Icon = theme === 'light' ? SunIcon : theme === 'dark' ? MoonIcon : MonitorIcon

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className="gap-2 rounded-full bg-background/80 shadow-sm backdrop-blur"
      aria-label={`Theme is ${theme}. Switch to ${nextTheme} theme.`}
      title={`Theme: ${theme}`}
      onClick={() => setTheme(nextTheme)}
    >
      <Icon aria-hidden="true" />
      <span className="capitalize">{theme}</span>
    </Button>
  )
}
