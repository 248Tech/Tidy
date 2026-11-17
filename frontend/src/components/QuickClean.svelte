<script lang="ts">
  import { Flame, Globe, FileWarning, Image, Package, Trash2, Sparkles, Check, ArrowRight, RefreshCw } from '@lucide/svelte'
  import { get, post, type CleanItem } from '../lib/api'
  import { bytes, count } from '../lib/format'
  import { confirmDialog, toast } from '../lib/ui.svelte'
  import { app } from '../lib/state.svelte'

  let { onreview }: { onreview: (path: string) => void } = $props()

  const icons: Record<string, any> = { flame: Flame, globe: Globe, 'file-warning': FileWarning, image: Image, package: Package, 'trash-2': Trash2 }
  let items = $state<CleanItem[] | null>(null)
  let picked = $state<Set<string>>(new Set())
  let busy = $state(false)
  let done = $state<{ freed: number } | null>(null)

  async function load() {
    items = null
    items = await get<CleanItem[]>('/api/clean').catch(() => [])
    picked = new Set(items.filter((i) => !i.reviewIn && i.size > 0).map((i) => i.id))
  }
  load()

  const total = $derived(items?.filter((i) => picked.has(i.id)).reduce((a, i) => a + i.size, 0) ?? 0)

  function toggle(id: string) {
    const n = new Set(picked)
    n.has(id) ? n.delete(id) : n.add(id)
    picked = n
  }

  async function run() {
    if (app.viewOnly) return toast({ tone: 'warn', title: 'View-only access', body: 'Changes are turned off for remote viewers.' })
    const chosen = items!.filter((i) => picked.has(i.id))
    const ok = await confirmDialog({
      title: `Clean up ${bytes(total)}?`,
      body: 'These files are safe to remove. Anything currently in use is skipped automatically.',
      items: chosen.map((c) => ({ label: c.title, sub: bytes(c.size) })),
      confirmLabel: 'Clean up', tone: 'primary', icon: 'sparkles',
      note: chosen.some((c) => c.id === 'recycle') ? 'Emptying the Recycle Bin is permanent.' : undefined,
    })
    if (!ok) return
    busy = true
    try {
      const r: any = await post('/api/clean', { ids: chosen.map((c) => c.id) })
      done = { freed: r.freed }
      toast({ tone: 'good', title: `Freed ${bytes(r.freed)}`, body: r.skipped ? `${count(r.skipped)} files were in use and were left alone.` : 'Your PC is a little tidier now ✨' })
      await load()
    } catch (e: any) {
      toast({ tone: 'bad', title: 'Clean-up failed', body: e.message })
    } finally {
      busy = false
    }
  }
</script>

<div class="qc">
  <div class="head">
    <div>
      <h2>Quick clean</h2>
      <p class="muted">Safe clutter Tidy found on your PC. Pick what to clear.</p>
    </div>
    <button class="btn sm ghost" onclick={load} disabled={!items}><RefreshCw size={14} /> Re-check</button>
  </div>

  <div class="grid">
    {#if !items}
      {#each Array(6) as _}<div class="item card"><div class="skeleton" style="height:64px"></div></div>{/each}
    {:else}
      {#each items as it (it.id)}
        {@const Icon = icons[it.icon] ?? Sparkles}
        {@const on = picked.has(it.id)}
        <div class="item card" class:on class:empty={it.size === 0}>
          <span class="ico"><Icon size={18} /></span>
          <div class="txt">
            <strong>{it.title}</strong>
            <p class="muted">{it.desc}</p>
            {#if it.note && it.size > 0}<p class="note">{it.note}</p>{/if}
          </div>
          <div class="side">
            <span class="num size">{it.size ? bytes(it.size) : 'Clean'}</span>
            {#if it.reviewIn}
              <button class="btn sm" disabled={!it.size} onclick={() => onreview(it.reviewIn!)}>Review <ArrowRight size={13} /></button>
            {:else if it.size > 0}
              <button class="check" class:on onclick={() => toggle(it.id)} aria-label="Include {it.title}" aria-pressed={on}>{#if on}<Check size={12} strokeWidth={3} />{/if}</button>
            {:else}
              <span class="ok"><Check size={14} /></span>
            {/if}
          </div>
        </div>
      {/each}
    {/if}
  </div>

  <div class="cta card">
    <div class="sum">
      <span class="faint">Selected</span>
      <strong class="num grad-text">{bytes(total)}</strong>
    </div>
    <button class="btn primary lg" disabled={!total || busy} onclick={run}>
      <Sparkles size={17} /> {busy ? 'Cleaning…' : 'Clean up'}
    </button>
  </div>
</div>

<style>
  .head { display: flex; justify-content: space-between; align-items: flex-end; margin-bottom: 14px; }
  .grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
  .item { display: flex; gap: 14px; padding: 16px; align-items: flex-start; transition: border-color .2s, background .2s; }
  .item.on { border-color: color-mix(in srgb, var(--accent) 45%, transparent); background: color-mix(in srgb, var(--accent) 5%, var(--surface)); }
  .item.empty { opacity: .6; }
  .ico { flex: none; width: 38px; height: 38px; border-radius: 12px; display: grid; place-items: center; background: var(--accent-soft); color: var(--accent); }
  .txt { flex: 1; min-width: 0; }
  .txt strong { font-weight: 600; }
  .txt p { font-size: 12.5px; margin-top: 2px; }
  .txt .note { color: var(--warn); }
  .side { display: flex; flex-direction: column; align-items: flex-end; gap: 10px; }
  .size { font-weight: 650; font-size: 14px; white-space: nowrap; }
  .ok { color: var(--good); }
  .cta { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; margin-top: 16px; }
  .sum { display: flex; flex-direction: column; }
  .sum strong { font-size: 26px; font-weight: 700; letter-spacing: -.02em; }
  @media (max-width: 860px) { .grid { grid-template-columns: 1fr; } }
</style>
