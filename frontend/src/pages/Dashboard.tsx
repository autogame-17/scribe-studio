import { useEffect, useState } from 'react'
import { Play, Square, ShieldCheck, ShieldAlert, FolderOpen, Copy } from 'lucide-react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import {
  GetConfig,
  GetProxyStatus,
  GetVersion,
  OpenInFinder,
  StartProxy,
  StopProxy,
} from '../../wailsjs/go/scribe/App'
import type { scribe, sphkit } from '../../wailsjs/go/models'

type Status = scribe.ProxyStatus
type Version = scribe.VersionInfo
type Config = sphkit.Config

export function DashboardPage() {
  const [status, setStatus] = useState<Status | null>(null)
  const [version, setVersion] = useState<Version | null>(null)
  const [config, setConfig] = useState<Config | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    GetVersion().then(setVersion).catch(() => {})
    GetConfig().then(setConfig).catch(() => {})
    let cancelled = false
    async function pull() {
      try {
        const s = await GetProxyStatus()
        if (!cancelled) setStatus(s)
      } catch {
        /* ignore transient errors */
      }
    }
    pull()
    const id = setInterval(pull, 2000)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [])

  async function toggle() {
    setBusy(true)
    try {
      if (status?.running) {
        await StopProxy()
        toast.success('代理已停止')
      } else {
        await StartProxy()
        toast.success('代理启动成功')
      }
      const s = await GetProxyStatus()
      setStatus(s)
    } catch (e) {
      toast.error(String(e))
    } finally {
      setBusy(false)
    }
  }

  async function copyAddr() {
    const addr = status?.interceptorAddr
    if (!addr) return
    await navigator.clipboard.writeText(`http://${addr}`)
    toast.success('代理地址已复制')
  }

  const certInstalled = false // TODO: wire to GetCertStatus once it lands

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
            <div>
              <CardTitle>代理服务</CardTitle>
              <CardDescription>
                启动后拦截微信 PC 客户端的视频号流量，给视频页面注入下载按钮。
              </CardDescription>
            </div>
            <ProxyBadge running={status?.running} error={status?.lastError} />
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-lg border bg-muted/40 p-4">
              <div className="text-xs uppercase tracking-wider text-muted-foreground">
                代理地址
              </div>
              <div className="mt-2 flex items-center gap-2">
                <code className="flex-1 truncate font-mono text-sm">
                  {status?.interceptorAddr
                    ? `http://${status.interceptorAddr}`
                    : '未启动'}
                </code>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={copyAddr}
                  disabled={!status?.interceptorAddr}
                >
                  <Copy className="h-3.5 w-3.5" /> 复制
                </Button>
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              {status?.running ? (
                <Button variant="outline" onClick={toggle} disabled={busy}>
                  <Square className="h-4 w-4" /> 停止
                </Button>
              ) : (
                <Button onClick={toggle} disabled={busy}>
                  <Play className="h-4 w-4" /> 启动
                </Button>
              )}
            </div>

            {status?.lastError && (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
                {status.lastError}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-start justify-between gap-2 space-y-0">
            <div>
              <CardTitle>证书</CardTitle>
              <CardDescription>HTTPS 拦截所需的本地 CA</CardDescription>
            </div>
            {certInstalled ? (
              <Badge variant="success">已安装</Badge>
            ) : (
              <Badge variant="outline">未检测</Badge>
            )}
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex items-start gap-3 text-muted-foreground">
              {certInstalled ? (
                <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
              ) : (
                <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
              )}
              <p className="leading-relaxed">
                视频号下载依赖本地 MITM，需要把 CA 证书加入系统信任。证书安装流程将在下一版本内置。
              </p>
            </div>
            <Button variant="outline" size="sm" disabled className="w-full">
              安装证书（即将推出）
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>使用步骤</CardTitle>
          <CardDescription>四步完成一次下载</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Step n={1} title="安装证书" desc="首次使用需要把本地 CA 加入系统信任。" />
          <Step n={2} title="启动代理" desc="点右上方「启动」按钮。状态灯变绿即可。" />
          <Step n={3} title="打开微信视频号" desc="在 PC 客户端播放想下载的视频并暂停。" />
          <Step n={4} title="点击下载按钮" desc="页面里注入的下载按钮会自动出现。" />
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <MiniCard label="App 版本" value={`v${version?.app ?? '?'}`} />
        <MiniCard label="核心" value={version?.core ?? 'wx_channel'} mono />
        <MiniCard
          label="下载目录"
          value={config?.downloadDir ?? '加载中…'}
          mono
          action={
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2"
              onClick={() => config?.downloadDir && OpenInFinder(config.downloadDir)}
              disabled={!config?.downloadDir}
              title="在 Finder 中打开"
            >
              <FolderOpen className="h-3.5 w-3.5" />
            </Button>
          }
        />
      </div>
    </div>
  )
}

function ProxyBadge({ running, error }: { running?: boolean; error?: string }) {
  if (running) return <Badge variant="success">运行中</Badge>
  if (error) return <Badge variant="destructive">异常</Badge>
  return <Badge variant="outline">已停止</Badge>
}

function Step({ n, title, desc }: { n: number; title: string; desc: string }) {
  return (
    <div className="flex gap-3">
      <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-foreground/[0.06] text-[11px] font-semibold text-foreground/80">
        {n}
      </div>
      <div className="min-w-0">
        <div className="text-sm font-medium">{title}</div>
        <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{desc}</div>
      </div>
    </div>
  )
}

function MiniCard({
  label,
  value,
  mono,
  action,
}: {
  label: string
  value: string
  mono?: boolean
  action?: React.ReactNode
}) {
  return (
    <div className="flex items-center justify-between rounded-xl border border-border/40 bg-card/60 px-4 py-3 shadow-sm backdrop-blur-xl">
      <div className="min-w-0">
        <div className="text-[11px] uppercase tracking-wider text-muted-foreground">{label}</div>
        <div
          className={
            'mt-1 truncate text-sm ' + (mono ? 'font-mono text-foreground/90' : 'font-medium')
          }
        >
          {value}
        </div>
      </div>
      {action}
    </div>
  )
}
