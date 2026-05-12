import { useEffect, useState } from 'react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { CheckCircle2, AlertCircle, Sparkles, Trash2 } from 'lucide-react'
import {
  GetAISettings,
  SetAISettings,
  TestAIConnection,
  ClearProofreadCache,
} from '../../wailsjs/go/scribe/App'
import type { proofread } from '../../wailsjs/go/models'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

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
      {tab === 'proxy' && <PlaceholderTab title="代理" note="host/port/系统代理开关" />}
      {tab === 'download' && <PlaceholderTab title="下载" note="下载目录、并发、命名模板" />}
      {tab === 'transcribe' && <PlaceholderTab title="转写" note="模型、语言、自动触发（部分绑定已在 Dashboard）" />}
      {tab === 'advanced' && <AdvancedTab />}
    </div>
  )
}

function PlaceholderTab({ title, note }: { title: string; note: string }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{note}</CardDescription>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        这一页的可编辑字段还在路上。相关字段当前从{' '}
        <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">config.yaml</code>{' '}
        读取。
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
      const reply = await TestAIConnection()
      setTestOK({ ok: true, msg: reply })
      toast.success('AI 连通', { description: reply })
    } catch (e) {
      setTestOK({ ok: false, msg: String(e) })
      toast.error('测试失败：' + String(e))
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

const inputCls =
  'w-full rounded-md border border-border/60 bg-background px-3 py-1.5 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/30'
