// Toasts and a promise-based confirm dialog, shared app-wide.

export interface Toast {
  id: number
  tone: 'good' | 'bad' | 'info' | 'warn'
  title: string
  body?: string
  action?: { label: string; run: () => void | Promise<void> }
  ttl: number
}

class UI {
  toasts = $state<Toast[]>([])
  confirm = $state<ConfirmReq | null>(null)
}
export const ui = new UI()

let nextId = 1
export function toast(t: Omit<Toast, 'id' | 'ttl'> & { ttl?: number }) {
  const id = nextId++
  const item: Toast = { ttl: t.action ? 9000 : 5000, ...t, id }
  ui.toasts = [...ui.toasts, item].slice(-4)
  setTimeout(() => dismiss(id), item.ttl)
  return id
}
export function dismiss(id: number) {
  ui.toasts = ui.toasts.filter((t) => t.id !== id)
}

export interface ConfirmReq {
  title: string
  body: string
  items?: { label: string; sub?: string }[]
  confirmLabel: string
  tone?: 'danger' | 'primary'
  icon?: 'trash' | 'power' | 'shield' | 'sparkles' | 'alert'
  note?: string
  resolve: (ok: boolean) => void
}

export function confirmDialog(req: Omit<ConfirmReq, 'resolve'>): Promise<boolean> {
  return new Promise((resolve) => {
    ui.confirm = { ...req, resolve: (ok) => { ui.confirm = null; resolve(ok) } }
  })
}
