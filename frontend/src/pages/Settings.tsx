// SPDX-License-Identifier: GPL-3.0-or-later
import { useEffect, useState } from 'react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  CheckCircle2,
  AlertCircle,
  Sparkles,
  Trash2,
  Download,
  Cpu,
} from 'lucide-react'
import {
  GetAISettings,
  SetAISettings,
  TestAIConnection,
  ClearProofreadCache,
  ListModels,
  DownloadModel,
  GetTranscribeSettings,
  SetAutoTranscribe,
  GetConfig,
  SetProxyAddr,
  SetDownloadDir,
  PickDownloadDir,
  GetXiaoyuzhouAuthStatus,
  SetXiaoyuzhouCredentials,
} from '../../wailsjs/go/scribe/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { proofread, scribe, sphkit } from '../../wailsjs/go/models'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { FolderOpen, RefreshCw } from 'lucide-react'

type Provider = 'none' | 'gemini' | 'bedrock' | 'mock'
type AISettings = proofread.AISettings

const TABS = [
  { key: 'proxy', label: '代理' },
  { key: 'download', label: '下载' },
  { key: 'transcribe', label: '转写' },
  { key: 'ai', label: 'AI 校对' },
  { key: 'advanced', label: '高级' },
] as const

type TabKey = typeof TABS[number]['key']

export function SettingsPage() {
  const [tab, setTab] = useState<TabKey>('ai')

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-1 rounded-md border border-border/60 bg-muted/40 p-0.5">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={cn(
              'rounded-[5px] px-3 py-1 text-[12px] font-medium transition-colors',
              tab === t.key
                ? 'bg-background text-foreground shadow-sm ring-1 ring-border/70'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'ai' && <AITab />}
      {tab === 'proxy' && <ProxyTab />}
      {tab === 'download' && <DownloadTab />}
      {tab === 'transcribe' && <TranscribeTab />}
      {tab === 'advanced' && <AdvancedTab />}
    </div>
  )
}

type ProxyConfig = sphkit.Config

