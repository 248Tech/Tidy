<script lang="ts">
  import { fade, scale } from 'svelte/transition'
  import { Search, House, LayoutGrid, HardDrive, Activity, Settings, Sparkles, CornerDownLeft, Sun, Moon } from '@lucide/svelte'
  import { app, applyTheme } from '../lib/state.svelte'
  import { bytes } from '../lib/format'
  import AppIcon from './AppIcon.svelte'

  let q = $state('')
  let idx = $state(0)
  let input = $state<HTMLInputElement>()

  type Cmd = { id: string; label: string; hint?: string; icon?: any; exe?: string; run: () => void; group: string }

  const commands = $derived.by((): Cmd[] => {
    const base: Cmd[] = [
      { id: 'home', group: 'Go to', label: 'Home', hint: 'Health overview', icon: House, run: () => app.go('home') },
      { id: 'apps', group: 'Go to', label: 'Apps', hint: "What's running", icon: LayoutGrid, run: () => app.go('apps') },
      { id: 'storage', group: 'Go to', label: 'Storage', hint: "What's taking space", icon: HardDrive, run: () => app.go('storage') },
      { id: 'activity', group: 'Go to', label: 'Activity', hint: 'Live graphs', icon: Activity, run: () => app.go('activity') },
      { id: 'settings', group: 'Go to', label: 'Settings', icon: Settings, run: () => app.go('settings') },
      { id: 'clean', group: 'Actions', label: 'Quick clean', hint: 'Clear temp files & caches', icon: Sparkles, run: () => { app.storageIntent = { tab: 'clean' }; app.go('storage') } },
      { id: 'theme', group: 'Actions', label: 'Switch light / dark', icon: document.documentElement.dataset.theme === 'light' ? Moon : Sun, run: () => { app.theme = document.documentElement.dataset.theme === 'light' ? 'dark' : 'light'; applyTheme() } },
      ...(app.metrics?.drives ?? []).map((d) => ({ id: 'scan' + d.mount, group: 'Actions', label: `Scan drive ${d.mount.replace('\\', '')}`, hint: `${bytes(d.free)} free`, icon: HardDrive, run: () => { app.storageIntent = { drive: d.mount }; app.go('storage') } })),
    ]
    const apps: Cmd[] = app.groups.filter((g) => g.kind !== 'system' || q).map((g) => ({
      id: 'app' + g.key, group: 'Running apps', label: g.name, hint: bytes(g.mem), exe: g.exe, run: () => (app.selectedApp = g.key),
    }))
    const all = [...base, ...apps]
    const n = q.trim().toLowerCase()
    if (!n) return all.slice(0, 14)
    const ranked = all
      .map((c) => ({ c, s: score(c.label.toLowerCase(), n) }))
      .filter((x) => x.s > 0)
      .sort((a, b) => b.s - a.s)
      .slice(0, 14)
      .map((x) => x.c)
    // keep each group together, ordered by its best match
    const groups = [...new Set(ranked.map((c) => c.group))]
    return groups.flatMap((g) => ranked.filter((c) => c.group === g))
  })

  function score(t: string, n: string) {
    if (t.startsWith(n)) return 3
    if (t.includes(' ' + n)) return 2
    if (t.includes(n)) return 1
    let i = 0
    for (const ch of t) if (ch === n[i]) i++
    return i === n.length && n.length >= 4 ? 0.5 : 0
  }

  $effect(() => {
    if (app.paletteOpen) {
      q = ''
      idx = 0
      setTimeout(() => input?.focus(), 30)
    }
  })
  $effect(() => { q; idx = 0 })

  function run(c: Cmd) {
    app.paletteOpen = false
    c.run()
  }

  function key(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { idx = Math.min(commands.length - 1, idx + 1); e.preventDefault() }
    else if (e.key === 'ArrowUp') { idx = Math.max(0, idx - 1); e.preventDefault() }
    else if (e.key === 'Enter' && commands[idx]) run(commands[idx])
    else if (e.key === 'Escape') app.paletteOpen = false
  }
</script>

{#if app.paletteOpen}
  <div class="scrim" transition:fade={{ duration: 120 }} onclick={() => (app.paletteOpen = false)} role="presentation"></div>
  <div class="pal" transition:scale={{ start: 0.97, duration: 160 }} role="dialog" aria-label="Search">
    <div class="in">
      <Search size={18} />
      <input bind:this={input} bind:value={q} onkeydown={key} placeholder="Search apps, pages and actions…" />
      <kbd>Esc</kbd>
    </div>
    <div class="list">
      {#each commands as c, i (c.id)}
        {#if i === 0 || commands[i - 1].group !== c.group}<div class="grp">{c.group}</div>{/if}
        <button class="cmd" class:on={i === idx} onmouseenter={() => (idx = i)} onclick={() => run(c)}>
          {#if c.exe !== undefined}
            <AppIcon exe={c.exe} name={c.label} size={26} />
          {:else}
            {@const Icon = c.icon}
            <span class="ci"><Icon size={15} /></span>
          {/if}
          <span class="lbl ellipsis">{c.label}</span>
          {#if c.hint}<span class="faint hint num">{c.hint}</span>{/if}
          {#if i === idx}<CornerDownLeft size={14} class="faint" />{/if}
        </button>
      {:else}
        <p class="none faint">Nothing found for “{q}”</p>
      {/each}
    </div>
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; z-index: 80; background: rgba(5, 6, 10, .45); backdrop-filter: blur(3px); }
  .pal { position: fixed; z-index: 81; left: 50%; top: 12vh; translate: -50% 0; width: min(600px, calc(100vw - 24px)); border-radius: 18px; background: var(--surface); box-shadow: var(--shadow-pop); overflow: hidden; }
  .in { display: flex; align-items: center; gap: 12px; padding: 0 16px; height: 56px; border-bottom: 1px solid var(--border); color: var(--faint); }
  .in input { flex: 1; border: 0; outline: none; background: none; font-size: 15.5px; color: var(--text); user-select: text; }
  .list { max-height: 420px; overflow-y: auto; padding: 6px; }
  .grp { font-size: 11px; font-weight: 650; text-transform: uppercase; letter-spacing: .08em; color: var(--faint); padding: 10px 10px 4px; }
  .cmd { width: 100%; display: flex; align-items: center; gap: 12px; padding: 8px 10px; border: 0; border-radius: 10px; background: transparent; text-align: left; font-size: 13.5px; }
  .cmd.on { background: var(--accent-soft); }
  .ci { width: 26px; height: 26px; border-radius: 8px; display: grid; place-items: center; background: var(--surface-2); color: var(--muted); flex: none; }
  .cmd.on .ci { color: var(--accent); }
  .lbl { flex: 1; font-weight: 550; }
  .hint { font-size: 12px; }
  .none { padding: 30px; text-align: center; }
</style>
