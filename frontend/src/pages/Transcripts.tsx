import { useEffect, useMemo, useState } from 'react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Film, FolderOpen, RefreshCw, FileText, PenLine } from 'lucide-react'
import {
  ListTranscripts,
  RetryTranscribe,
  OpenInFinder,
} from '../../wailsjs/go/scribe/App'
import type { pipeline } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { TranscribeProgress } from '@/components/TranscribeProgress'
import { toast } from 'sonner'
import { useNavigate } from 'react-router-dom'

type Job = pipeline.Job

export function TranscriptsPage() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function pull() {
      try {
        const list = await ListTranscripts()
        if (!cancelled) setJobs(list ?? [])
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    pull()
    const id = setInterval(pull, 3000)

    // Live updates — each stage transition + progress tick emits this.
    const off = EventsOn('transcribe:job', (j: Job) => {
      setJobs((prev) => {
        const idx = prev.findIndex((x) => x.taskID === j.taskID)
        const next = idx >= 0 ? [...prev] : [j, ...prev]
        if (idx >= 0) next[idx] = j
        // Sort by updatedAt desc
        next.sort((a, b) => (a.updatedAt > b.updatedAt ? -1 : 1))
        return next
      })
    })

    return () => {
      cancelled = true
      clearInterval(id)
      off()
    }
  }, [])

  const stats = useMemo(() => {
    const running = jobs.filter((j) => ['pending', 'extracting', 'transcribing', 'saving'].includes(j.stage)).length
    const done = jobs.filter((j) => j.stage === 'done').length
    const failed = jobs.filter((j) => j.stage === 'failed').length
    return { running, done, failed }
  }, [jobs])

  return (
    <Card>
      <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
        <div>
          <CardTitle>转写记录</CardTitle>
          <CardDescription>
            共 {jobs.length} 条 · 进行中 {stats.running} · 完成 {stats.done}
            {stats.failed > 0 ? ` · 失败 ${stats.failed}` : ''}
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Empty icon={<FileText className="h-10 w-10 opacity-40" />} text="读取中…" />
        ) : jobs.length === 0 ? (
          <Empty
            icon={<FileText className="h-10 w-10 opacity-40" />}
            text="暂无转写记录"
            sub="下载完成后会自动转写——先去下载页拉一个视频号视频吧。"
          />
        ) : (
          <div className="divide-y divide-border/40">
            {jobs.map((j) => (
              <JobRow key={j.taskID} job={j} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function JobRow({ job }: { job: Job }) {
  const navigate = useNavigate()
  const title = useMemo(() => {
    const raw = job.title || job.videoPath.split('/').pop() || job.taskID
    const idx = raw.indexOf('#')
    return idx > 0 ? raw.slice(0, idx).trim() : raw
  }, [job])

  async function retry() {
    try {
      await RetryTranscribe(job.taskID)
      toast.success('已重新加入转写队列')
    } catch (e) {
      toast.error(String(e))
    }
  }

  async function reveal() {
    if (job.srtPath) return OpenInFinder(job.srtPath)
    if (job.videoPath) return OpenInFinder(job.videoPath)
  }

  function openEditor() {
    navigate(`/editor/${encodeURIComponent(job.taskID)}`)
  }

  return (
    <div className="flex items-center gap-4 py-3">
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted/60 text-muted-foreground">
        <Film className="h-5 w-5" />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <div className="truncate text-sm font-medium" title={job.title || job.videoPath}>
            {title}
          </div>
          {job.model && (
            <Badge variant="outline" className="h-5 shrink-0 px-1.5 py-0 font-mono text-[10px]">
              {job.model.replace('whisper-cpp:', '')}
            </Badge>
          )}
        </div>
        <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
          {(job.duration ?? 0) > 0 && <span>{formatDur(job.duration!)}</span>}
          {job.srtPath && (
            <>
              <span>·</span>
              <span className="truncate font-mono" title={job.srtPath}>
                {shortPath(job.srtPath)}
              </span>
            </>
          )}
          <span>·</span>
          <span className="font-mono">{shortTime(job.updatedAt)}</span>
        </div>
        {job.error && (
          <div className="mt-2 truncate text-[11px] text-destructive" title={job.error}>
            {job.error}
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <TranscribeProgress job={job} />
        {job.stage === 'done' && (
          <Button
            variant="outline"
            size="sm"
            className="h-7 gap-1 px-2"
            onClick={openEditor}
            title="打开编辑器"
          >
            <PenLine className="h-3.5 w-3.5" /> 编辑
          </Button>
        )}
        {job.stage === 'done' && job.srtPath && (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2"
            onClick={reveal}
            title="在 Finder 中显示"
          >
            <FolderOpen className="h-3.5 w-3.5" />
          </Button>
        )}
        {(job.stage === 'failed' || job.stage === 'done') && (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2"
            onClick={retry}
            title="重新转写"
          >
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>
    </div>
  )
}

function Empty({
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

function shortPath(p: string): string {
  const home = '/Users/'
  const idx = p.indexOf(home)
  if (idx === 0) {
    const rest = p.slice(home.length)
    const slash = rest.indexOf('/')
    return slash > 0 ? '~/' + rest.slice(slash + 1) : p
  }
  return p
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

function formatDur(sec: number): string {
  if (sec < 60) return `${Math.round(sec)}s`
  const m = Math.floor(sec / 60)
  const s = Math.round(sec % 60)
  return `${m}m${String(s).padStart(2, '0')}s`
}
