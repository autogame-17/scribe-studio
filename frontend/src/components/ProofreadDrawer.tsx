// SPDX-License-Identifier: GPL-3.0-or-later
import { useCallback, useState } from 'react'
import { X, Sparkles, Check, XCircle, BookPlus, RefreshCw, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import {
  ProofreadTranscript,
  AcceptFixFromCache,
  RejectFixFromCache,
  AcceptNewTermFromCache,
  ClearProofreadCache,
} from '../../wailsjs/go/scribe/App'
import type { pipeline, proofread } from '../../wailsjs/go/models'
import { toast } from 'sonner'

type Fix = proofread.Fix
type NewTerm = proofread.NewTerm
type Saved = pipeline.SavedTranscript

const TYPE_COLOR: Record<string, string> = {
  homophone: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200',
  punctuation: 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200',
  term: 'bg-violet-100 text-violet-800 dark:bg-violet-900/40 dark:text-violet-200',
  grammar: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200',
  other: 'bg-muted text-muted-foreground',
}

const TYPE_LABEL: Record<string, string> = {
  homophone: '同音',
  punctuation: '标点',
  term: '术语',
  grammar: '语法',
  other: '其它',
}

/**
 * ProofreadDrawer — right sheet that hosts the LLM校对 run.
 *
 * Flow: user clicks 校对 in Editor → we call ProofreadTranscript and
 * render the returned fixes / newTerms. Accept/Reject per item; on
 * Accept we post back to the backend which mutates the transcript and
 * optionally writes to the glossary, then hand the refreshed Saved
 * to the editor so it can re-render.
 */
export function ProofreadDrawer({
  open,
  onClose,
  taskID,
  onApplied,
}: {
  open: boolean
  onClose: () => void
  taskID: string | undefined
  onApplied: (saved: Saved) => void
}) {
  const [loading, setLoading] = useState(false)
  const [fixes, setFixes] = useState<Fix[]>([])
  const [newTerms, setNewTerms] = useState<NewTerm[]>([])
  const [model, setModel] = useState<string>('')
  const [ran, setRan] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [cacheKey, setCacheKey] = useState('')

  const run = useCallback(async (force = false) => {
    if (!taskID) return
    setLoading(true)
    setError(null)
    try {
      if (force) {
        await ClearProofreadCache()
      }
      const res = await ProofreadTranscript(taskID)
      setFixes(res.fixes ?? [])
      setNewTerms(res.newTerms ?? [])
      setModel(res.model ?? '')
      setCacheKey(res.cacheKey ?? '')
      setRan(true)
      const total = (res.fixes?.length ?? 0) + (res.newTerms?.length ?? 0)
      if (total === 0) {
        toast.success('LLM 没有新建议')
      } else {
        toast.success(`LLM 返回 ${res.fixes?.length ?? 0} 处修正 / ${res.newTerms?.length ?? 0} 个新词`)
      }
    } catch (e) {
      setError(cleanErrorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [taskID])

  async function accept(fix: Fix, learn: boolean) {
    if (!taskID) return
    try {
      const saved = await AcceptFixFromCache(taskID, fix.id, cacheKey, learn)
      setFixes((prev) => prev.filter((f) => f.id !== fix.id))
      onApplied(saved)
      toast.success(learn ? '已替换并加入词表' : '已替换')
    } catch (e) {
      toast.error(String(e))
    }
  }

  async function reject(fix: Fix) {
    if (!taskID) return
    try {
      await RejectFixFromCache(taskID, fix.id, cacheKey)
      setFixes((prev) => prev.filter((f) => f.id !== fix.id))
    } catch (e) {
      toast.error(String(e))
    }
  }

  async function addTerm(t: NewTerm) {
    if (!taskID) return
    try {
      await AcceptNewTermFromCache(taskID, t.id, cacheKey)
      setNewTerms((prev) => prev.filter((x) => x.id !== t.id))
      toast.success(`已加入词表：${t.term}`)
    } catch (e) {
      toast.error(String(e))
    }
  }

  return (
    <>
      <div
        onClick={onClose}
        className={cn(
          'fixed inset-0 z-40 bg-black/30 backdrop-blur-sm transition-opacity',
          open ? 'opacity-100' : 'pointer-events-none opacity-0'
        )}
      />
      <aside
        className={cn(
          'fixed right-0 top-0 z-50 flex h-screen w-[520px] flex-col border-l border-border bg-background shadow-2xl transition-transform',
          open ? 'translate-x-0' : 'translate-x-full'
        )}
      >
        <header className="flex items-center justify-between border-b border-border/60 px-5 py-4">
          <div>
            <h2 className="flex items-center gap-2 text-base font-semibold">
              <Sparkles className="h-4 w-4 text-violet-500" />
              AI 校对
            </h2>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {ran
                ? `${fixes.length} 处修正 · ${newTerms.length} 个新词建议${model ? ' · ' + model : ''}`
                : '点「开始校对」让 LLM 扫一遍当前转写稿'}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => run(true)}
              disabled={loading || !taskID}
              className="gap-1"
              title="清缓存后重新校对"
            >
              <RefreshCw className={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
              重跑
            </Button>
            <button
              onClick={onClose}
              className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="sph-scroll flex-1 overflow-y-auto">
          {!ran ? (
            <div className="flex h-full flex-col items-center justify-center gap-4 p-8 text-center">
              <Sparkles className="h-10 w-10 text-muted-foreground/40" />
              <div className="text-sm text-muted-foreground">
                LLM 会识别同音字误识别、未入库的专名、标点等。
                <br />
                用的 provider 在「设置 → AI 校对」里改。
              </div>
              <Button onClick={() => run(false)} disabled={loading || !taskID} className="gap-1.5">
                <Sparkles className="h-3.5 w-3.5" />
                {loading ? '校对中…' : '开始校对'}
              </Button>
            </div>
          ) : loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">校对中…</div>
          ) : fixes.length === 0 && newTerms.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">
              LLM 没有发现需要处理的问题。
            </div>
          ) : (
            <div className="space-y-4 p-4">
              {fixes.length > 0 && (
                <section>
                  <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    建议修正 ({fixes.length})
                  </h3>
                  <ul className="space-y-2">
                    {fixes.map((f) => (
                      <FixRow key={f.id} fix={f} onAccept={accept} onReject={reject} />
                    ))}
                  </ul>
                </section>
              )}
              {newTerms.length > 0 && (
                <section>
                  <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    疑似新词 ({newTerms.length})
                  </h3>
                  <ul className="space-y-2">
                    {newTerms.map((t) => (
                      <TermRow key={t.id} term={t} onAdd={addTerm} />
                    ))}
                  </ul>
                </section>
              )}
            </div>
          )}
        </div>
      </aside>
      {error && (
        <ProofreadErrorDialog
          message={error}
          loading={loading}
          onClose={() => setError(null)}
          onRetry={() => run(true)}
        />
      )}
    </>
  )
}

function ProofreadErrorDialog({
  message,
  loading,
  onClose,
  onRetry,
}: {
  message: string
  loading: boolean
  onClose: () => void
  onRetry: () => void
}) {
  const hint = proofreadErrorHint(message)
  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm">
      <section
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="proofread-error-title"
        className="w-full max-w-lg rounded-xl border border-border bg-background p-5 shadow-2xl"
      >
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-destructive/10 text-destructive">
            <AlertCircle className="h-5 w-5" />
          </div>
          <div className="min-w-0 flex-1">
            <h3 id="proofread-error-title" className="text-base font-semibold">
              校对失败
            </h3>
            <p className="mt-2 break-words rounded-md border border-border/60 bg-muted/40 p-3 font-mono text-xs leading-5 text-foreground">
              {message}
            </p>
            {hint && (
              <p className="mt-3 text-sm leading-6 text-muted-foreground">{hint}</p>
            )}
            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              详细 provider、耗时和文本规模会写入「日志」页。
            </p>
          </div>
          <button
            onClick={onClose}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            aria-label="关闭弹窗"
            title="关闭弹窗"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={onClose}>
            关闭
          </Button>
          <Button size="sm" onClick={onRetry} disabled={loading} className="gap-1.5">
            <RefreshCw className={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
            {loading ? '重跑中…' : '清缓存重跑'}
          </Button>
        </div>
      </section>
    </div>
  )
}

function cleanErrorMessage(e: unknown) {
  return String(e).replace(/^Error:\s*/, '')
}

function proofreadErrorHint(message: string) {
  if (message.includes('context deadline exceeded')) {
    return '这通常表示上游 LLM 在本次校对超时时间内没有返回。长稿会拆成多个 chunk；可以重跑一次，或换更快的模型/中转/代理后再试。'
  }
  if (message.includes('ai provider not configured')) {
    return '还没有配置可用的 AI Provider。请到「设置 → AI 校对」填写并测试连通。'
  }
  return ''
}

function FixRow({
  fix,
  onAccept,
  onReject,
}: {
  fix: Fix
  onAccept: (f: Fix, learn: boolean) => void
  onReject: (f: Fix) => void
}) {
  const badgeCls = TYPE_COLOR[fix.type] ?? TYPE_COLOR.other
  const learnable = fix.type === 'homophone' || fix.type === 'term'
  return (
    <li className="rounded-lg border border-border/60 bg-card/60 p-3">
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <span
            className={cn(
              'rounded px-1.5 py-0.5 text-[10px] font-medium',
              badgeCls
            )}
          >
            {TYPE_LABEL[fix.type] ?? fix.type}
          </span>
          <Badge variant="outline" className="h-4 px-1.5 font-mono text-[10px]">
            #{fix.segmentIndex + 1}
          </Badge>
        </div>
      </div>
      <div className="mt-2 space-y-1 text-sm leading-6">
        <div className="flex items-baseline gap-2">
          <span className="shrink-0 text-[11px] text-muted-foreground">原</span>
          <span className="break-words font-mono text-[13px] text-rose-600 dark:text-rose-300">
            {fix.original}
          </span>
        </div>
        <div className="flex items-baseline gap-2">
          <span className="shrink-0 text-[11px] text-muted-foreground">改</span>
          <span className="break-words font-mono text-[13px] text-emerald-600 dark:text-emerald-300">
            {fix.suggested}
          </span>
        </div>
      </div>
      {fix.reason && (
        <div className="mt-2 text-[11px] text-muted-foreground">{fix.reason}</div>
      )}
      <div className="mt-3 flex justify-end gap-2">
        <Button size="sm" variant="ghost" onClick={() => onReject(fix)} className="h-7 gap-1">
          <XCircle className="h-3.5 w-3.5" /> 忽略
        </Button>
        {learnable && (
          <Button
            size="sm"
            variant="outline"
            onClick={() => onAccept(fix, true)}
            className="h-7 gap-1"
            title="采纳并加入词表"
          >
            <BookPlus className="h-3.5 w-3.5" /> 采纳+入库
          </Button>
        )}
        <Button size="sm" onClick={() => onAccept(fix, false)} className="h-7 gap-1">
          <Check className="h-3.5 w-3.5" /> 采纳
        </Button>
      </div>
    </li>
  )
}

function TermRow({
  term,
  onAdd,
}: {
  term: NewTerm
  onAdd: (t: NewTerm) => void
}) {
  return (
    <li className="rounded-lg border border-border/60 bg-card/60 p-3">
      <div className="flex items-center gap-2">
        <span className="font-mono text-sm font-semibold">{term.term}</span>
        {term.confidence > 0 && (
          <Badge variant="outline" className="h-4 px-1.5 text-[10px]">
            {Math.round(term.confidence * 100)}%
          </Badge>
        )}
      </div>
      {term.wrongs?.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1">
          {term.wrongs.map((w, i) => (
            <code
              key={i}
              className="rounded bg-muted/70 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground"
            >
              {w}
            </code>
          ))}
        </div>
      )}
      {term.evidence && (
        <div
          className="mt-2 line-clamp-2 text-[11px] text-muted-foreground"
          title={term.evidence}
        >
          {term.evidence}
        </div>
      )}
      <div className="mt-2 flex justify-end">
        <Button size="sm" variant="outline" onClick={() => onAdd(term)} className="h-7 gap-1">
          <BookPlus className="h-3.5 w-3.5" /> 加入词表
        </Button>
      </div>
    </li>
  )
}
