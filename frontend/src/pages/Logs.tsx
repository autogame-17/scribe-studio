import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollText } from 'lucide-react'

export function LogsPage() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>日志</CardTitle>
        <CardDescription>核心服务的实时日志流</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col items-center justify-center gap-3 py-16 text-center text-muted-foreground">
          <ScrollText className="h-10 w-10 opacity-40" />
          <p className="text-sm">日志视图即将推出</p>
        </div>
      </CardContent>
    </Card>
  )
}
