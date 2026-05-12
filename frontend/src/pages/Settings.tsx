import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function SettingsPage() {
  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>代理</CardTitle>
          <CardDescription>MITM 监听的本地地址与端口</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <SettingRow label="Host" value="127.0.0.1" />
          <SettingRow label="Port" value="2023" />
          <SettingRow label="自动设为系统代理" value="开启" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>下载</CardTitle>
          <CardDescription>下载目录与并发策略</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <SettingRow label="下载目录" value="~/Downloads/sph" mono />
          <SettingRow label="最大并发" value="3" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>高级</CardTitle>
          <CardDescription>调试用的开关与文件位置</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <SettingRow label="配置文件" value="config.yaml" mono />
          <SettingRow label="调试日志" value="关闭" />
        </CardContent>
      </Card>
    </div>
  )
}

function SettingRow({
  label,
  value,
  mono,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border/40 py-2 last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? 'font-mono text-xs' : 'font-medium'}>{value}</span>
    </div>
  )
}
