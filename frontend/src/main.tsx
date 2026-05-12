import React from 'react'
import { createRoot } from 'react-dom/client'
import './styles.css'
import { App } from './App'
import { applyStoredTheme } from './lib/theme'

// Apply the persisted theme synchronously, before React renders, so the
// first paint already matches the user's saved preference.
applyStoredTheme()

const container = document.getElementById('root')!
const root = createRoot(container)

root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
