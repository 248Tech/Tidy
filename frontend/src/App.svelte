<script lang="ts">
  import { fade, fly } from 'svelte/transition'
  import { app } from './lib/state.svelte'
  import TitleBar from './components/TitleBar.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Toasts from './components/Toasts.svelte'
  import ConfirmDialog from './components/ConfirmDialog.svelte'
  import AppDrawer from './components/AppDrawer.svelte'
  import CommandPalette from './components/CommandPalette.svelte'
  import Login from './components/Login.svelte'
  import Home from './routes/Home.svelte'
  import Apps from './routes/Apps.svelte'
  import Storage from './routes/Storage.svelte'
  import Activity from './routes/Activity.svelte'
  import Settings from './routes/Settings.svelte'

  const routes = { home: Home, apps: Apps, storage: Storage, activity: Activity, settings: Settings }
  const Page = $derived(routes[app.route])

  function onKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      app.paletteOpen = !app.paletteOpen
    }
    if (e.ctrlKey && /^[1-5]$/.test(e.key)) {
      e.preventDefault()
      app.go((['home', 'apps', 'storage', 'activity', 'settings'] as const)[+e.key - 1])
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="backdrop" aria-hidden="true"></div>

{#if app.needLogin}
  <Login />
{:else}
  <div class="shell" class:desktop={app.desktop}>
    {#if app.desktop}<TitleBar />{/if}
    <div class="body">
      <Sidebar />
      <main>
        {#if app.offline}
          <div class="offline" transition:fly={{ y: -10 }}>Can't reach Tidy right now — retrying…</div>
        {/if}
        {#key app.route}
          <div class="route" in:fly={{ y: 10, duration: 260, delay: 60 }} out:fade={{ duration: 60 }}>
            <Page />
          </div>
        {/key}
      </main>
    </div>
  </div>
  <AppDrawer />
  <CommandPalette />
{/if}

<ConfirmDialog />
<Toasts />

<style>
  .backdrop {
    position: fixed; inset: 0; z-index: -1; pointer-events: none;
    background:
      radial-gradient(900px 600px at 0% -10%, var(--bg-glow-1), transparent 60%),
      radial-gradient(700px 500px at 110% 110%, var(--bg-glow-2), transparent 60%),
      var(--bg);
  }
  .shell { height: 100%; display: flex; flex-direction: column; }
  .body { flex: 1; display: flex; min-height: 0; }
  main { flex: 1; overflow-y: auto; overflow-x: hidden; position: relative; scroll-behavior: smooth; }
  .route { min-height: 100%; }
  .offline {
    position: sticky; top: 0; z-index: 5; margin: 0 auto; width: fit-content;
    padding: 6px 14px; border-radius: 0 0 10px 10px; font-size: 12.5px; font-weight: 550;
    background: var(--warn-soft); color: var(--warn); backdrop-filter: blur(10px);
  }
  @media (max-width: 720px) {
    .body { flex-direction: column-reverse; }
  }
</style>
