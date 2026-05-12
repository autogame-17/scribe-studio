import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Download,
  FileText,
  ScrollText,
  Settings as SettingsIcon,
  Info,
  AudioLines,
} from 'lucide-react'
import { cn } from '@/lib/utils'

type NavEntry = {
  to: string
  label: string
  icon: React.ComponentType<{ className?: string }>
}

const entries: NavEntry[] = [
  { to: '/', label: '概览', icon: LayoutDashboard },
  { to: '/downloads', label: '下载', icon: Download },
  { to: '/transcripts', label: '转写', icon: FileText },
  { to: '/logs', label: '日志', icon: ScrollText },
  { to: '/settings', label: '设置', icon: SettingsIcon },
  { to: '/about', label: '关于', icon: Info },
]

export function Sidebar() {
  return (
    <aside
      className={cn(
        'flex h-full w-[220px] flex-col border-r border-border/60',
        'bg-white/70 supports-[backdrop-filter]:bg-white/55 backdrop-blur-xl',
        'dark:bg-black/30 dark:supports-[backdrop-filter]:bg-black/20'
      )}
    >
      {/* Leave vertical space for the macOS traffic lights at the top-left */}
      <div className="h-8 w-full" style={{ '--wails-draggable': 'drag' } as any} />

      <div
        className="flex items-center gap-3 px-5 py-4"
        style={{ '--wails-draggable': 'drag' } as any}
      >
        <div
          className={cn(
            'flex h-12 w-12 items-center justify-center rounded-[14px]',
            'bg-gradient-to-br from-violet-500 via-fuchsia-500 to-pink-500',
            'shadow-[0_6px_20px_rgba(168,85,247,0.30)] ring-1 ring-black/10 dark:ring-white/10'
          )}
        >
          <AudioLines className="h-6 w-6 text-white" strokeWidth={2.25} />
        </div>
        <div className="flex flex-col leading-tight">
          <span className="text-[15px] font-semibold tracking-tight text-foreground">
            Scribe
          </span>
          <span className="mt-1 text-[10px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
            多平台转写
          </span>
        </div>
      </div>

      <nav className="flex-1 space-y-0.5 px-3 py-2">
        {entries.map((e) => (
          <NavLink
            key={e.to}
            to={e.to}
            end={e.to === '/'}
            className={({ isActive }) =>
              cn(
                'group flex items-center gap-3 rounded-lg px-3 py-2 text-[13px] font-medium transition-colors duration-150',
                isActive
                  ? cn(
                      'bg-foreground/[0.06] text-foreground',
                      'dark:bg-white/10 dark:text-foreground',
                      'shadow-[inset_0_0_0_1px_rgba(0,0,0,0.04)]',
                      'dark:shadow-[inset_0_0_0_1px_rgba(255,255,255,0.06)]'
                    )
                  : 'text-muted-foreground hover:bg-foreground/[0.04] hover:text-foreground dark:hover:bg-white/5'
              )
            }
          >
            {({ isActive }) => (
              <>
                <e.icon
                  className={cn(
                    'h-4 w-4 transition-colors',
                    isActive ? 'text-foreground' : 'opacity-60 group-hover:opacity-90'
                  )}
                />
                {e.label}
              </>
            )}
          </NavLink>
        ))}
      </nav>

      <div className="p-5 text-[10px] font-medium text-muted-foreground/60">
        v0.2.0-dev · scribe core
      </div>
    </aside>
  )
}
