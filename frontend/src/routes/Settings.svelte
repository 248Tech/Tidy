<script lang="ts">
  import qrcode from 'qrcode-generator'
  import { Sun, Moon, Monitor, Smartphone, ShieldCheck, ShieldAlert, RefreshCw, Copy, Eye, Info, LogOut, Wifi } from '@lucide/svelte'
  import { app, applyTheme } from '../lib/state.svelte'
  import { get, post, type Settings, type RemoteStatus } from '../lib/api'
  import { toast, confirmDialog } from '../lib/ui.svelte'
  import { bytes } from '../lib/format'

  let s = $state<Settings | null>(null)
  let remote = $state<RemoteStatus | null>(null)
  let saving = $state(false)
  let portInput = $state(7878)
  let qrUrl = $state('')
  const themeOpts = [{ id: 'system', label: 'Auto', Icon: Monitor }, { id: 'light', label: 'Light', Icon: Sun }, { id: 'dark', label: 'Dark', Icon: Moon }] as const

  async function load() {
    s = await get<Settings>('/api/settings')
    portInput = s.remotePort
    if (app.info?.local) remote = await get<RemoteStatus>('/api/remote')
  }
  load()

  $effect(() => {
    if (!app.info?.local) return
    const t = setInterval(async () => { remote = await get<RemoteStatus>('/api/remote').catch(() => remote) }, 4000)
    return () => clearInterval(t)
  })

  $effect(() => {
    if (remote?.urls?.length && !remote.urls.includes(qrUrl)) qrUrl = remote.urls[0]
  })

  const qrSvg = $derived.by(() => {
    if (!qrUrl) return ''
    const q = qrcode(0, 'M')
    q.addData(qrUrl)
    q.make()
    return q.createSvgTag({ cellSize: 5, margin: 0, scalable: true })
  })

  async function save(patch: Partial<Settings>) {
    if (!s) return
    saving = true
    try {
      s = await post<Settings>('/api/settings', { ...s, ...patch })
      if (app.info?.local) remote = await get<RemoteStatus>('/api/remote')
    } catch (e: any) {
      toast({ tone: 'bad', title: "Couldn't save", body: e.message })
      await load()
    } finally {
      saving = false
    }
  }

  function setTheme(t: 'system' | 'light' | 'dark') {
    app.theme = t
    applyTheme()
    save({ theme: t })
  }

  async function toggleRemote() {
    if (!s) return
    if (!s.remoteEnabled) {
      const ok = await confirmDialog({
        title: 'Turn on remote access?',
        body: 'Phones and computers on your home network will be able to open Tidy in a browser — but only after typing the pairing code shown here.',
        confirmLabel: 'Turn on', icon: 'shield', tone: 'primary',
        note: 'Only do this on networks you trust (like your home Wi-Fi). Turn on “View only” if remote devices shouldn\'t be able to close apps or delete files.',
      })
      if (!ok) return
    }
    await save({ remoteEnabled: !s.remoteEnabled })
    if (s.remoteEnabled && remote?.running) toast({ tone: 'good', title: 'Remote access is on', body: 'Scan the QR code with your phone to connect.' })
  }

  async function newPin() {
    const ok = await confirmDialog({ title: 'Make a new pairing code?', body: 'Every device that is connected now will be signed out and need the new code.', confirmLabel: 'New code', icon: 'shield' })
    if (ok) remote = await post<RemoteStatus>('/api/remote/pin')
  }

  async function adminRestart() {
    try { await post('/api/admin/restart') } catch (e: any) { toast({ tone: 'warn', title: 'Not restarted', body: e.message }) }
  }

  function copy(t: string) {
    navigator.clipboard?.writeText(t).then(() => toast({ tone: 'good', title: 'Link copied' }))
  }

  async function logout() {
    await post('/api/logout').catch(() => {})
    location.reload()
  }
</script>

