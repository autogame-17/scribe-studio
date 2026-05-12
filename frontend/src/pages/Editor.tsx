import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  ArrowLeft,
  Save,
  FileDown,
  FileText,
  RefreshCw,
  BookOpen,
  Film,
  Sparkles,
} from 'lucide-react'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  GetTranscript,
  SaveTranscript,
  ExportSRT,
  ExportMD,
  OpenInFinder,
  ListTranscripts,
} from '../../wailsjs/go/scribe/App'
import type { pipeline, transcribe } from '../../wailsjs/go/models'
import { toast } from 'sonner'
import { GlossaryDrawer } from '@/components/GlossaryDrawer'
import { ProofreadDrawer } from '@/components/ProofreadDrawer'

type Segment = transcribe.Segment
type Saved = pipeline.SavedTranscript
type Job = pipeline.Job

export function EditorPage() {
  const { taskID } = useParams<{ taskID: string }>()
  const navigate = useNavigate()

  const [saved, setSaved] = useState<Saved | null>(null)
  const [segments, setSegments] = useState<Segment[]>([])
  const [job, setJob] = useState<Job | null>(null)
  const [loading, setLoading] = useState(true)
  const [dirty, setDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [proofreadOpen, setProofreadOpen] = useState(false)

  const load = useCallback(async () => {
    if (!taskID) return
    try {
      const [t, jobs] = await Promise.all([
        GetTranscript(taskID),
        ListTranscripts(),
      ])
      setSaved(t)
      setSegments(t.segments ?? [])
      setJob((jobs ?? []).find((j) => j.taskID === taskID) ?? null)
      setDirty(false)
    } catch (e) {
      toast.error('加载失败：' + String(e))
    } finally {
      setLoading(false)
    }
  }, [taskID])

  useEffect(() => { load() }, [load])

  // Cmd+S / Ctrl+S anywhere in the editor page (including while a
  // textarea is focused) triggers save. preventDefault stops the
  // browser's Save-As dialog from flashing in.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && (e.key === 's' || e.key === 'S')) {
        e.preventDefault()
        if (!saving && dirty) save()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
    // `save` is fresh each render; including it would re-register on
    // every keystroke. Re-register only when dirty/saving flip.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dirty, saving, taskID, segments])

  // Window close / Wails quit with unsaved edits — at least nudge.
  useEffect(() => {
    if (!dirty) return
    function beforeUnload(e: BeforeUnloadEvent) {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', beforeUnload)
    return () => window.removeEventListener('beforeunload', beforeUnload)
  }, [dirty])

  // Guarded internal nav — sidebar NavLinks + the back button go
  // through this. Route blocking in RR v7 is an unstable hook we
  // don't need yet; confirm() at navigation call sites covers it.
  function guardedNavigate(path: string) {
    if (dirty && !window.confirm('有未保存的改动，确定离开？')) return
    navigate(path)
  }

  function updateSegment(i: number, text: string) {
    setSegments((prev) => {
      const next = prev.slice()
      next[i] = { ...next[i], text }
      return next
    })
    setDirty(true)
  }

  async function save() {
    if (!taskID) return
    setSaving(true)
    try {
      await SaveTranscript(taskID, segments)
      // Re-read so the recomputed canonical hits land back in state;
      // the backend refreshes them from the saved text.
      const t = await GetTranscript(taskID)
      setSaved(t)
      setSegments(t.segments ?? segments)
      setDirty(false)
      toast.success('已保存')
    } catch (e) {
      toast.error('保存失败：' + String(e))
    } finally {
      setSaving(false)
    }
  }

  async function exportSRT() {
    if (!taskID) return
    try {
      if (dirty) await SaveTranscript(taskID, segments)
      const path = await ExportSRT(taskID)
      toast.success('SRT 已导出', { description: path })
      OpenInFinder(path)
    } catch (e) {
      toast.error('导出失败：' + String(e))
    }
  }

  async function exportMD() {
    if (!taskID) return
    try {
      if (dirty) await SaveTranscript(taskID, segments)
      const path = await ExportMD(taskID)
      toast.success('Markdown 已导出', { description: path })
      OpenInFinder(path)
    } catch (e) {
      toast.error('导出失败：' + String(e))
    }
  }

  const hitsBySegment = useMemo(() => {
    const m: Record<number, pipeline.SavedTranscript['hits']> = {}
    for (const h of saved?.hits ?? []) {
      ;(m[h.segmentIndex] ??= []).push(h)
    }
    return m
  }, [saved])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <button
          onClick={() => guardedNavigate('/transcripts')}
          className="flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" /> 返回转写列表
        </button>
        <div className="flex gap-2">
          <Button
            size="sm"
            onClick={() => setProofreadOpen(true)}
            className="gap-1.5 bg-violet-600 text-white hover:bg-violet-500"
          >
            <Sparkles className="h-3.5 w-3.5" /> AI 校对
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setDrawerOpen(true)}
            className="gap-1.5"
          >
            <BookOpen className="h-3.5 w-3.5" /> 词表
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={load}
            className="gap-1.5"
            disabled={loading}
          >
            <RefreshCw className="h-3.5 w-3.5" /> 重新加载
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={exportSRT}
            className="gap-1.5"
          >
            <FileDown className="h-3.5 w-3.5" /> SRT
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={exportMD}
            className="gap-1.5"
          >
            <FileText className="h-3.5 w-3.5" /> Markdown
          </Button>
          <Button size="sm" onClick={save} disabled={!dirty || saving} className="gap-1.5">
            <Save className="h-3.5 w-3.5" /> {saving ? '保存中…' : dirty ? '保存' : '已保存'}
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader className="space-y-1">
          <CardTitle className="flex items-center gap-2 text-base">
            <Film className="h-4 w-4 text-muted-foreground" />
            <span className="truncate" title={job?.title ?? job?.videoPath ?? ''}>
              {cleanTitle(job?.title, job?.videoPath)}
            </span>
            {saved?.model && (
              <Badge variant="outline" className="h-5 px-1.5 font-mono text-[10px]">
                {saved.model.replace('whisper-cpp:', '')}
              </Badge>
            )}
            {(saved?.hits?.length ?? 0) > 0 && (
              <Badge variant="success" className="h-5 px-1.5">
                {saved!.hits!.length} 处词表替换
              </Badge>
            )}
          </CardTitle>
          <CardDescription>
            编辑每段文本，保存后 SRT / Markdown 导出会用更新后的内容。时间戳保持不变。
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="py-12 text-center text-sm text-muted-foreground">读取中…</div>
          ) : segments.length === 0 ? (
            <div className="py-12 text-center text-sm text-muted-foreground">
              没有 segment — 可能转写还没完成
            </div>
          ) : (
            <div className="space-y-3">
              {segments.map((seg, i) => (
                <SegmentRow
                  key={i}
                  index={i}
                  seg={seg}
                  hits={hitsBySegment[i] ?? []}
                  onChange={(t) => updateSegment(i, t)}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <GlossaryDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)} />
      <ProofreadDrawer
        open={proofreadOpen}
        onClose={() => setProofreadOpen(false)}
        taskID={taskID}
        onApplied={(s) => {
          setSaved(s)
          setSegments(s.segments ?? [])
          setDirty(false)
        }}
      />
    </div>
  )
}

function SegmentRow({
  index,
  seg,
  hits,
  onChange,
}: {
  index: number
  seg: Segment
  hits: NonNullable<pipeline.SavedTranscript['hits']>
  onChange: (text: string) => void
}) {
  // Lay out the text with <mark>-style pills over glossary hits.
  // We render a read-only "preview" strip above the textarea so the
  // user sees exactly which words came from the glossary without us
  // having to build an in-textarea overlay renderer.
  const hasHits = hits.length > 0

  return (
    <div className="rounded-lg border border-border/50 bg-card/40 p-3 transition-colors hover:bg-card/60">
      <div className="mb-1.5 flex items-center gap-2 text-[11px] text-muted-foreground">
        <span className="rounded bg-muted/60 px-1.5 py-0.5 font-mono">
          {fmtTime(seg.start)} – {fmtTime(seg.end)}
        </span>
        <span className="opacity-60">#{index + 1}</span>
        {hasHits && (
          <span className="text-emerald-600 dark:text-emerald-400">
            {hits.length} 处自动替换
          </span>
        )}
      </div>
      {hasHits && (
        <div className="mb-2 rounded-md bg-emerald-50/70 p-2 text-[13px] leading-6 dark:bg-emerald-950/30">
          {renderHighlighted(seg.text, hits)}
        </div>
      )}
      <textarea
        value={seg.text}
        onChange={(e) => onChange(e.target.value)}
        rows={Math.min(6, Math.max(2, Math.ceil(seg.text.length / 60)))}
        className="w-full resize-y rounded-md border border-border/60 bg-background px-3 py-2 text-sm leading-6 outline-none focus:border-ring focus:ring-2 focus:ring-ring/30"
      />
    </div>
  )
}

function renderHighlighted(text: string, hits: NonNullable<pipeline.SavedTranscript['hits']>) {
  const sorted = [...hits].sort((a, b) => a.start - b.start)
  const nodes: React.ReactNode[] = []
  let cursor = 0
  for (let i = 0; i < sorted.length; i++) {
    const h = sorted[i]
    if (h.start > cursor) nodes.push(text.slice(cursor, h.start))
    nodes.push(
      <mark
        key={i}
        className="rounded bg-emerald-200/60 px-0.5 text-emerald-900 dark:bg-emerald-700/40 dark:text-emerald-100"
        title={`${h.original} → ${h.replacement}`}
      >
        {text.slice(h.start, h.end)}
      </mark>
    )
    cursor = h.end
  }
  if (cursor < text.length) nodes.push(text.slice(cursor))
  return <>{nodes}</>
}

function cleanTitle(title?: string, path?: string): string {
  const raw = title || (path ? path.split('/').pop() || '' : '') || '(未命名)'
  const idx = raw.indexOf('#')
  return idx > 0 ? raw.slice(0, idx).trim() : raw
}

function fmtTime(sec: number): string {
  if (!sec || sec < 0) sec = 0
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return h > 0
    ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    : `${m}:${String(s).padStart(2, '0')}`
}
