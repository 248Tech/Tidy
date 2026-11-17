// One tiny client for both modes: inside the desktop window the API is served by
// the app itself; in a remote browser it's the same paths over HTTPS + a session cookie.

export class ApiError extends Error {
  status: number
  code?: string
  data: any
  constructor(status: number, data: any) {
    super(data?.error || `Request failed (${status})`)
    this.status = status
    this.code = data?.code
    this.data = data
  }
}

type Listener = () => void
const authListeners = new Set<Listener>()
export const onAuthRequired = (fn: Listener) => (authListeners.add(fn), () => authListeners.delete(fn))

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json', 'X-Tidy': '1' } : { 'X-Tidy': '1' },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: 'same-origin',
  })
  let data: any = null
  const text = await res.text()
  try { data = text ? JSON.parse(text) : null } catch { data = { error: text } }
  if (!res.ok) {
    if (res.status === 401 && data?.code === 'login') authListeners.forEach((f) => f())
    throw new ApiError(res.status, data)
  }
  return data as T
}

export const get = <T>(path: string) => request<T>('GET', path)
export const post = <T>(path: string, body: unknown = {}) => request<T>('POST', path, body)

export const iconUrl = (exe: string) => (exe ? `/api/icon?path=${encodeURIComponent(exe)}` : '')

// ───────── Types mirrored from Go ─────────
export interface Drive { mount: string; label: string; kind: string; total: number; free: number }
export interface Metrics {
  time: number; cpu: number; perCore: number[]; memUsed: number; memTotal: number
  commitUsed: number; commitMax: number; drives: Drive[]
  netUp: number; netDown: number; diskRead: number; diskWrite: number; uptime: number; procs: number
}
export interface SystemInfo { hostname: string; cpuModel: string; cores: number; memTotal: number; isAdmin: boolean; version: string }
export interface Info { system: SystemInfo; local: boolean; viewOnly: boolean; headless: boolean; theme: string }
export interface Proc { pid: number; ppid: number; name: string; exe: string; cpu: number; mem: number; threads: number; started: number; session: number }
export interface AppGroup {
  key: string; name: string; exe: string; publisher: string; kind: 'app' | 'background' | 'system'
  count: number; cpu: number; mem: number; protect: '' | 'caution' | 'blocked'; reason: string; pids: number[]; procs?: Proc[]
}
export interface ScanStatus {
  running: boolean; done: boolean; root: string; files: number; dirs: number; bytes: number
  current: string; elapsed: number; error?: string; denied: number; expected: number
}
export interface TreeItem { name: string; path: string; size: number; isDir: boolean; files?: number; category: string; modified?: number; other?: boolean }
export interface TreeView { path: string; name: string; size: number; files: number; dirs: number; crumbs: TreeItem[]; items: TreeItem[] }
export interface FileEntry { path: string; name: string; size: number; modified: number; category: string }
export interface CategoryTotal { category: string; size: number; count: number }
export interface CleanItem { id: string; title: string; desc: string; icon: string; size: number; count: number; note?: string; reviewIn?: string }
export interface DeleteItem { path: string; ok: boolean; size: number; error?: string; tooBig?: boolean }
export interface Settings { theme: string; remoteEnabled: boolean; remotePort: number; remoteViewOnly: boolean; showSystem: boolean }
export interface RemoteStatus { running: boolean; port: number; urls: string[]; pin?: string; viewOnly: boolean; error?: string; sessions: number }
