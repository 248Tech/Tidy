<script lang="ts">
  import { Search, X, Power, AppWindow, Layers, ShieldCheck, Info } from '@lucide/svelte'
  import { app } from '../lib/state.svelte'
  import { bytes } from '../lib/format'
  import { stopApp } from '../lib/actions'
  import type { AppGroup } from '../lib/api'
  import AppIcon from '../components/AppIcon.svelte'
  import Sparkline from '../components/Sparkline.svelte'

  let q = $state('')
  let sort = $state<'mem' | 'cpu' | 'name'>('mem')
  let showSystem = $state(false)

  const filtered = $derived.by(() => {
    const needle = q.trim().toLowerCase()
    let gs = app.groups.filter((g) => !needle || g.name.toLowerCase().includes(needle) || g.exe.toLowerCase().includes(needle) || g.publisher.toLowerCase().includes(needle))
    gs = [...gs].sort((a, b) => sort === 'mem' ? b.mem - a.mem : sort === 'cpu' ? b.cpu - a.cpu || b.mem - a.mem : a.name.localeCompare(b.name))
    return gs
  })
  const sections = $derived([
    { id: 'app', title: 'Open apps', hint: 'Apps with a window you can see', icon: AppWindow, items: filtered.filter((g) => g.kind === 'app') },
    { id: 'background', title: 'Running in the background', hint: 'Helpers, updaters and tray apps', icon: Layers, items: filtered.filter((g) => g.kind === 'background') },
    ...(showSystem || q ? [{ id: 'system', title: 'Windows', hint: 'Parts of Windows itself — best left alone', icon: ShieldCheck, items: filtered.filter((g) => g.kind === 'system') }] : []),
  ])
  const total = $derived(app.groups.reduce((a, g) => a + g.mem, 0))
  const maxMem = $derived(Math.max(1, ...app.groups.map((g) => g.mem)))
  const nApps = $derived(app.groups.filter((g) => g.kind === 'app').length)
  const nBg = $derived(app.groups.filter((g) => g.kind === 'background').length)
  const hiddenSys = $derived(app.groups.filter((g) => g.kind === 'system'))

  function trend(g: AppGroup) {
    app.trendTick
    return app.memTrend.get(g.key) ?? []
  }

  function cpuTone(c: number) { return c > 50 ? 'bad' : c > 15 ? 'warn' : '' }
</script>

