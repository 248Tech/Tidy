import { post, ApiError, type AppGroup, type DeleteItem } from './api'
import { bytes } from './format'
import { confirmDialog, toast } from './ui.svelte'
import { app, refreshProcsNow } from './state.svelte'

function viewOnlyBlocked() {
  if (app.viewOnly) {
    toast({ tone: 'warn', title: 'View-only access', body: 'Changes are turned off for remote viewers.' })
    return true
  }
  return false
}

/** Close an app nicely, or force-quit it. Handles every protection level with a friendly explanation. */
export async function stopApp(g: Pick<AppGroup, 'key' | 'name' | 'count' | 'mem' | 'protect' | 'reason'>, force: boolean): Promise<boolean> {
  if (viewOnlyBlocked()) return false
  if (g.protect === 'blocked') {
    toast({ tone: 'info', title: `${g.name} is protected`, body: g.reason })
    return false
  }
  const many = g.count > 1 ? ` (all ${g.count} parts)` : ''
  const ok = await confirmDialog(force
    ? {
        title: `Force quit ${g.name}?`,
        body: `This stops it immediately${many}. Anything you haven't saved in it will be lost.`,
        confirmLabel: 'Force quit', tone: 'danger', icon: 'power',
        note: g.protect === 'caution' ? `Heads up: ${g.reason}` : undefined,
      }
    : {
        title: `Close ${g.name}?`,
        body: `Tidy will ask it to close, just like clicking its ✕ button. It may ask you to save your work first.`,
        confirmLabel: 'Close app', tone: 'primary', icon: 'power',
        note: g.protect === 'caution' ? `Heads up: ${g.reason}` : undefined,
      })
  if (!ok) return false
  try {
    const res: any = await post('/api/procs/stop', { key: g.key, force, confirmed: true })
    const r = res.result
    if (force) {
      if (r.failed && !r.killed) {
        toast({ tone: 'bad', title: `Couldn't stop ${g.name}`, body: r.denied ? 'Windows needs admin rights for this one. Try “Restart as admin” in Settings.' : 'It may have already closed.' })
        return false
      }
      toast({ tone: 'good', title: `${g.name} stopped`, body: res.freed ? `About ${bytes(res.freed)} of memory freed up.` : undefined })
    } else {
      toast({ tone: 'good', title: `Asked ${g.name} to close`, body: 'If it has unsaved work it may ask you first. Still stuck? Use Force quit.' })
    }
    setTimeout(refreshProcsNow, 700)
    return true
  } catch (e) {
    const err = e as ApiError
    toast({ tone: err.code === 'blocked' ? 'info' : 'bad', title: `Couldn't ${force ? 'stop' : 'close'} ${g.name}`, body: err.message })
    return false
  }
}

export interface Deletable { path: string; name: string; size: number }

/** Move files to the Recycle Bin (with Undo) or delete permanently. Returns paths that were removed. */
export async function deleteFiles(items: Deletable[], permanent = false): Promise<string[]> {
  if (viewOnlyBlocked() || !items.length) return []
  const total = items.reduce((a, b) => a + b.size, 0)
  const one = items.length === 1
  const ok = await confirmDialog({
    title: permanent
      ? `Delete ${one ? `“${items[0].name}”` : `${items.length} items`} forever?`
      : `Move ${one ? `“${items[0].name}”` : `${items.length} items`} to the Recycle Bin?`,
    body: permanent
      ? `This frees ${bytes(total)} right away, but it can't be undone.`
      : `This frees up ${bytes(total)}. Changed your mind? You can undo it, or restore from the Recycle Bin later.`,
    items: items.map((i) => ({ label: i.path || i.name, sub: bytes(i.size) })),
    confirmLabel: permanent ? 'Delete forever' : 'Move to Recycle Bin',
    tone: 'danger', icon: 'trash',
  })
  if (!ok) return []
  let res: { items: DeleteItem[]; freed: number }
  try {
    res = await post('/api/files/delete', { paths: items.map((i) => i.path), permanent })
  } catch (e: any) {
    toast({ tone: 'bad', title: 'Delete failed', body: e.message })
    return []
  }
  const done = res.items.filter((i) => i.ok)
  const tooBig = res.items.filter((i) => i.tooBig)
  const failed = res.items.filter((i) => !i.ok && !i.tooBig)

  if (done.length) {
    toast({
      tone: 'good',
      title: permanent ? `Deleted ${done.length} item${done.length > 1 ? 's' : ''}` : `Moved ${done.length} item${done.length > 1 ? 's' : ''} to the Recycle Bin`,
      body: `${bytes(res.freed)} freed`,
      action: permanent ? undefined : { label: 'Undo', run: () => restore(done.map((d) => d.path)) },
    })
  }
  if (failed.length) {
    toast({ tone: 'bad', title: `${failed.length} item${failed.length > 1 ? 's' : ''} couldn't be removed`, body: failed[0].error })
  }
  let removed = done.map((d) => d.path)
  if (tooBig.length) {
    const again = await confirmDialog({
      title: 'Too big for the Recycle Bin',
      body: `${tooBig.length === 1 ? 'This item is' : 'These items are'} larger than the Recycle Bin can hold, so ${tooBig.length === 1 ? 'it' : 'they'} can only be deleted forever.`,
      items: tooBig.map((i) => ({ label: i.path, sub: bytes(i.size) })),
      confirmLabel: 'Delete forever', tone: 'danger', icon: 'trash',
    })
    if (again) {
      const r2: { items: DeleteItem[]; freed: number } = await post('/api/files/delete', { paths: tooBig.map((i) => i.path), permanent: true })
      const ok2 = r2.items.filter((i) => i.ok)
      if (ok2.length) toast({ tone: 'good', title: `Deleted ${ok2.length} item${ok2.length > 1 ? 's' : ''} forever`, body: `${bytes(r2.freed)} freed` })
      removed = removed.concat(ok2.map((d) => d.path))
    }
  }
  return removed
}

export const restoreListeners = new Set<() => void>()

async function restore(paths: string[]) {
  try {
    const res: { items: DeleteItem[] } = await post('/api/files/restore', { paths })
    const ok = res.items.filter((i) => i.ok).length
    if (ok) toast({ tone: 'good', title: `Restored ${ok} item${ok > 1 ? 's' : ''}`, body: 'Everything is back where it was.' })
    const bad = res.items.find((i) => !i.ok)
    if (bad) toast({ tone: 'bad', title: "Some items couldn't be restored", body: `${bad.error} You can still restore them from the Recycle Bin.` })
    restoreListeners.forEach((f) => f())
  } catch (e: any) {
    toast({ tone: 'bad', title: 'Undo failed', body: e.message })
  }
}

export async function reveal(path: string) {
  try { await post('/api/files/reveal', { path }) } catch (e: any) { toast({ tone: 'bad', title: "Couldn't open File Explorer", body: e.message }) }
}
