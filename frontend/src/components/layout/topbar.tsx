import { useLocation } from 'react-router-dom'
import { Moon, Sun } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useEffect, useState } from 'react'
import {
  applyTheme,
  getStoredTheme,
  setStoredTheme,
  trackSystemTheme,
  type Theme,
} from '@/lib/theme'

const titleMap: Record<string, string> = {
  '/': '概览',
  '/downloads': '下载',
  '/transcripts': '转写',
  '/logs': '日志',
  '/settings': '设置',
  '/about': '关于',
}

type ProxyState = 'running' | 'stopped' | 'error' | 'starting'

export function Topbar({ status }: { status: ProxyState }) {
  const { pathname } = useLocation()
  const title = titleMap[pathname] ?? '概览'

  const pill = {
    running: <Badge variant="success">运行中</Badge>,
    starting: <Badge variant="warning">启动中</Badge>,
    error: <Badge variant="destructive">异常</Badge>,
    stopped: <Badge variant="outline">已停止</Badge>,
  }[status]

  const [theme, setTheme] = useState<Theme>(getStoredTheme())
  useEffect(() => {
    applyTheme(theme)
    trackSystemTheme(theme === 'system')
    setStoredTheme(theme)
  }, [theme])

  return (
    <header
      className="flex h-[52px] items-center justify-between border-b border-border/50 bg-background/80 px-6 backdrop-blur-md"
      style={{ '--wails-draggable': 'drag' } as any}
    >
      <h1 className="text-lg font-semibold tracking-tight">{title}</h1>
      <div className="flex items-center gap-3" style={{ '--wails-draggable': 'no-drag' } as any}>
        <button
          onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          title={theme === 'dark' ? '切换到浅色' : '切换到深色'}
        >
          {theme === 'dark' ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
        </button>
        {pill}
      </div>
    </header>
  )
}
