// SPDX-License-Identifier: GPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Search, Plus, Trash2, X, TrendingUp } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  ListGlossary,
  UpsertGlossary,
  DeleteGlossary,
} from '../../wailsjs/go/scribe/App'
import type { proofread } from '../../wailsjs/go/models'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

type Entry = proofread.Entry

/**
 * GlossaryDrawer — side sheet with full CRUD over the user's
 * glossary.  Backed by AppSupport/Scribe/glossary.json via the
 * ListGlossary / UpsertGlossary / DeleteGlossary Wails bindings.
 */
export function GlossaryDrawer({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const [entries, setEntries] = useState<Entry[]>([])
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<Entry | null>(null)

  const refresh = useCallback(async () => {
    try {
      const list = await ListGlossary(query)
      setEntries(list ?? [])
    } catch (e) {
      toast.error(String(e))
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => { if (open) refresh() }, [open, refresh])

  async function save(e: Entry) {
    try {
      await UpsertGlossary(e)
      setEditing(null)
      toast.success('已保存')
      refresh()
    } catch (err) {
      toast.error(String(err))
    }
  }

  async function remove(id: string) {
    try {
      await DeleteGlossary(id)
      toast.success('已删除')
      refresh()
    } catch (err) {
      toast.error(String(err))
    }
  }

  function startNew() {
    setEditing({
      id: '',
      right: '',
      wrong: [],
      category: 'custom',
      source: 'user',
      hitCount: 0,
      createdAt: '',
    } as Entry)
  }

  return (
    <>
      {/* Backdrop */}
      <div
        onClick={onClose}
        className={cn(
          'fixed inset-0 z-40 bg-black/30 backdrop-blur-sm transition-opacity',
          open ? 'opacity-100' : 'pointer-events-none opacity-0'
        )}
      />
      {/* Sheet */}
      <aside
        className={cn(
          'fixed right-0 top-0 z-50 flex h-screen w-[480px] flex-col border-l border-border bg-background shadow-2xl transition-transform',
          open ? 'translate-x-0' : 'translate-x-full'
        )}
      >
        <header className="flex items-center justify-between border-b border-border/60 px-5 py-4">
          <div>
            <h2 className="text-base font-semibold">词表</h2>
            <p className="text-xs text-muted-foreground">
              确定性替换规则·按命中次数排序
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" onClick={startNew} className="gap-1">
              <Plus className="h-3.5 w-3.5" /> 新增
            </Button>
            <button
              onClick={onClose}
              className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="border-b border-border/60 px-5 py-3">
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
            <input
              placeholder="搜 right / wrong / 分类"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="w-full rounded-md border border-border/60 bg-background py-1.5 pl-8 pr-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/30"
            />
          </div>
        </div>

        <div className="sph-scroll flex-1 overflow-y-auto">
          {loading ? (
            <div className="p-8 text-center text-sm text-muted-foreground">读取中…</div>
          ) : entries.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">
              词表为空 — 点右上「新增」创建一条
            </div>
          ) : (
            <ul className="divide-y divide-border/40">
              {entries.map((e) => (
                <EntryRow
                  key={e.id}
                  entry={e}
                  onEdit={() => setEditing(e)}
                  onDelete={() => remove(e.id)}
                />
              ))}
            </ul>
          )}
        </div>

        {editing && (
          <EntryEditor
            entry={editing}
            onCancel={() => setEditing(null)}
            onSave={save}
          />
        )}
      </aside>
    </>
  )
}

function EntryRow({
  entry,
  onEdit,
  onDelete,
}: {
  entry: Entry
  onEdit: () => void
  onDelete: () => void
}) {
  return (
    <li className="group flex items-start gap-3 px-5 py-3 hover:bg-accent/40">
      <div className="min-w-0 flex-1 cursor-pointer" onClick={onEdit}>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-mono text-sm font-medium">{entry.right}</span>
          <Badge variant="outline" className="h-4 px-1.5 text-[10px]">
            {entry.category}
          </Badge>
          {entry.source === 'seed' ? (
            <Badge variant="outline" className="h-4 px-1.5 text-[10px] opacity-60">
              种子
            </Badge>
          ) : (
            <Badge variant="outline" className="h-4 px-1.5 text-[10px]">
              自定义
            </Badge>
          )}
          {entry.hitCount > 0 && (
            <span className="inline-flex items-center gap-0.5 text-[10px] text-muted-foreground">
              <TrendingUp className="h-3 w-3" />
              {entry.hitCount}
            </span>
          )}
        </div>
        <div className="mt-1 flex flex-wrap gap-1">
          {(entry.wrong ?? []).length > 0 ? (
            (entry.wrong ?? []).map((w, i) => (
              <code
                key={i}
                className="rounded bg-muted/70 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground"
              >
                {w}
              </code>
            ))
          ) : (
            <span className="text-[11px] text-muted-foreground">仅作为 AI 术语提示</span>
          )}
        </div>
      </div>
      <button
        onClick={onDelete}
        className="rounded-md p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100"
        title="删除"
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>
    </li>
  )
}

function EntryEditor({
  entry,
  onCancel,
  onSave,
}: {
  entry: Entry
  onCancel: () => void
  onSave: (e: Entry) => void
}) {
  const [right, setRight] = useState(entry.right)
  const [wrongsText, setWrongsText] = useState((entry.wrong ?? []).join('\n'))
  const [category, setCategory] = useState(entry.category || 'custom')

  function submit() {
    const wrongs = wrongsText
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
    if (!right.trim()) {
      toast.error('Right 不能为空')
      return
    }
    onSave({
      ...entry,
      right: right.trim(),
      wrong: wrongs,
      category,
    } as Entry)
  }

  return (
    <div className="border-t border-border bg-card/60 px-5 py-4 backdrop-blur">
      <div className="mb-3 text-xs font-medium text-muted-foreground">
        {entry.id ? '编辑条目' : '新建条目'}
      </div>
      <div className="space-y-2">
        <label className="block">
          <span className="mb-1 block text-[11px] text-muted-foreground">
            Right（正确写法）
          </span>
          <input
            value={right}
            onChange={(e) => setRight(e.target.value)}
            placeholder="Evolver"
            className="w-full rounded-md border border-border/60 bg-background px-2 py-1 font-mono text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/30"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-[11px] text-muted-foreground">
            Wrong（一行一条，可留空；留空时只作为 AI 术语提示）
          </span>
          <textarea
            value={wrongsText}
            onChange={(e) => setWrongsText(e.target.value)}
            rows={3}
            placeholder={'伊沃弗\n依沃弗\nevolver'}
            className="w-full resize-y rounded-md border border-border/60 bg-background px-2 py-1 font-mono text-xs outline-none focus:border-ring focus:ring-2 focus:ring-ring/30"
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-[11px] text-muted-foreground">分类</span>
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className="w-full rounded-md border border-border/60 bg-background px-2 py-1 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-ring/30"
          >
            <option value="brand">brand（品牌/产品）</option>
            <option value="term">term（术语）</option>
            <option value="person">person（人名）</option>
            <option value="custom">custom（自定义）</option>
          </select>
        </label>
      </div>
      <div className="mt-3 flex justify-end gap-2">
        <Button size="sm" variant="outline" onClick={onCancel}>取消</Button>
        <Button size="sm" onClick={submit}>保存</Button>
      </div>
    </div>
  )
}