<div class="page narrow">
  <div class="page-head"><div><h1>Settings</h1><p>Make Tidy yours.</p></div></div>

  <section class="card group">
    <div class="row">
      <div class="txt"><strong>Appearance</strong><p class="muted">Light, dark, or follow Windows.</p></div>
      <div class="themes">
        {#each themeOpts as { id, label, Icon }}
          <button class="theme" class:on={app.theme === id} onclick={() => setTheme(id as any)}>
            <span class="swatch {id}"></span>
            <span><Icon size={13} /> {label}</span>
          </button>
        {/each}
      </div>
    </div>
  </section>

  {#if app.info?.local}
    <section class="card group">
      <div class="row">
        <span class="g-ico"><Smartphone size={18} /></span>
        <div class="txt">
          <strong>Remote access</strong>
          <p class="muted">Check on this PC from your phone or another computer on the same network.</p>
        </div>
        {#if app.info.headless}
          <span class="badge good">Always on</span>
        {:else}
          <button class="switch" class:on={s?.remoteEnabled} onclick={toggleRemote} disabled={saving || !s} aria-label="Remote access"></button>
        {/if}
      </div>

      {#if remote?.running}
        <div class="remote">
          <div class="qr-wrap">
            <div class="qr">{@html qrSvg}</div>
            <span class="faint small">Scan with your phone's camera</span>
          </div>
          <div class="r-info">
            <span class="faint small">Pairing code</span>
            <div class="pin num">{#each (remote.pin ?? '').split('') as d}<span>{d}</span>{/each}</div>
            <button class="btn sm ghost" onclick={newPin}><RefreshCw size={13} /> New code</button>

            <span class="faint small" style="margin-top:14px">Or open this address</span>
            {#each remote.urls as u}
              <div class="url">
                <button class="mono ellipsis" class:sel={u === qrUrl} onclick={() => (qrUrl = u)}>{u}</button>
                <button class="btn sm icon ghost" onclick={() => copy(u)} aria-label="Copy"><Copy size={13} /></button>
              </div>
            {:else}
              <p class="muted small">No network connection found.</p>
            {/each}
            <p class="small faint" style="margin-top:8px"><Wifi size={12} /> {remote.sessions} device{remote.sessions === 1 ? '' : 's'} connected</p>
          </div>
        </div>
        <div class="note"><Info size={14} /><span>Your browser will warn that the connection “isn't private” the first time. That's expected — Tidy uses its own security certificate. Choose <em>Advanced → Continue</em>.</span></div>
      {:else if remote?.error && s?.remoteEnabled}
        <div class="note bad"><ShieldAlert size={14} /><span>{remote.error}</span></div>
      {/if}

      {#if s?.remoteEnabled || app.info.headless}
        <div class="row sub">
          <span class="g-ico soft"><Eye size={16} /></span>
          <div class="txt"><strong>View only</strong><p class="muted">Remote devices can look, but can't close apps or delete files.</p></div>
          <button class="switch" class:on={s?.remoteViewOnly} onclick={() => save({ remoteViewOnly: !s?.remoteViewOnly })} disabled={saving} aria-label="View only"></button>
        </div>
        {#if !app.info.headless}
          <div class="row sub">
            <div class="txt"><strong>Port</strong><p class="muted">Change this only if something else uses {s?.remotePort}.</p></div>
            <input class="input port num" type="number" min="1024" max="65535" bind:value={portInput} onchange={() => save({ remotePort: +portInput })} />
          </div>
        {/if}
      {/if}
    </section>

    {#if !app.info.headless}
      <section class="card group">
        <div class="row">
          <span class="g-ico {app.info.system.isAdmin ? 'good' : ''}">{#if app.info.system.isAdmin}<ShieldCheck size={18} />{:else}<ShieldAlert size={18} />{/if}</span>
          <div class="txt">
            <strong>{app.info.system.isAdmin ? 'Running as administrator' : 'Admin mode'}</strong>
            <p class="muted">{app.info.system.isAdmin ? 'Tidy can see and stop everything, and scan protected folders.' : 'Some apps and folders need admin rights. Restart Tidy as admin to include them.'}</p>
          </div>
          {#if !app.info.system.isAdmin}<button class="btn" onclick={adminRestart}>Restart as admin</button>{/if}
        </div>
      </section>
    {/if}
  {:else}
    <section class="card group">
      <div class="row">
        <span class="g-ico"><Smartphone size={18} /></span>
        <div class="txt"><strong>Connected remotely</strong><p class="muted">You're viewing {app.info?.system.hostname ?? 'this PC'} from another device{app.viewOnly ? ' in view-only mode' : ''}.</p></div>
        <button class="btn" onclick={logout}><LogOut size={15} /> Sign out</button>
      </div>
    </section>
  {/if}

  <section class="card group about">
    <div class="row">
      <div class="txt">
        <strong>Tidy {app.info?.system.version ?? ''}</strong>
        <p class="muted">{app.info?.system.cpuModel} · {app.info ? bytes(app.info.system.memTotal) : ''} memory · {app.info?.system.cores} threads</p>
        <p class="faint small" style="margin-top:6px">Every app closed or file removed is noted in <span class="mono">%APPDATA%\Tidy\audit.log</span>.</p>
      </div>
    </div>
  </section>
</div>

<style>
  .narrow > :global(*) { max-width: 860px; }
  .group { padding: 6px 20px; margin-bottom: 14px; }
  .row { display: flex; align-items: center; gap: 14px; padding: 16px 0; }
  .row + .row { border-top: 1px solid var(--border); }
  .row.sub { border-top: 1px solid var(--border); }
  .txt { flex: 1; min-width: 0; }
  .txt strong { font-weight: 600; }
  .txt p { font-size: 13px; }
  .small { font-size: 12px; display: inline-flex; align-items: center; gap: 5px; }
  .g-ico { flex: none; width: 38px; height: 38px; border-radius: 12px; display: grid; place-items: center; background: var(--accent-soft); color: var(--accent); }
  .g-ico.soft { background: var(--surface-2); color: var(--muted); }
  .g-ico.good { background: var(--good-soft); color: var(--good); }

  .themes { display: flex; gap: 10px; }
  .theme { display: flex; flex-direction: column; gap: 6px; padding: 6px; border-radius: 12px; border: 1.5px solid var(--border); background: transparent; font-size: 12px; font-weight: 550; color: var(--muted); transition: border-color .15s; }
  .theme span:last-child { display: flex; align-items: center; justify-content: center; gap: 5px; padding-bottom: 2px; }
  .theme.on { border-color: var(--accent); color: var(--text); }
  .swatch { width: 74px; height: 46px; border-radius: 8px; display: block; border: 1px solid var(--border); }
  .swatch.light { background: linear-gradient(135deg, #fff 50%, #eceef5 50%); }
  .swatch.dark { background: linear-gradient(135deg, #1a1d27 50%, #0b0c11 50%); }
  .swatch.system { background: linear-gradient(135deg, #fff 50%, #0b0c11 50%); }

  .remote { display: flex; gap: 28px; padding: 18px; margin-bottom: 12px; border-radius: 14px; background: var(--surface-2); }
  .qr-wrap { display: flex; flex-direction: column; align-items: center; gap: 8px; }
  .qr { width: 156px; height: 156px; padding: 12px; border-radius: 14px; background: #fff; }
  .qr :global(svg) { width: 100%; height: 100%; display: block; }
  .qr :global(rect[fill='#ffffff']), .qr :global(path) { shape-rendering: crispEdges; }
  .r-info { flex: 1; min-width: 0; display: flex; flex-direction: column; align-items: flex-start; gap: 6px; }
  .pin { display: flex; gap: 6px; }
  .pin span { width: 38px; height: 46px; border-radius: 10px; display: grid; place-items: center; font-size: 22px; font-weight: 700; background: var(--surface); border: 1px solid var(--border-strong); }
  .url { display: flex; align-items: center; gap: 4px; width: 100%; }
  .url .mono { flex: 1; min-width: 0; text-align: left; border: 0; background: none; padding: 4px 8px; border-radius: 6px; color: var(--muted); user-select: text; }
  .url .mono.sel { background: var(--accent-soft); color: var(--text); }
  .note { display: flex; gap: 10px; align-items: flex-start; padding: 12px 14px; margin-bottom: 14px; border-radius: 12px; background: var(--accent-soft); font-size: 12.5px; color: var(--muted); }
  .note :global(svg) { flex: none; margin-top: 2px; color: var(--accent); }
  .note.bad { background: var(--bad-soft); color: var(--bad); }
  .note.bad :global(svg) { color: var(--bad); }
  .port { width: 110px; }
  @media (max-width: 720px) {
    .remote { flex-direction: column; align-items: center; }
    .themes .swatch { width: 54px; }
    .row { flex-wrap: wrap; }
  }
</style>
