import { get, onAuthRequired, type AppGroup, type Info, type Metrics } from './api'

export type Route = 'home' | 'apps' | 'storage' | 'activity' | 'settings'
const HISTORY = 120

export interface Sample { t: number; cpu: number; mem: number; diskR: number; diskW: number; up: number; down: number }

class AppState {
  route = $state<Route>('home')
  info = $state<Info | null>(null)
  needLogin = $state(false)
  offline = $state(false)
  metrics = $state<Metrics | null>(null)
  history = $state<Sample[]>([])
  groups = $state<AppGroup[]>([])
  groupsLoaded = $state(false)
  memTrend = new Map<string, number[]>()
  trendTick = $state(0)
  selectedApp = $state<string | null>(null)
  paletteOpen = $state(false)
  theme = $state<'system' | 'light' | 'dark'>('system')
  storageIntent = $state<{ drive?: string; tab?: string } | null>(null)

  get desktop() { return !!this.info?.local && typeof (window as any).runtime !== 'undefined' }
  get viewOnly() { return !!this.info?.viewOnly }

  go(r: Route) {
    this.route = r
    this.selectedApp = null
    if (location.hash !== `#/${r}`) history.replaceState(null, '', `#/${r}`)
    document.querySelector('main')?.scrollTo({ top: 0 })
  }
}

export const app = new AppState()

const ROUTES: Route[] = ['home', 'apps', 'storage', 'activity', 'settings']
function routeFromHash() {
  const r = location.hash.replace(/^#\/?/, '') as Route
  if (ROUTES.includes(r) && r !== app.route) {
    app.route = r
    app.selectedApp = null
  }
}
routeFromHash()
window.addEventListener('hashchange', routeFromHash)

// ───────── theme ─────────
const media = window.matchMedia('(prefers-color-scheme: light)')
export function applyTheme() {
  const light = app.theme === 'light' || (app.theme === 'system' && media.matches)
  document.documentElement.dataset.theme = light ? 'light' : 'dark'
  document.querySelector('meta[name=theme-color]')?.setAttribute('content', light ? '#f4f5f9' : '#0b0c11')
}
media.addEventListener('change', applyTheme)

// ───────── polling ─────────
let metricsTimer: number | undefined
let procsTimer: number | undefined

async function pollMetrics() {
  try {
    const m = await get<Metrics>('/api/metrics')
    app.offline = false
    app.metrics = m
    const s: Sample = {
      t: m.time, cpu: m.cpu, mem: (m.memUsed / m.memTotal) * 100,
      diskR: m.diskRead, diskW: m.diskWrite, up: m.netUp, down: m.netDown,
    }
    const h = app.history.length >= HISTORY ? app.history.slice(1) : app.history.slice()
    h.push(s)
    app.history = h
  } catch (e: any) {
    if (e?.status !== 401) app.offline = true
  }
}

async function pollProcs() {
  try {
    const gs = await get<AppGroup[]>('/api/procs')
    const seen = new Set<string>()
    for (const g of gs) {
      seen.add(g.key)
      // new array each time so components see a changed reference
      app.memTrend.set(g.key, [...(app.memTrend.get(g.key) ?? []).slice(-39), g.mem])
    }
    for (const k of app.memTrend.keys()) if (!seen.has(k)) app.memTrend.delete(k)
    app.groups = gs
    app.groupsLoaded = true
    app.trendTick++
  } catch { /* handled by metrics poll */ }
}

function wantsProcs() {
  return app.route === 'home' || app.route === 'apps' || app.selectedApp !== null || app.paletteOpen
}

function schedule() {
  clearTimeout(metricsTimer)
  clearTimeout(procsTimer)
  if (document.hidden || app.needLogin) return
  const tickM = async () => { await pollMetrics(); metricsTimer = window.setTimeout(tickM, 1000) }
  const tickP = async () => {
    if (wantsProcs()) await pollProcs()
    procsTimer = window.setTimeout(tickP, 2000)
  }
  tickM()
  tickP()
}

let booted = false
export async function boot() {
  if (!booted) {
    booted = true
    onAuthRequired(() => { app.needLogin = true; schedule() })
    document.addEventListener('visibilitychange', schedule)
  }
  try {
    app.info = await get<Info>('/api/info')
    app.theme = (app.info.theme as any) || 'system'
    app.needLogin = false
  } catch (e: any) {
    if (e?.status === 401) app.needLogin = true
    else app.offline = true
  }
  applyTheme()
  schedule()
}

export function refreshProcsNow() { return pollProcs() }

export function afterLogin() {
  boot()
}
