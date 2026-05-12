import { useEffect, useState } from 'react'
import { HashRouter, Route, Routes } from 'react-router-dom'
import { Toaster } from 'sonner'
import { Sidebar } from '@/components/layout/sidebar'
import { Topbar } from '@/components/layout/topbar'
import { DashboardPage } from '@/pages/Dashboard'
import { DownloadsPage } from '@/pages/Downloads'
import { TranscriptsPage } from '@/pages/Transcripts'
import { EditorPage } from '@/pages/Editor'
import { LogsPage } from '@/pages/Logs'
import { SettingsPage } from '@/pages/Settings'
import { AboutPage } from '@/pages/About'
import { GetProxyStatus } from '../wailsjs/go/scribe/App'

type ProxyState = 'running' | 'stopped' | 'error' | 'starting'

export function App() {
  const [status, setStatus] = useState<ProxyState>('stopped')

  useEffect(() => {
    let cancelled = false
    async function check() {
      try {
        const s = await GetProxyStatus()
        if (cancelled) return
        if (s.running) setStatus('running')
        else if (s.lastError) setStatus('error')
        else setStatus('stopped')
      } catch {
        if (!cancelled) setStatus('error')
      }
    }
    check()
    const id = setInterval(check, 3000)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [])

  return (
    <HashRouter>
      <div className="flex h-screen overflow-hidden bg-background text-foreground selection:bg-primary/30">
        {/* Subtle ambient gradient — same idea as Prism's hero gradient */}
        <div className="pointer-events-none fixed inset-0 z-0 bg-[radial-gradient(ellipse_80%_80%_at_50%_-20%,rgba(168,85,247,0.10),rgba(255,255,255,0))]" />
        <div className="z-10 flex h-full w-full">
          <Sidebar />
          <div className="relative flex flex-1 flex-col overflow-hidden">
            <Topbar status={status} />
            <main className="sph-scroll flex-1 overflow-y-auto p-8">
              <div className="mx-auto max-w-5xl">
                <Routes>
                  <Route path="/" element={<DashboardPage />} />
                  <Route path="/downloads" element={<DownloadsPage />} />
                  <Route path="/transcripts" element={<TranscriptsPage />} />
                  <Route path="/editor/:taskID" element={<EditorPage />} />
                  <Route path="/logs" element={<LogsPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="/about" element={<AboutPage />} />
                </Routes>
              </div>
            </main>
          </div>
        </div>
        <Toaster
          position="bottom-right"
          toastOptions={{
            classNames: {
              toast:
                'group bg-card text-card-foreground border-border/60 shadow-lg rounded-xl',
            },
          }}
        />
      </div>
    </HashRouter>
  )
}

export default App
