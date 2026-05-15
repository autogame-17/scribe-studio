// SPDX-License-Identifier: GPL-3.0-or-later
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { X, Loader2, Link as LinkIcon, ChevronDown, ChevronRight, AlertTriangle, FolderOpen, ExternalLink } from 'lucide-react'
import { ResolveURL, AddExternalURL, OpenChannelsURL } from '../../wailsjs/go/scribe/App'
import type { external } from '../../wailsjs/go/models'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

type ProbeResult = external.ProbeResult
type Format = external.Format

// channelsURLPattern matches WeChat Channels web URLs the embedded MITM
// + injection pipeline can take over. Mirrors the backend's
// isChannelsURL so the UI can flip into "channels mode" without a
// round-trip on every keystroke. The set of recognised hosts must stay
// in sync with backend/scribe/channels_open.go.
const channelsURLPattern = /^https?:\/\/(channels\.weixin\.qq\.com|finder\.video\.qq\.com)\b/i

/**
 * AddURLDialog — modal for adding a YouTube / B站 / yt-dlp-supported
 * URL to the download queue. UX flow:
 *
 *  1. User pastes URL → debounced auto-probe (`yt-dlp -J`)
 *  2. Preview appears (title, uploader, duration, format dropdown)
 *  3. User picks a format (defaults to highest available) and clicks
 *     "开始下载". Advanced options (cookie file, subtitles) live in a
 *     collapsed panel so casual users don't see them.
 *  4. Download starts in the background; modal closes immediately so
 *     the user can keep working.  Progress is rendered in Downloads.
 */
