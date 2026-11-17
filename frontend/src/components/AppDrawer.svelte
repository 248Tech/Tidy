<script lang="ts">
  import { fade, fly } from 'svelte/transition'
  import { X, Power, ShieldCheck, TriangleAlert, FolderOpen, Clock, Cpu, MemoryStick, Layers } from '@lucide/svelte'
  import { app } from '../lib/state.svelte'
  import { get, type AppGroup } from '../lib/api'
  import { bytes, ago } from '../lib/format'
  import { stopApp, reveal } from '../lib/actions'
  import AppIcon from './AppIcon.svelte'
  import Sparkline from './Sparkline.svelte'

  let detail = $state<AppGroup | null>(null)
  let gone = $state(false)

  const live = $derived(app.groups.find((g) => g.key === app.selectedApp) ?? null)
  const g = $derived(live ?? detail)

  $effect(() => {
    const key = app.selectedApp
    detail = null
    gone = false
    if (!key) return
    let alive = true
    const load = async () => {
      try {
        const d = await get<AppGroup>(`/api/procs/detail?key=${encodeURIComponent(key)}`)
        if (alive) detail = d
      } catch {
        if (alive) gone = true
      }
    }
    load()
    const t = setInterval(load, 2000)
    return () => { alive = false; clearInterval(t) }
  })

  const trend = $derived.by(() => { app.trendTick; return app.selectedApp ? app.memTrend.get(app.selectedApp) ?? [] : [] })
  const started = $derived(detail?.procs?.length ? Math.min(...detail.procs.map((p) => p.started).filter(Boolean)) : 0)
  const close = () => (app.selectedApp = null)
  const kindLabel = { app: 'Open app', background: 'Background helper', system: 'Part of Windows' }

  async function act(force: boolean) {
    if (g && (await stopApp(g, force)) && force) setTimeout(close, 400)
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && app.selectedApp && close()} />