// ProxyTab edits api.hostname / api.port. The interceptor address is
// derived (port - 1) by sphkit so we show it but don't let the user edit
// it directly — keeps the two ports a known offset apart and matches the
// upstream wx_channel CLI behaviour.
function ProxyTab() {
  const [cfg, setCfg] = useState<ProxyConfig | null>(null)
  const [host, setHost] = useState('')
  const [port, setPort] = useState('')
  const [busy, setBusy] = useState(false)

  const refresh = async () => {
    try {
      const c = await GetConfig()
      setCfg(c)
      // Re-hydrate the editable fields from the live config so a save
      // always round-trips through the backend rather than trusting our
      // local state to still match disk.
      const apiAddr = c.apiAddr || ''
      const colon = apiAddr.lastIndexOf(':')
      if (colon > 0) {
        setHost(apiAddr.slice(0, colon))
        setPort(apiAddr.slice(colon + 1))
      }
    } catch (e) {
      toast.error(String(e).replace(/^Error: /, ''))
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  async function save() {
    const portNum = Number(port)
    if (!host.trim()) {
      toast.error('Hostname 不能为空')
      return
    }
    if (!Number.isInteger(portNum) || portNum < 1 || portNum > 65535) {
      toast.error('Port 需要是 1-65535 的整数')
      return
    }
    setBusy(true)
    try {
      await SetProxyAddr(host.trim(), portNum)
      toast.success('已保存', { description: '重启代理后生效' })
      await refresh()
    } catch (e) {
      toast.error(String(e).replace(/^Error: /, ''))
    } finally {
      setBusy(false)
    }
  }

  if (!cfg) {
    return (
      <Card>
        <CardContent className="py-12 text-center text-sm text-muted-foreground">
          读取中…
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>代理</CardTitle>
        <CardDescription>
          API 服务监听地址。拦截端口自动取 API 端口 - 1，启动时由 sphkit 派生。改完需要重启代理（概览页「停止 → 启动」）才生效。
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Field label="API Hostname">
          <input
            value={host}
            onChange={(e) => setHost(e.target.value)}
            placeholder="127.0.0.1"
            className={inputCls + ' font-mono text-xs'}
          />
        </Field>
        <Field label="API Port">
          <input
            value={port}
            onChange={(e) => setPort(e.target.value)}
            inputMode="numeric"
            placeholder="2022"
            className={inputCls + ' font-mono text-xs'}
          />
        </Field>
        <div className="rounded-md border border-border/40 bg-muted/30 p-3 text-xs text-muted-foreground">
          <div>
            拦截端口（只读）：
            <span className="ml-1 font-mono text-foreground/80">{cfg.interceptorAddr || '—'}</span>
          </div>
          <div className="mt-1">
            当前 API 地址：
            <span className="ml-1 font-mono text-foreground/80">{cfg.apiAddr || '—'}</span>
          </div>
        </div>
        <div className="flex justify-end gap-2 pt-1">
          <Button variant="outline" size="sm" onClick={refresh} className="gap-1.5">
            <RefreshCw className="h-3.5 w-3.5" /> 重读
          </Button>
          <Button size="sm" onClick={save} disabled={busy}>
            {busy ? '保存中…' : '保存'}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

// DownloadTab edits download.dir. Folder picker uses Wails'
// OpenDirectoryDialog so the user gets a real native picker. We refuse
// to save an empty path here even though the backend would error too —
// keeps the UI honest about what "" actually means (≈ "use OS default").
function XiaoyuzhouAuthCard() {
  const [refreshToken, setRefreshToken] = useState('')
  const [deviceID, setDeviceID] = useState('')
  const [status, setStatus] = useState<{ configured: boolean; valid: boolean } | null>(null)
  const [busy, setBusy] = useState(false)

  const refresh = async () => {
    try {
      const s = await GetXiaoyuzhouAuthStatus()
      setStatus(s)
    } catch {
      setStatus(null)
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  async function save() {
    if (!refreshToken.trim() || !deviceID.trim()) {
      toast.error('请填写 refresh_token 和 device_id')
      return
    }
    setBusy(true)
    try {
      await SetXiaoyuzhouCredentials(refreshToken.trim(), deviceID.trim())
      toast.success('小宇宙凭证已保存')
      setRefreshToken('')
      setDeviceID('')
      await refresh()
    } catch (e) {
      toast.error(String(e).replace(/^Error: /, ''))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="rounded-lg border border-border/50 bg-muted/20 p-3 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="text-sm font-medium">小宇宙</div>
        {status?.valid ? (
          <Badge variant="outline" className="text-emerald-600 border-emerald-500/40">已登录</Badge>
        ) : status?.configured ? (
          <Badge variant="outline" className="text-amber-600 border-amber-500/40">凭证失效</Badge>
        ) : (
          <Badge variant="outline">未配置</Badge>
        )}
      </div>
      <p className="text-[11px] text-muted-foreground leading-relaxed">
        集成 xyz-dl 同款 API。请用抓包或{' '}
        <a
          href="https://github.com/shiquda/xyz-dl"
          className="underline hover:text-foreground"
          target="_blank"
          rel="noreferrer"
        >
          xyz-dl
        </a>{' '}
        获取 refresh_token 与 device_id（须成对匹配）。
      </p>
      <Field label="refresh_token">
        <input
          type="password"
          value={refreshToken}
          onChange={(e) => setRefreshToken(e.target.value)}
          placeholder="粘贴 refresh_token"
          className={inputCls + ' font-mono text-xs'}
        />
      </Field>
      <Field label="device_id">
        <input
          value={deviceID}
          onChange={(e) => setDeviceID(e.target.value)}
          placeholder="x-jike-device-id"
          className={inputCls + ' font-mono text-xs'}
        />
      </Field>
      <div className="flex justify-end">
        <Button size="sm" onClick={save} disabled={busy}>
          {busy ? '验证中…' : '保存凭证'}
        </Button>
      </div>
    </div>
  )
}

function DownloadTab() {
  const [cfg, setCfg] = useState<ProxyConfig | null>(null)
  const [dir, setDir] = useState('')
  const [busy, setBusy] = useState(false)

  const refresh = async () => {
    try {
      const c = await GetConfig()
      setCfg(c)
      setDir(c.downloadDir || '')
    } catch (e) {
      toast.error(String(e).replace(/^Error: /, ''))
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  async function pick() {
    try {
      const picked = await PickDownloadDir()
      if (picked) setDir(picked)
    } catch (e) {
      toast.error(String(e).replace(/^Error: /, ''))
    }
  }

  async function save() {
    if (!dir.trim()) {
      toast.error('请选择一个目录')
      return
    }
    setBusy(true)
    try {
      await SetDownloadDir(dir.trim())
      toast.success('已保存', {
        description: '新下载会落到这个目录；视频号代理需重启后才会切换',
      })
      await refresh()
    } catch (e) {
      toast.error(String(e).replace(/^Error: /, ''))
    } finally {
      setBusy(false)
    }
  }

  if (!cfg) {
    return (
      <Card>
        <CardContent className="py-12 text-center text-sm text-muted-foreground">
          读取中…
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>下载</CardTitle>
        <CardDescription>
          视频号、yt-dlp、小宇宙下载均保存到此目录。yt-dlp / 小宇宙保存后立即生效；视频号代理需重启后切换路径。
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <XiaoyuzhouAuthCard />
        <Field label="下载目录">
          <div className="flex items-center gap-2">
            <input
              value={dir}
              onChange={(e) => setDir(e.target.value)}
              placeholder="/Users/you/Downloads/Scribe"
              className={inputCls + ' font-mono text-xs'}
            />
            <Button
              variant="outline"
              size="sm"
              className="h-9 shrink-0 gap-1"
              onClick={pick}
              title="浏览…"
            >
              <FolderOpen className="h-3.5 w-3.5" /> 浏览
            </Button>
          </div>
        </Field>
        <div className="text-[11px] text-muted-foreground">
          并发数（MaxRunning）当前为上游硬编码的 3，可配置化排在 v0.5。
        </div>
        <div className="flex justify-end gap-2 pt-1">
          <Button variant="outline" size="sm" onClick={refresh} className="gap-1.5">
            <RefreshCw className="h-3.5 w-3.5" /> 重读
          </Button>
          <Button size="sm" onClick={save} disabled={busy}>
            {busy ? '保存中…' : '保存'}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function AdvancedTab() {
  async function clearCache() {
    try {
      await ClearProofreadCache()
      toast.success('已清空 LLM 缓存')
    } catch (e) {
      toast.error(String(e))
    }
  }
  return (
    <Card>
      <CardHeader>
        <CardTitle>高级</CardTitle>
        <CardDescription>调试开关 + 缓存清理</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div className="flex items-center justify-between gap-4 border-b border-border/40 py-2">
          <div>
            <div className="font-medium">LLM 校对缓存</div>
            <div className="text-xs text-muted-foreground">
              清掉后下一次校对会重新调用 AI provider
            </div>
          </div>
          <Button variant="outline" size="sm" onClick={clearCache} className="gap-1">
            <Trash2 className="h-3.5 w-3.5" /> 清空
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function AITab() {
  const [settings, setSettings] = useState<AISettings | null>(null)
  const [testing, setTesting] = useState(false)
  const [testOK, setTestOK] = useState<null | { ok: boolean; msg: string }>(null)

  useEffect(() => {
    GetAISettings().then(setSettings).catch(() => {})
  }, [])

  if (!settings) {
    return (
      <Card>
        <CardContent className="py-12 text-center text-sm text-muted-foreground">
          读取中…
        </CardContent>
      </Card>
    )
  }

  function patch(u: Partial<AISettings>) {
    setSettings({ ...settings!, ...u } as AISettings)
  }

  async function save() {
    try {
      await SetAISettings(settings!)
      toast.success('已保存')
    } catch (e) {
      toast.error(String(e))
    }
  }

  async function test() {
    setTesting(true)
    setTestOK(null)
    try {
      // Pass the live form state — letting the user verify a key /
      // proxy combination before persisting saves them a "save then
      // test then realise it's wrong" round trip.
      const reply = await TestAIConnection(settings!)
      setTestOK({ ok: true, msg: reply })
      toast.success('AI 连通', { description: reply })
    } catch (e) {
      const msg = String(e).replace(/^Error: /, '')
      setTestOK({ ok: false, msg })
      toast.error('测试失败：' + msg)
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Sparkles className="h-4 w-4 text-muted-foreground" />
            AI Provider
          </CardTitle>
          <CardDescription>
            选一个用于校对的 LLM。None 表示暂不启用；Mock 只产固定响应，用来联调前端。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {(['none', 'gemini', 'bedrock', 'mock'] as Provider[]).map((p) => (
              <button
                key={p}
                onClick={() => patch({ provider: p })}
                className={cn(
                  'rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                  settings.provider === p
                    ? 'border-primary bg-primary/10 text-foreground'
                    : 'border-border/60 text-muted-foreground hover:bg-accent/40'
                )}
              >
                {p === 'none' ? '关闭' : p.charAt(0).toUpperCase() + p.slice(1)}
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      {settings.provider === 'gemini' && (
        <Card>
          <CardHeader>
            <CardTitle>Google Gemini</CardTitle>
            <CardDescription>
              去{' '}
              <a
                href="https://aistudio.google.com/apikey"
                target="_blank"
                rel="noreferrer"
                className="underline underline-offset-2"
              >
                AI Studio
              </a>{' '}
              拿一个免费 key
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="API Key">
              <input
                type="password"
                value={settings.gemini.apiKey}
                onChange={(e) =>
                  patch({ gemini: { ...settings.gemini, apiKey: e.target.value } })
                }
                placeholder="AIza..."
                className={inputCls}
              />
            </Field>
            <Field label="模型">
              <input
                value={settings.gemini.model}
                onChange={(e) =>
                  patch({ gemini: { ...settings.gemini, model: e.target.value } })
                }
                className={inputCls + ' font-mono text-xs'}
              />
            </Field>
            <ProxyField
              value={settings.gemini.proxyURL ?? ''}
              onChange={(v) =>
                patch({ gemini: { ...settings.gemini, proxyURL: v } })
              }
              hint="国内访问 generativelanguage.googleapis.com 一般要走 VPN。常见值：http://127.0.0.1:7890（Clash）或 socks5://127.0.0.1:7891"
            />
          </CardContent>
        </Card>
      )}

      {settings.provider === 'bedrock' && (
        <Card>
          <CardHeader>
            <CardTitle>AWS Bedrock (Claude)</CardTitle>
            <CardDescription>需要在 AWS 区域里开通 Anthropic 模型访问</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="Region">
              <input
                value={settings.bedrock.region}
                onChange={(e) =>
                  patch({ bedrock: { ...settings.bedrock, region: e.target.value } })
                }
                placeholder="us-east-1"
                className={inputCls + ' font-mono text-xs'}
              />
            </Field>
            <Field label="Access Key ID">
              <input
                type="password"
                value={settings.bedrock.accessKey}
                onChange={(e) =>
                  patch({ bedrock: { ...settings.bedrock, accessKey: e.target.value } })
                }
                className={inputCls + ' font-mono text-xs'}
              />
            </Field>
            <Field label="Secret Access Key">
              <input
                type="password"
                value={settings.bedrock.secretKey}
                onChange={(e) =>
                  patch({ bedrock: { ...settings.bedrock, secretKey: e.target.value } })
                }
                className={inputCls + ' font-mono text-xs'}
              />
            </Field>
            <Field label="模型">
              <input
                value={settings.bedrock.model}
                onChange={(e) =>
                  patch({ bedrock: { ...settings.bedrock, model: e.target.value } })
                }
                className={inputCls + ' font-mono text-xs'}
              />
            </Field>
            <ProxyField
              value={settings.bedrock.proxyURL ?? ''}
              onChange={(v) =>
                patch({ bedrock: { ...settings.bedrock, proxyURL: v } })
              }
              hint="一般 AWS 不需要代理；只有当 bedrock-runtime 域名被网络限制时再填。"
            />
          </CardContent>
        </Card>
      )}

      <div className="flex items-center justify-between rounded-xl border border-border/40 bg-card/50 p-3">
        <div className="flex items-center gap-2 text-sm">
          {testOK === null ? (
            <span className="text-muted-foreground">保存后可以点「测试连通」验一下</span>
          ) : testOK.ok ? (
            <>
              <CheckCircle2 className="h-4 w-4 text-emerald-500" />{' '}
              <span>连通：{testOK.msg}</span>
            </>
          ) : (
            <>
              <AlertCircle className="h-4 w-4 text-destructive" />{' '}
              <span className="text-destructive">{testOK.msg}</span>
            </>
          )}
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={test}
            disabled={testing || settings.provider === 'none'}
          >
            {testing ? '测试中…' : '测试连通'}
          </Button>
          <Button size="sm" onClick={save}>
            保存
          </Button>
        </div>
      </div>

      <div className="text-[11px] text-muted-foreground">
        密钥明文保存在{' '}
        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[10px]">
          ~/Library/Application Support/Scribe/ai-settings.json
        </code>{' '}
        （mode 0600）。后续版本会接 macOS Keychain。
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-[11px] uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
      {children}
    </label>
  )
}

// ProxyField renders the "HTTP/SOCKS5 代理" input common to every
// provider that needs to reach an upstream HTTPS host. The hint
// surfaces region-specific advice (e.g. Clash on 7890 for Chinese
// Gemini users) without us having to invent a settings explorer.
function ProxyField({
  value,
  onChange,
  hint,
}: {
  value: string
  onChange: (v: string) => void
  hint?: string
}) {
  return (
    <div className="block">
      <span className="mb-1 block text-[11px] uppercase tracking-wider text-muted-foreground">
        代理 URL
      </span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="留空表示直连。示例：http://127.0.0.1:7890 或 socks5://127.0.0.1:7891"
        className={inputCls + ' font-mono text-xs'}
      />
      {hint && (
        <p className="mt-1 text-[11px] leading-relaxed text-muted-foreground/80">{hint}</p>
      )}
    </div>
  )
}

const inputCls =
  'w-full rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/30'

type Model = scribe.ModelSummary

function TranscribeTab() {
  const [models, setModels] = useState<Model[]>([])
  const [loading, setLoading] = useState(true)
  const [progress, setProgress] = useState<Record<string, { frac: number; msg: string }>>({})
  const [downloading, setDownloading] = useState<Record<string, boolean>>({})

  useEffect(() => {
    let cancelled = false
    async function refresh() {
      try {
        const list = await ListModels()
        if (!cancelled) setModels(list ?? [])
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    refresh()

    const offProgress = EventsOn(
      'model:progress',
      (p: { key: string; fraction: number; message: string }) => {
        setProgress((prev) => ({ ...prev, [p.key]: { frac: p.fraction, msg: p.message } }))
      }
    )
    const offDone = EventsOn('model:done', (p: { key: string; error?: string }) => {
      setDownloading((prev) => ({ ...prev, [p.key]: false }))
      setProgress((prev) => {
        const next = { ...prev }
        delete next[p.key]
        return next
      })
      if (p.error) {
        toast.error(`下载失败：${p.error}`)
      } else {
        toast.success('模型已安装')
      }
      refresh()
    })
    return () => {
      cancelled = true
      offProgress()
      offDone()
    }
  }, [])

  async function install(key: string) {
    setDownloading((prev) => ({ ...prev, [key]: true }))
    try {
      await DownloadModel(key)
      toast.info('下载开始…')
    } catch (e) {
      setDownloading((prev) => ({ ...prev, [key]: false }))
      toast.error(String(e))
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Cpu className="h-4 w-4 text-muted-foreground" />
            Whisper 模型
          </CardTitle>
          <CardDescription>
            本地跑 ASR 必需。模型文件存在{' '}
            <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
              ~/Library/Application Support/Scribe/models/
            </code>
            ；越大质量越好、越慢。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {loading ? (
            <div className="py-8 text-center text-sm text-muted-foreground">读取中…</div>
          ) : (
            models.map((m) => {
              const p = progress[m.key]
              const isDownloading = downloading[m.key] || !!p
              const pct = p && p.frac > 0 ? Math.round(p.frac * 100) : 0
              return (
                <div
                  key={m.key}
                  className="flex items-center gap-3 rounded-lg border border-border/40 bg-card/40 px-4 py-3"
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-medium">{m.key}</span>
                      {m.installed && <Badge variant="success">已安装</Badge>}
                      {/* Quantized badge: keys carrying the ggml q5_0
                          / q8_0 etc. suffix are smaller + faster but
                          have a (barely visible) quality cost. We tag
                          them so users can tell at a glance. */}
                      {/q[0-9]+_[0-9]+/.test(m.key) && (
                        <Badge variant="outline" className="font-mono text-[10px]">
                          quantized
                        </Badge>
                      )}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">{m.label}</div>
                    {isDownloading && p && (
                      <div className="mt-2">
                        <div className="flex justify-between text-[11px] text-muted-foreground">
                          <span>{p.msg}</span>
                          {p.frac > 0 && <span>{pct}%</span>}
                        </div>
                        <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-muted">
                          <div
                            className="h-full rounded-full bg-emerald-500 transition-all"
                            style={{ width: `${pct}%` }}
                          />
                        </div>
                      </div>
                    )}
                  </div>
                  {!m.installed && !isDownloading && (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => install(m.key)}
                      className="gap-1.5"
                    >
                      <Download className="h-3.5 w-3.5" /> 下载
                    </Button>
                  )}
                </div>
              )
            })
          )}
        </CardContent>
      </Card>

      <AutoTranscribeCard />
    </div>
  )
}

// AutoTranscribeCard renders the watcher's auto-enqueue toggle. We
// fetch the current value from the backend on mount (rather than
// tracking it locally) so the UI stays in sync if another window
// flips the bit via Wails — and so a refresh / re-mount doesn't
// silently flip back to the React default.
function AutoTranscribeCard() {
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    GetTranscribeSettings()
      .then((s) => setEnabled(!!s.autoEnabled))
      .catch(() => setEnabled(false))
  }, [])

  async function toggle() {
    if (enabled === null) return
    const next = !enabled
    setBusy(true)
    setEnabled(next)
    try {
      await SetAutoTranscribe(next)
      toast.success(next ? '已开启自动转写' : '已关闭自动转写')
    } catch (e) {
      // Roll back on failure so the UI doesn't claim a state the
      // pipeline didn't accept.
      setEnabled(!next)
      toast.error(String(e).replace(/^Error: /, ''))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>自动转写</CardTitle>
        <CardDescription>
          下载完成后是否自动跑 Whisper。关掉只影响新下载——已经在跑的不受影响，手动 Retry 也始终可用。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-4 rounded-lg border border-border/40 bg-card/40 px-4 py-3">
          <div className="min-w-0">
            <div className="text-sm font-medium">下载完成后自动转写</div>
            <div className="mt-0.5 text-xs text-muted-foreground">
              {enabled === null ? '读取中…' : enabled ? '开启中：每个完成的下载会自动入队' : '已关闭：需要手动点「转写」'}
            </div>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={enabled === true}
            disabled={busy || enabled === null}
            onClick={toggle}
            className={cn(
              'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors',
              'focus:outline-none focus:ring-2 focus:ring-ring/40 disabled:cursor-not-allowed disabled:opacity-60',
              enabled ? 'bg-emerald-500' : 'bg-muted'
            )}
          >
            <span
              className={cn(
                'inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform',
                enabled ? 'translate-x-[18px]' : 'translate-x-[3px]'
              )}
            />
          </button>
        </div>
      </CardContent>
    </Card>
  )
}
