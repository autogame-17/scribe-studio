import { useEffect, useMemo, useState } from 'react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Download as DownloadIcon, Film, FolderOpen } from 'lucide-react'
import { ListTasks, OpenInFinder } from '../../wailsjs/go/scribe/App'
import type { sphkit } from '../../wailsjs/go/models'

type Task = sphkit.TaskSummary

export function DownloadsPage() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [total, setTotal] = useState(0)
  const [filter, setFilter] = useState<'all' | 'running' | 'done' | 'error'>('all')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function pull() {
      try {
        const r = await ListTasks(filter, 1, 50)
        if (cancelled) return
        setTasks(r.tasks ?? [])
        setTotal(r.total ?? 0)
      } catch {
        /* proxy not running; list stays empty */
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    pull()
    const id = setInterval(pull, 2000)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [filter])

  const filters: { key: typeof filter; label: string }[] = [
    { key: 'all', label: '全部' },
    { key: 'running', label: '进行中' },
    { key: 'done', label: '已完成' },
    { key: 'error', label: '失败' },
  ]

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
        <div>
          <CardTitle>下载记录</CardTitle>
          <CardDescription>
            共 {total} 条 · 数据来源 wx_channel 本地 BoltDB
          </CardDescription>
        </div>
        <div className="flex items-center gap-1 rounded-md border border-border/60 bg-muted/40 p-0.5">
          {filters.map((f) => (
            <button
              key={f.key}
              onClick={() => setFilter(f.key)}
              className={
                'rounded-[5px] px-2.5 py-0.5 text-[11px] font-medium transition-colors ' +
                (filter === f.key
                  ? 'bg-background text-foreground shadow-sm ring-1 ring-border/70'
                  : 'text-muted-foreground hover:text-foreground')
              }
            >
              {f.label}
            </button>
          ))}
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <EmptyState icon={<DownloadIcon className="h-10 w-10 opacity-40" />} text="读取中…" />
        ) : tasks.length === 0 ? (
          <EmptyState
            icon={<DownloadIcon className="h-10 w-10 opacity-40" />}
            text="暂无下载记录"
            sub="启动代理、在微信视频号里点击注入的下载按钮，任务会实时出现在这里。"
          />
        ) : (
          <div className="divide-y divide-border/40">
            {tasks.map((t) => (
              <TaskRow key={t.id} task={t} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function EmptyState({
  icon,
  text,
  sub,
}: {
  icon: React.ReactNode
  text: string
  sub?: string
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16 text-center text-muted-foreground">
      {icon}
      <div>
        <p className="text-sm">{text}</p>
        {sub && <p className="mx-auto mt-1 max-w-sm text-xs opacity-70">{sub}</p>}
      </div>
    </div>
  )
}

function TaskRow({ task }: { task: Task }) {
  const percent =
    task.size > 0 ? Math.min(100, Math.round((task.downloaded / task.size) * 100)) : 0

  const displayTitle = useMemo(() => {
    const raw = task.title || task.filename || task.id
    // Tags like "#大模型 #AI" clutter the title — strip them for readability,
    // keep the prefix. Users can still see the full filename in the tooltip.
    const idx = raw.indexOf('#')
    return idx > 0 ? raw.slice(0, idx).trim() : raw
  }, [task.title, task.filename, task.id])

  async function reveal() {
    if (!task.path || !task.filename) return
    await OpenInFinder(`${task.path}/${task.filename}`)
  }

  return (
    <div className="flex items-center gap-4 py-3">
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted/60 text-muted-foreground">
        <Film className="h-5 w-5" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <div className="truncate text-sm font-medium" title={task.title}>
            {displayTitle}
          </div>
          {task.spec && (
            <Badge variant="outline" className="h-5 shrink-0 px-1.5 py-0 font-mono text-[10px]">
              {task.spec}
            </Badge>
          )}
        </div>
        <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
          <span>{humanSize(task.size)}</span>
          {task.status === 'running' && (
            <>
              <span>·</span>
              <span>{humanSize(task.speed)}/s</span>
              <span>·</span>
              <span>{percent}%</span>
            </>
          )}
          <span>·</span>
          <span className="font-mono">{shortTime(task.updatedAt)}</span>
        </div>
        {task.status === 'running' && (
          <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-emerald-500 transition-all"
              style={{ width: `${percent}%` }}
            />
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <StatusBadge status={task.status} />
        {task.status === 'done' && (
          <Button variant="ghost" size="sm" className="h-7 px-2" onClick={reveal} title="在 Finder 中显示">
            <FolderOpen className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  if (status === 'done' || status === 'completed')
    return <Badge variant="success">完成</Badge>
  if (status === 'running' || status === 'downloading')
    return <Badge variant="warning">下载中</Badge>
  if (status === 'error' || status === 'failed')
    return <Badge variant="destructive">失败</Badge>
  if (status === 'paused') return <Badge variant="outline">已暂停</Badge>
  return <Badge variant="outline">{status}</Badge>
}

function humanSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(n >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

function shortTime(iso: string): string {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    const hh = String(d.getHours()).padStart(2, '0')
    const mm = String(d.getMinutes()).padStart(2, '0')
    return `${d.getMonth() + 1}/${d.getDate()} ${hh}:${mm}`
  } catch {
    return iso.slice(0, 16)
  }
}