export function AddURLDialog({
  open,
  onClose,
  onSubmitted,
}: {
  open: boolean
  onClose: () => void
  onSubmitted?: (taskID: string) => void
}) {
  const [url, setUrl] = useState('')
  const [probing, setProbing] = useState(false)
  const [probe, setProbe] = useState<ProbeResult | null>(null)
  const [probeError, setProbeError] = useState<string | null>(null)
  const [formatID, setFormatID] = useState<string>('')
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [cookieFile, setCookieFile] = useState('')
  const [subLangsRaw, setSubLangsRaw] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // Whether the pasted URL belongs to WeChat Channels — drives a
  // separate UX branch that hands off to the embedded MITM pipeline
  // instead of yt-dlp. Computed live as the user types so the dialog
  // can swap its hint, suppress the probe call, and re-label the
  // primary action without waiting on the backend.
  const isChannels = useMemo(() => channelsURLPattern.test(url.trim()), [url])

  // Reset state every time the dialog opens — leaving stale probe
  // results around between adds is a great way to download the wrong
  // video.
  useEffect(() => {
    if (open) {
      setUrl('')
      setProbe(null)
      setProbeError(null)
      setFormatID('')
      setShowAdvanced(false)
      setCookieFile('')
      setSubLangsRaw('')
      setSubmitting(false)
    }
  }, [open])

  // ESC to close. Wired only when open to avoid the listener leaking.
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [open, onClose])

  // Debounced auto-probe: 600ms after the URL stops changing.
  const probeTimer = useRef<number | null>(null)
  const probeURL = useCallback(async (target: string) => {
    if (!target) return
    setProbing(true)
    setProbeError(null)
    try {
      const r = await ResolveURL(target, cookieFile)
      setProbe(r)
      // Default to the highest-resolution format available.
      if (r.formats && r.formats.length > 0) {
        setFormatID(r.formats[0].id)
      } else {
        setFormatID('')
      }
    } catch (err) {
      setProbe(null)
      setProbeError(String(err).replace(/^Error: /, ''))
    } finally {
      setProbing(false)
    }
  }, [cookieFile])

  useEffect(() => {
    if (!open) return
    if (probeTimer.current) {
      window.clearTimeout(probeTimer.current)
    }
    if (!url.trim()) {
      setProbe(null)
      setProbeError(null)
      return
    }
    // Channels URLs are handled by the MITM injection pipeline, not
    // yt-dlp — skip the probe entirely so we don't surface a confusing
    // "ERROR: Unsupported URL" toast.
    if (isChannels) {
      setProbe(null)
      setProbeError(null)
      setProbing(false)
      return
    }
    probeTimer.current = window.setTimeout(() => {
      probeURL(url.trim())
    }, 600)
    return () => {
      if (probeTimer.current) {
        window.clearTimeout(probeTimer.current)
      }
    }
  }, [url, open, probeURL, isChannels])

  async function submit() {
    if (!url.trim()) {
      toast.error('请输入 URL')
      return
    }
    setSubmitting(true)
    try {
      // Channels branch: hand the URL to the backend, which boots the
      // embedded proxy (idempotent) and opens the page in the user's
      // default browser. The injected download button shows up there
      // automatically once the page loads, and any download flows
      // back into the existing wx_channel task list.
      if (isChannels) {
        await OpenChannelsURL(url.trim())
        toast.success('已在浏览器中打开，点击页面上的"下载"按钮即可')
        onSubmitted?.('')
        onClose()
        return
      }
      const subLangs = subLangsRaw
        .split(/[,\s]+/)
        .map((s) => s.trim())
        .filter(Boolean)
      const req = {
        url: url.trim(),
        format: formatID,
        formatHint: probe?.formats?.find((f) => f.id === formatID)?.label ?? '',
        cookieFile: cookieFile || '',
        subLangs: subLangs.length > 0 ? subLangs : undefined,
        title: probe?.title ?? '',
        site: probe?.site ?? '',
        duration: probe?.duration ?? 0,
      } as external.AddRequest
      const t = await AddExternalURL(req)
      toast.success('已加入下载队列')
      onSubmitted?.(t.id)
      onClose()
    } catch (err) {
      toast.error(String(err).replace(/^Error: /, ''))
    } finally {
      setSubmitting(false)
    }
  }

  if (!open) return null

  // For Channels URLs we skip probing, so the only gate is "have URL"
  // and "not currently submitting". For yt-dlp URLs we additionally
  // wait for the probe to settle so the user can pick a format.
  const canSubmit = !!url.trim() && !submitting && (isChannels || !probing)

  return (
    <div
      className={cn(
        'fixed inset-0 z-50 flex items-center justify-center',
        'bg-black/40 backdrop-blur-sm'
      )}
      onMouseDown={(e) => {
        // Close on backdrop click, but not on content click.
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className={cn(
          'w-[520px] max-w-[92vw] rounded-xl border border-border/60 bg-background shadow-2xl',
          'flex max-h-[88vh] flex-col overflow-hidden'
        )}
      >
        <div className="flex items-start justify-between gap-2 border-b border-border/40 px-5 py-3">
          <div className="flex items-center gap-2">
            <LinkIcon className="h-4 w-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold">添加链接</h2>
          </div>
          <button
            onClick={onClose}
            className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          <label className="mb-1 block text-xs font-medium text-muted-foreground">
            视频 URL
          </label>
          <input
            type="text"
            autoFocus
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://www.youtube.com/... · https://www.bilibili.com/... · https://channels.weixin.qq.com/..."
            className={cn(
              'w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm',
              'focus:border-foreground/30 focus:outline-none focus:ring-2 focus:ring-foreground/10'
            )}
          />
          <p className="mt-1 text-[11px] text-muted-foreground">
            支持 YouTube · B站 · X / Twitter · 抖音 · TikTok 等 1700+ 站点（由 yt-dlp 提供）；视频号链接将由 MITM 注入流程接管
          </p>

          {/* Channels branch: show a self-contained guidance panel
              instead of probe + format dropdown. yt-dlp can't decrypt
              Channels streams, but our MITM + injected download button
              can — so the action here is "open in browser" rather than
              "queue download". */}
          {isChannels && (
            <div className="mt-4 rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3">
              <div className="flex items-start gap-2 text-xs">
                <ExternalLink className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
                <div className="space-y-1 leading-relaxed">
                  <p className="font-medium text-foreground">视频号链接</p>
                  <p className="text-muted-foreground">
                    点击下方按钮，将在系统默认浏览器打开此链接；页面加载后会自动出现"下载"按钮，点击即可下载，结果会出现在下载列表里。
                  </p>
                  <p className="text-muted-foreground">
                    首次使用需在 设置 → 代理 中安装根证书，并把代理设置成"系统代理"。
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Probe state UI — only relevant for the yt-dlp branch */}
          {!isChannels && probing && (
            <div className="mt-4 flex items-center gap-2 rounded-md border border-border/40 bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              正在解析…
            </div>
          )}

          {!isChannels && probeError && !probing && (
            <div className="mt-4 flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              <span className="leading-relaxed">{probeError}</span>
            </div>
          )}

          {!isChannels && probe && !probing && (
            <div className="mt-4 rounded-lg border border-border/40 bg-muted/30 p-3">
              <div className="flex gap-3">
                {probe.thumbnail && (
                  <img
                    src={probe.thumbnail}
                    alt=""
                    className="h-16 w-28 shrink-0 rounded object-cover"
                  />
                )}
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium" title={probe.title}>
                    {probe.title}
                  </div>
                  <div className="mt-1 flex items-center gap-2 text-[11px] text-muted-foreground">
                    <span className="font-mono">{probe.site}</span>
                    {probe.uploader && (
                      <>
                        <span>·</span>
                        <span className="truncate">{probe.uploader}</span>
                      </>
                    )}
                    {probe.duration > 0 && (
                      <>
                        <span>·</span>
                        <span>{formatDuration(probe.duration)}</span>
                      </>
                    )}
                  </div>
                </div>
              </div>

              {probe.formats && probe.formats.length > 0 && (
                <div className="mt-3">
                  <label className="mb-1 block text-[11px] font-medium text-muted-foreground">
                    清晰度
                  </label>
                  <select
                    value={formatID}
                    onChange={(e) => setFormatID(e.target.value)}
                    className={cn(
                      'w-full rounded-md border border-border/60 bg-background px-2.5 py-1.5 text-sm',
                      'focus:border-foreground/30 focus:outline-none'
                    )}
                  >
                    {probe.formats.map((f: Format) => (
                      <option key={f.id} value={f.id}>
                        {f.label}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>
          )}

          {/* Advanced options — yt-dlp branch only. Cookie file +
              subtitle langs are meaningless for the Channels MITM
              path (cookies live in the user's browser; subtitles are
              not part of Channels' web stream). */}
          {!isChannels && (
            <>
              <button
                type="button"
                onClick={() => setShowAdvanced((v) => !v)}
                className="mt-4 flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                {showAdvanced ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                高级
              </button>
              {showAdvanced && (
                <div className="mt-2 space-y-3 rounded-md border border-border/40 bg-muted/20 p-3">
                  <div>
                    <label className="mb-1 block text-[11px] font-medium text-muted-foreground">
                      Cookie 文件路径
                    </label>
                    <div className="flex items-center gap-1">
                      <input
                        type="text"
                        value={cookieFile}
                        onChange={(e) => setCookieFile(e.target.value)}
                        placeholder="/path/to/cookies.txt（B站会员 / YouTube 年龄限制视频）"
                        className={cn(
                          'flex-1 rounded-md border border-border/60 bg-background px-2 py-1 text-xs font-mono',
                          'focus:border-foreground/30 focus:outline-none'
                        )}
                      />
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 px-2"
                        title="提示：浏览器扩展 'Get cookies.txt LOCALLY' 可导出"
                        onClick={() => {
                          toast.message(
                            '使用浏览器扩展 "Get cookies.txt LOCALLY" 导出，然后把文件路径填到这里'
                          )
                        }}
                      >
                        <FolderOpen className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                  <div>
                    <label className="mb-1 block text-[11px] font-medium text-muted-foreground">
                      字幕语言
                    </label>
                    <input
                      type="text"
                      value={subLangsRaw}
                      onChange={(e) => setSubLangsRaw(e.target.value)}
                      placeholder="zh,en  （留空则不下载字幕；多个用逗号分隔）"
                      className={cn(
                        'w-full rounded-md border border-border/60 bg-background px-2 py-1 text-xs font-mono',
                        'focus:border-foreground/30 focus:outline-none'
                      )}
                    />
                    {probe?.subLangs && probe.subLangs.length > 0 && (
                      <p className="mt-1 text-[10px] text-muted-foreground">
                        可用：{probe.subLangs.slice(0, 12).join(', ')}
                        {probe.subLangs.length > 12 ? '…' : ''}
                      </p>
                    )}
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-border/40 px-5 py-3">
          <Button variant="ghost" onClick={onClose} disabled={submitting}>
            取消
          </Button>
          <Button onClick={submit} disabled={!canSubmit}>
            {submitting ? (
              <>
                <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
                {isChannels ? '打开中' : '创建中'}
              </>
            ) : isChannels ? (
              <>
                <ExternalLink className="mr-1 h-3.5 w-3.5" /> 在浏览器中打开
              </>
            ) : (
              '开始下载'
            )}
          </Button>
        </div>
      </div>
    </div>
  )
}

function formatDuration(seconds: number): string {
  const s = Math.round(seconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
  }
  return `${m}:${String(sec).padStart(2, '0')}`
}