{#if app.selectedApp}
  <div class="scrim" transition:fade={{ duration: 180 }} onclick={close} role="presentation"></div>
  <aside class="drawer" transition:fly={{ x: 440, duration: 320, opacity: 1 }} aria-label="App details">
    <button class="btn icon ghost x" onclick={close} aria-label="Close panel"><X size={18} /></button>
    {#if g}
      <header>
        <AppIcon exe={g.exe} name={g.name} size={60} />
        <div class="title">
          <h2>{g.name}</h2>
          <p class="muted">{g.publisher || 'Unknown publisher'}</p>
          <span class="badge {g.kind === 'app' ? 'accent' : ''}">{kindLabel[g.kind]}</span>
        </div>
      </header>

      {#if gone}
        <div class="banner good"><ShieldCheck size={16} /><span>This app isn't running anymore.</span></div>
      {:else if g.protect === 'blocked'}
        <div class="banner"><ShieldCheck size={16} /><span><strong>Protected.</strong> {g.reason}</span></div>
      {:else if g.protect === 'caution'}
        <div class="banner warn"><TriangleAlert size={16} /><span><strong>Careful.</strong> {g.reason}</span></div>
      {/if}

      <div class="stats">
        <div class="stat"><MemoryStick size={15} /><span class="faint">Memory</span><strong class="num">{bytes(g.mem)}</strong></div>
        <div class="stat"><Cpu size={15} /><span class="faint">Processor</span><strong class="num">{g.cpu.toFixed(1)}%</strong></div>
        <div class="stat"><Layers size={15} /><span class="faint">Parts</span><strong class="num">{g.count}</strong></div>
        <div class="stat"><Clock size={15} /><span class="faint">Started</span><strong>{started ? ago(started) : '–'}</strong></div>
      </div>

      <div class="trend card">
        <span class="faint">Memory over the last minute</span>
        <Sparkline data={trend} width={360} height={56} />
      </div>

      {#if g.exe}
        <div class="path">
          <span class="mono ellipsis" title={g.exe}>{g.exe}</span>
          {#if app.desktop}<button class="btn sm ghost" onclick={() => reveal(g.exe)}><FolderOpen size={14} /> Show</button>{/if}
        </div>
      {/if}

      {#if detail?.procs?.length}
        <h3 class="sub">Parts of this app <span class="faint">({detail.procs.length})</span></h3>
        <div class="procs">
          {#each detail.procs.slice(0, 60) as p (p.pid)}
            <div class="proc">
              <span class="ellipsis">{p.name}</span>
              <span class="faint mono">#{p.pid}</span>
              <span class="num">{bytes(p.mem)}</span>
              <span class="num faint">{p.cpu.toFixed(1)}%</span>
              {#if g.protect !== 'blocked' && g.count > 1}
                <button class="btn sm icon ghost danger-h" title="Stop just this part" aria-label="Stop process {p.pid}"
                  onclick={() => stopApp({ key: `pid:${p.pid}`, name: `${p.name} (#${p.pid})`, count: 1, mem: p.mem, protect: g.protect, reason: g.reason }, true)}><Power size={13} /></button>
              {/if}
            </div>
          {/each}
        </div>
      {/if}

      {#if g.protect !== 'blocked' && !gone}
        <footer>
          {#if g.kind === 'app'}
            <button class="btn primary lg" onclick={() => act(false)}>Close app</button>
          {/if}
          <button class="btn lg {g.kind === 'app' ? '' : 'danger'}" onclick={() => act(true)}><Power size={16} /> Force quit</button>
          <p class="faint hint">{g.kind === 'app' ? '“Close” lets it save your work. “Force quit” stops it right away.' : 'Background helpers usually start again on their own when needed.'}</p>
        </footer>
      {/if}
    {/if}
  </aside>
{/if}

<style>
  .scrim { position: fixed; inset: 0; z-index: 60; background: rgba(5, 6, 10, .35); backdrop-filter: blur(2px); }
  .drawer {
    position: fixed; z-index: 61; top: 0; right: 0; bottom: 0; width: min(440px, 100vw);
    overflow-y: auto; padding: 28px 26px 24px;
    background: var(--surface); border-left: 1px solid var(--border); box-shadow: var(--shadow-pop);
    display: flex; flex-direction: column; gap: 16px;
  }
  .x { position: absolute; top: 14px; right: 14px; }
  header { display: flex; gap: 16px; align-items: center; padding-right: 30px; }
  .title { min-width: 0; display: flex; flex-direction: column; gap: 3px; align-items: flex-start; }
  .title h2 { font-size: 19px; line-height: 1.2; }
  .title p { font-size: 13px; }
  .banner { display: flex; gap: 10px; align-items: flex-start; padding: 12px 14px; border-radius: 12px; background: var(--accent-soft); font-size: 13px; }
  .banner :global(svg) { flex: none; margin-top: 2px; color: var(--accent); }
  .banner.warn { background: var(--warn-soft); }
  .banner.warn :global(svg) { color: var(--warn); }
  .banner.good { background: var(--good-soft); }
  .stats { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  .stat { display: grid; grid-template-columns: auto 1fr; grid-template-rows: auto auto; column-gap: 8px; padding: 12px; border-radius: 12px; background: var(--surface-2); color: var(--muted); align-items: center; }
  .stat span { font-size: 12px; }
  .stat strong { grid-column: 1 / -1; color: var(--text); font-size: 18px; font-weight: 650; margin-top: 4px; }
  .trend { padding: 12px 14px; box-shadow: none; background: var(--surface-2); display: flex; flex-direction: column; gap: 8px; font-size: 12px; }
  .trend :global(svg) { width: 100%; }
  .path { display: flex; align-items: center; gap: 8px; padding: 8px 8px 8px 12px; border-radius: 10px; border: 1px dashed var(--border-strong); color: var(--muted); }
  .path .mono { flex: 1; min-width: 0; user-select: text; }
  .sub { font-size: 13px; margin-top: 4px; }
  .procs { display: flex; flex-direction: column; border-radius: 12px; background: var(--surface-2); padding: 4px; max-height: 260px; overflow-y: auto; }
  .proc { display: grid; grid-template-columns: 1fr auto 72px 48px 30px; gap: 10px; align-items: center; padding: 5px 8px; font-size: 12.5px; border-radius: 8px; min-height: 36px; }
  .proc:hover { background: var(--hover); }
  .proc .num { text-align: right; }
  .danger-h:hover { color: var(--bad); background: var(--bad-soft); }
  footer { margin-top: auto; padding-top: 12px; display: flex; flex-wrap: wrap; gap: 10px; }
  footer .btn { flex: 1; }
  .hint { width: 100%; font-size: 12px; text-align: center; }
</style>