<div class="page">
  <div class="page-head">
    <div>
      <h1>Apps</h1>
      <p>{nApps} open apps and {nBg} background helpers are using <strong class="num">{bytes(total)}</strong> of memory.</p>
    </div>
    <div class="tools">
      <label class="search">
        <Search size={15} />
        <input class="input" placeholder="Find an app…" bind:value={q} />
        {#if q}<button class="clear" onclick={() => (q = '')} aria-label="Clear"><X size={14} /></button>{/if}
      </label>
      <div class="segmented" role="tablist" aria-label="Sort by">
        <button class:on={sort === 'mem'} onclick={() => (sort = 'mem')}>Memory</button>
        <button class:on={sort === 'cpu'} onclick={() => (sort = 'cpu')}>Processor</button>
        <button class:on={sort === 'name'} onclick={() => (sort = 'name')}>Name</button>
      </div>
    </div>
  </div>

  {#if !app.groupsLoaded}
    <div class="card list">{#each Array(8) as _}<div class="row"><div class="skeleton" style="width:34px;height:34px;border-radius:10px"></div><div class="skeleton" style="height:12px;width:40%"></div></div>{/each}</div>
  {/if}

  {#each sections as s (s.id)}
    {#if s.items.length}
      {@const Icon = s.icon}
      <div class="section-title"><Icon size={16} class="faint" /><h2>{s.title}</h2><span class="muted">{s.hint} · {s.items.length}</span></div>
      <div class="card list">
        <div class="row head">
          <span>Name</span><span>Memory</span><span><span class="long">Processor</span><span class="short">CPU</span></span><span class="trend-col">Last minute</span><span></span>
        </div>
        {#each s.items as g (g.key)}
          <div class="row" role="button" tabindex="0" onclick={() => (app.selectedApp = g.key)} onkeydown={(e) => e.key === 'Enter' && (app.selectedApp = g.key)}>
            <div class="name">
              <AppIcon exe={g.exe} name={g.name} size={34} />
              <div class="nm">
                <strong class="ellipsis">{g.name}</strong>
                <span class="faint ellipsis">{g.publisher || g.exe.split('\\').pop() || 'Windows'}{g.count > 1 ? ` · ${g.count} parts` : ''}</span>
              </div>
            </div>
            <div class="mem">
              <span class="num">{bytes(g.mem)}</span>
              <div class="bar"><span style="width:{(g.mem / maxMem) * 100}%"></span></div>
            </div>
            <div class="cpu num {cpuTone(g.cpu)}">{g.cpu < 0.1 ? '0' : g.cpu.toFixed(g.cpu < 10 ? 1 : 0)}%</div>
            <div class="trend-col"><Sparkline data={trend(g)} width={110} height={26} color={g.kind === 'app' ? 'var(--accent)' : 'var(--accent-2)'} /></div>
            <div class="actions">
              {#if g.protect === 'blocked'}
                <span class="badge" title={g.reason}><ShieldCheck size={12} /> Protected</span>
              {:else}
                {#if g.kind === 'app'}
                  <button class="btn sm" onclick={(e) => { e.stopPropagation(); stopApp(g, false) }}>Close</button>
                {/if}
                <button class="btn sm icon ghost danger-h" title="Force quit" aria-label="Force quit {g.name}" onclick={(e) => { e.stopPropagation(); stopApp(g, true) }}><Power size={15} /></button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {/each}

  {#if app.groupsLoaded && !q}
    <div class="sys-toggle card">
      <Info size={16} class="faint" />
      <p class="muted">{showSystem ? 'Showing' : 'Hiding'} <strong>{hiddenSys.length}</strong> parts of Windows ({bytes(hiddenSys.reduce((a, g) => a + g.mem, 0))}). They keep your PC running.</p>
      <button class="switch" class:on={showSystem} onclick={() => (showSystem = !showSystem)} aria-label="Show Windows parts"></button>
    </div>
  {/if}

  {#if app.groupsLoaded && q && !filtered.length}
    <div class="empty"><Search size={28} /><h2>No apps match “{q}”</h2><p class="muted">Check the spelling, or it may not be running.</p></div>
  {/if}
</div>

<style>
  .tools { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
  .search { position: relative; display: flex; align-items: center; color: var(--faint); }
  .search :global(svg:first-child) { position: absolute; left: 12px; pointer-events: none; }
  .search .input { padding-left: 36px; width: 240px; }
  .clear { position: absolute; right: 6px; border: 0; background: var(--surface-3); border-radius: 6px; width: 22px; height: 22px; display: grid; place-items: center; color: var(--muted); }

  .list { padding: 4px; }
  .row {
    display: grid; grid-template-columns: minmax(0, 2.2fr) minmax(140px, 1.3fr) 80px 130px 120px;
    align-items: center; gap: 16px; padding: 9px 12px; border-radius: 10px; cursor: pointer;
    transition: background .15s;
  }
  .row:not(.head):hover { background: var(--hover); }
  .row:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }
  .row.head { cursor: default; padding-top: 8px; padding-bottom: 6px; font-size: 11.5px; font-weight: 600; color: var(--faint); text-transform: uppercase; letter-spacing: .06em; }
  .name { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .nm { display: flex; flex-direction: column; min-width: 0; }
  .nm strong { font-weight: 600; font-size: 13.5px; }
  .nm span { font-size: 12px; }
  .mem { display: flex; flex-direction: column; gap: 5px; font-size: 13px; font-weight: 550; }
  .mem .bar { height: 4px; }
  .short { display: none; }
  .cpu { font-size: 13px; font-weight: 550; color: var(--muted); }
  .cpu.warn { color: var(--warn); }
  .cpu.bad { color: var(--bad); }
  .actions { display: flex; justify-content: flex-end; gap: 6px; opacity: 0; transition: opacity .15s; }
  .row:hover .actions, .row:focus-within .actions { opacity: 1; }
  .actions .badge { opacity: .8; }
  .danger-h:hover { color: var(--bad); background: var(--bad-soft); }

  .sys-toggle { display: flex; align-items: center; gap: 12px; padding: 14px 16px; margin-top: 24px; }
  .sys-toggle p { flex: 1; font-size: 13px; }
  .empty { text-align: center; padding: 60px 0; color: var(--faint); display: flex; flex-direction: column; align-items: center; gap: 8px; }
  .empty h2 { color: var(--text); }

  @media (max-width: 1000px) {
    .row { grid-template-columns: minmax(160px, 2fr) minmax(110px, 1.2fr) 64px 96px; }
    .trend-col { display: none; }
  }
  @media (max-width: 720px) {
    .search .input { width: 100%; }
    .tools, .search { width: 100%; }
    .row { grid-template-columns: minmax(0, 1fr) 84px 48px; gap: 10px; padding: 9px 8px; }
    .row .trend-col { display: none; }
    .mem .bar { display: none; }
    .row .actions { display: none; }
    .long { display: none; }
    .short { display: inline; }
    .row.head span:last-child { display: none; }
  }
</style>
