import { Routes, Route, NavLink } from 'react-router-dom'
import { Overview } from './views/Overview'
import { Graph } from './views/Graph'
import { SpecDetail } from './views/SpecDetail'

const tabs = [
  { label: 'Overview', to: '/' },
  { label: 'Graph', to: '/graph' },
]

export function App() {
  return (
    <>
      {/* Grain overlay */}
      <div className="fixed inset-0 grain-overlay z-[100]" />

      {/* Skip link for accessibility */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:px-4 focus:py-2 focus:bg-surface-container-high focus:text-on-surface focus:rounded-md focus:text-sm"
      >
        Skip to dashboard content
      </a>

      {/* Top header */}
      <header className="fixed top-0 left-0 right-0 z-30 bg-surface/60 backdrop-blur-xl border-b border-white/10 h-16 flex items-center justify-between px-8 shadow-2xl shadow-black/50">
        <div className="flex items-center gap-10">
          {/* Brand */}
          <NavLink to="/" className="flex items-center gap-3 shrink-0">
            <h1 className="font-headline font-black text-primary tracking-tighter text-xl">SPECCTL</h1>
            <span className="text-[9px] font-mono text-zinc-600 uppercase tracking-[0.15em] hidden sm:block">
              Governance Dashboard
            </span>
          </NavLink>

          {/* Navigation tabs */}
          <nav className="flex items-center gap-6 font-headline tracking-tight text-sm">
            {tabs.map((tab) => (
              <NavLink
                key={tab.to}
                to={tab.to}
                end={tab.to === '/'}
                className={({ isActive }) =>
                  isActive
                    ? 'text-primary-container border-b-2 border-primary-container pb-2 font-bold transition-colors'
                    : 'text-zinc-400 hover:text-zinc-100 transition-colors'
                }
              >
                {tab.label}
              </NavLink>
            ))}
          </nav>
        </div>

        {/* Right side — specctl info */}
        <div className="flex items-center gap-4">
          <a
            href="https://github.com/aitoroses/specctl"
            target="_blank"
            rel="noopener noreferrer"
            className="text-zinc-500 hover:text-zinc-300 transition-colors flex items-center gap-1.5 text-xs font-mono"
          >
            <span className="material-symbols-outlined text-sm">open_in_new</span>
            Docs
          </a>
        </div>
      </header>

      {/* Main content */}
      <main id="main-content" className="min-h-screen pt-24 px-8 pb-16 max-w-7xl mx-auto">
        <Routes>
          <Route path="/" element={<Overview />} />

          <Route path="/graph" element={<Graph />} />
          <Route path="/specs/:charter/:slug" element={<SpecDetail />} />
        </Routes>
      </main>

      {/* Footer — minimal, real info only */}
      <footer className="fixed bottom-0 left-0 right-0 h-8 border-t border-white/5 bg-surface px-8 flex items-center justify-between z-30">
        <span className="text-[9px] font-mono text-zinc-600 uppercase tracking-wider">
          specctl dashboard
        </span>
        <span className="text-[9px] font-mono text-zinc-600">
          Generated from .specs/
        </span>
      </footer>
    </>
  )
}
