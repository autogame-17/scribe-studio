import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ExternalLink } from 'lucide-react'

export function AboutPage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Scribe</CardTitle>
          <CardDescription>多平台视频 → 转写 → 智能校对</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm leading-relaxed text-muted-foreground">
          <p>
            Scribe 把音视频转成高质量文字稿。下载完自动跑本地 Whisper，配合 LLM 校对 + 智能词表，逐渐把你的常用术语"喂"给工具——Typeless 风格的增量学习。
          </p>
          <p>
            本期支持微信视频号（内置 MITM 代理），B 站 / YouTube 等会在后续版本接入。视频号下载部分基于{' '}
            <a
              href="https://github.com/ltaoo/wx_channels_download"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 text-foreground/80 underline-offset-4 hover:underline"
            >
              ltaoo/wx_channels_download
              <ExternalLink className="h-3 w-3" />
            </a>
            （MIT + Commons Clause）。UI 脚手架对齐 autogame-17/prism。
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>致谢</CardTitle>
          <CardDescription>本项目站在这些开源项目的肩膀上</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <CreditRow name="ltaoo/wx_channels_download" note="视频号 MITM 代理 + 注入脚本" />
          <CreditRow name="ggerganov/whisper.cpp" note="本地 Whisper 推理" />
          <CreditRow name="FFmpeg" note="音视频格式转换" />
          <CreditRow name="GopeedLab/gopeed" note="多线程下载引擎" />
          <CreditRow name="wailsapp/wails" note="Go + Web 原生桌面框架" />
          <CreditRow name="shadcn/ui · Radix · lucide-react" note="UI 组件与图标" />
        </CardContent>
      </Card>
    </div>
  )
}

function CreditRow({ name, note }: { name: string; note: string }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border/40 py-2 last:border-0">
      <span className="font-mono text-xs text-foreground/90">{name}</span>
      <span className="text-xs text-muted-foreground">{note}</span>
    </div>
  )
}
