<script lang="ts">
  import { House, LayoutGrid, HardDrive, Activity, Settings, Search, ShieldCheck, Eye } from '@lucide/svelte'
  import { app, type Route } from '../lib/state.svelte'
  import { bytes } from '../lib/format'
  import Logo from './Logo.svelte'

  const items: { id: Route; label: string; hint: string; icon: any }[] = [
    { id: 'home', label: 'Home', hint: 'How your PC is doing', icon: House },
    { id: 'apps', label: 'Apps', hint: "What's running", icon: LayoutGrid },
    { id: 'storage', label: 'Storage', hint: "What's taking space", icon: HardDrive },
    { id: 'activity', label: 'Activity', hint: 'Live graphs', icon: Activity },
  ]

  const sys = $derived(app.metrics?.drives?.find((d) => d.mount.toUpperCase().startsWith('C')) ?? app.metrics?.drives?.[0])
  const memPct = $derived(app.metrics ? (app.metrics.memUsed / app.metrics.memTotal) * 100 : 0)
</script>

<nav class="sidebar">
  {#if !app.desktop}
    <div class="brand"><Logo size={26} /><div><strong>Tidy</strong><small>{app.info?.system.hostname ?? ''}</small></div></div>
  {/if}

  <div class="links">
    {#each items as it, i}
      {@const Icon = it.icon}
      <button class="link" class:on={app.route === it.id} onclick={() => app.go(it.id)} title={`${it.hint} (Ctrl+${i + 1})`}>
        <span class="ico"><Icon size={18} strokeWidth={app.route === it.id ? 2.2 : 1.8} /></span>
        <span class="label">{it.label}</span>
      </button>
    {/each}
  </div>

  <div class="spacer"></div>

  {#if app.metrics}
    <div class="mini card">
      <div class="row"><span class="muted">Memory</span><span class="num">{memPct.toFixed(0)}%</span></div>
      <div class="bar"><span style="width:{memPct}%"></span></div>
      {#if sys}
        {@const used = ((sys.total - sys.free) / sys.total) * 100}
        <div class="row" style="margin-top:10px"><span class="muted">Drive {sys.mount.replace('\\', '')}</span><span class="num">{bytes(sys.free)} free</span></div>
        <div class="bar"><span style="width:{used}%"></span></div>
      {/if}
    </div>
  {/if}

  {#if app.viewOnly}
    <div class="status"><Eye size={14} /> View-only access</div>
  {:else if app.info?.system.isAdmin && app.info?.local}
    <div class="status good"><ShieldCheck size={14} /> Running as admin</div>
  {/if}

  {#if !app.desktop}
    <button class="link" onclick={() => (app.paletteOpen = true)}>
      <span class="ico"><Search size={18} /></span><span class="label">Search</span>
    </button>
  {/if}
  <button class="link" class:on={app.route === 'settings'} onclick={() => app.go('settings')}>
    <span class="ico"><Settings size={18} /></span><span class="label">Settings</span>
  </button>
</nav>

<style>
  .sidebar {
    width: 228px; flex: none; display: flex; flex-direction: column; gap: 4px;
    padding: 16px 12px;
    background: var(--sidebar); backdrop-filter: blur(24px);
    border-right: 1px solid var(--border);
  }
  .brand { display: flex; align-items: center; gap: 10px; padding: 4px 10px 16px; }
  .brand strong { display: block; font-size: 15px; letter-spacing: -.01em; }
  .brand small { display: block; color: var(--faint); font-size: 11.5px; margin-top: -2px; }
  .links { display: flex; flex-direction: column; gap: 3px; }
  .link {
    position: relative; display: flex; align-items: center; gap: 12px;
    height: 40px; padding: 0 12px; border: 0; border-radius: 10px;
    background: transparent; color: var(--muted); font-weight: 550; font-size: 13.5px; text-align: left;
    transition: background .15s, color .15s;
  }
  .link:hover { background: var(--hover); color: var(--text); }
  .link.on { background: var(--accent-soft); color: var(--text); }
  .link.on::before {
    content: ''; position: absolute; left: -12px; top: 10px; bottom: 10px; width: 3px;
    border-radius: 0 3px 3px 0; background: var(--accent-grad);
  }
  .link.on .ico { color: var(--accent); }
  .ico { display: grid; place-items: center; transition: transform .25s var(--spring); }
  .link:hover .ico { transform: scale(1.08); }
  .spacer { flex: 1; }
  .mini { padding: 12px; margin: 0 2px 8px; font-size: 12px; box-shadow: none; background: var(--surface-2); }
  .mini .row { display: flex; justify-content: space-between; margin-bottom: 6px; }
  .mini .bar { height: 4px; }
  .status { display: flex; align-items: center; gap: 7px; font-size: 12px; color: var(--muted); padding: 4px 12px 8px; }
  .status.good { color: var(--good); }

  @media (max-width: 720px) {
    .sidebar {
      width: 100%; flex-direction: row; padding: 6px 8px calc(6px + env(safe-area-inset-bottom));
      border-right: 0; border-top: 1px solid var(--border); justify-content: space-around;
    }
    .brand, .spacer, .mini, .status { display: none; }
    .links { flex-direction: row; flex: 1; justify-content: space-around; }
    .link { flex-direction: column; gap: 2px; height: 52px; padding: 0 10px; font-size: 10.5px; }
    .link.on::before { display: none; }
  }
</style>
