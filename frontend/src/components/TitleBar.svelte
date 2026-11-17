<script lang="ts">
  import { Minus, Square, X, Search } from '@lucide/svelte'
  import { app } from '../lib/state.svelte'
  import Logo from './Logo.svelte'

  const rt = () => (window as any).runtime
</script>

<header class="titlebar" style="--wails-draggable: drag" ondblclick={() => rt()?.WindowToggleMaximise()}>
  <div class="brand">
    <Logo size={18} />
    <span>Tidy</span>
  </div>

  <button class="search" style="--wails-draggable: no-drag" onclick={() => (app.paletteOpen = true)}>
    <Search size={14} />
    <span>Search apps, files, actions…</span>
    <kbd>Ctrl K</kbd>
  </button>

  <div class="controls" style="--wails-draggable: no-drag">
    <button aria-label="Minimize" onclick={() => rt()?.WindowMinimise()}><Minus size={15} /></button>
    <button aria-label="Maximize" onclick={() => rt()?.WindowToggleMaximise()}><Square size={12} /></button>
    <button aria-label="Close" class="close" onclick={() => rt()?.Quit()}><X size={16} /></button>
  </div>
</header>

<style>
  .titlebar {
    height: 44px; flex: none;
    display: grid; grid-template-columns: 1fr auto 1fr; align-items: center;
    padding-left: 18px;
    border-bottom: 1px solid var(--border);
    background: var(--sidebar);
    backdrop-filter: blur(20px);
  }
  .brand { display: flex; align-items: center; gap: 9px; font-weight: 650; font-size: 13px; letter-spacing: -.01em; }
  .search {
    display: flex; align-items: center; gap: 10px;
    width: 380px; height: 30px; padding: 0 6px 0 12px;
    border-radius: 9px; border: 1px solid var(--border);
    background: var(--surface-2); color: var(--faint); font-size: 12.5px;
    transition: border-color .15s, background .15s;
  }
  .search:hover { border-color: var(--border-strong); color: var(--muted); }
  .search span { flex: 1; text-align: left; }
  .controls { justify-self: end; display: flex; height: 100%; }
  .controls button {
    width: 46px; height: 100%; border: 0; background: transparent; color: var(--muted);
    display: grid; place-items: center; transition: background .12s, color .12s;
  }
  .controls button:hover { background: var(--hover); color: var(--text); }
  .controls .close:hover { background: #e81123; color: #fff; }
  @media (max-width: 1000px) { .search { width: 260px; } }
</style>
