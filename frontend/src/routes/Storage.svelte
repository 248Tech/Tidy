<script lang="ts">
  import { fly, fade } from 'svelte/transition'
  import { HardDrive, Usb, Network, FolderSearch, ChevronRight, RefreshCw, X, Trash2, FolderOpen, Check, Radar, ArrowLeft, ShieldAlert } from '@lucide/svelte'
  import { app } from '../lib/state.svelte'
  import { get, post, type Drive, type ScanStatus, type TreeView, type TreeItem, type FileEntry, type CategoryTotal } from '../lib/api'
  import { bytes, count, ago, categoryMeta, sizeFeel } from '../lib/format'
  import { deleteFiles, reveal, type Deletable } from '../lib/actions'
  import { toast } from '../lib/ui.svelte'
  import Treemap from '../components/Treemap.svelte'
  import FileIcon from '../components/FileIcon.svelte'
  import QuickClean from '../components/QuickClean.svelte'

  type Tab = 'explore' | 'biggest' | 'clean'
  let tab = $state<Tab>('explore')
  let drives = $state<Drive[]>([])
  let status = $state<ScanStatus | null>(null)
  let tree = $state<TreeView | null>(null)
  let biggest = $state<FileEntry[]>([])
  let cats = $state<CategoryTotal[]>([])
  let catFilter = $state('all')
  let folderInput = $state('')
  let selected = $state<Map<string, Deletable>>(new Map())
  let pollT: number | undefined

  const selSet = $derived(new Set(selected.keys()))
  const selTotal = $derived([...selected.values()].reduce((a, b) => a + b.size, 0))

  async function init() {
    drives = await get<Drive[]>('/api/drives').catch(() => [])
    status = await get<ScanStatus>('/api/scan/status').catch(() => null)
    const intent = app.storageIntent
    app.storageIntent = null
    if (intent?.tab === 'clean') tab = 'clean'
    if (status?.running) poll()
    else if (status?.done) {
      if (intent?.drive && status.root.toLowerCase() !== intent.drive.toLowerCase()) await start(intent.drive)
      else await loadResults()
    } else if (intent?.drive) await start(intent.drive)
  }
  init()
  $effect(() => () => clearTimeout(pollT))

  async function start(path: string) {
    try {
      status = await post<ScanStatus>('/api/scan/start', { path })
      tree = null
      selected = new Map()
      tab = 'explore'
      poll()
    } catch (e: any) {
      toast({ tone: 'bad', title: "Couldn't start the scan", body: e.message })
    }
  }

  async function poll() {
    clearTimeout(pollT)
    status = await get<ScanStatus>('/api/scan/status').catch(() => status)
    if (status?.running) pollT = window.setTimeout(poll, 350)
    else if (status?.done) loadResults()
  }

  async function loadResults(path = '') {
    const [t, b, c] = await Promise.all([
      get<TreeView>(`/api/scan/tree?path=${encodeURIComponent(path)}`).catch(() => null),
      get<FileEntry[]>(`/api/scan/biggest?limit=300`).catch(() => []),
      get<CategoryTotal[]>('/api/scan/categories').catch(() => []),
    ])
    tree = t
    biggest = b
    cats = c
  }

  async function openFolder(path: string) {
    const t = await get<TreeView>(`/api/scan/tree?path=${encodeURIComponent(path)}`).catch(() => null)
    if (t) tree = t
  }

  function toggle(d: Deletable) {
    const n = new Map(selected)
    n.has(d.path) ? n.delete(d.path) : n.set(d.path, d)
    selected = n
  }

  async function removeSelected(e?: MouseEvent) {
    const removed = await deleteFiles([...selected.values()], !!e?.shiftKey)
    if (!removed.length) return
    const n = new Map(selected)
    removed.forEach((p) => n.delete(p))
    selected = n
    await loadResults(tree?.path ?? '')
    drives = await get<Drive[]>('/api/drives').catch(() => drives)
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Delete' && selected.size && !(e.target instanceof HTMLInputElement)) removeSelected(e as any)
    if (e.key === 'Escape' && selected.size && !app.selectedApp) selected = new Map()
  }

  const shownBiggest = $derived(biggest.filter((f) => catFilter === 'all' || f.category === catFilter).slice(0, 150))
  const catTotal = $derived(cats.reduce((a, c) => a + c.size, 0))
  const progress = $derived(status && status.expected ? Math.min(99, (status.bytes / status.expected) * 100) : null)
  const driveIcon = (k: string) => (k === 'removable' ? Usb : k === 'network' ? Network : HardDrive)
</script>

<svelte:window onkeydown={onKey} />

<div class="page">
  <div class="page-head">
    <div>
      <h1>Storage</h1>
      <p>See what's taking up space, and clear what you don't need.</p>
    </div>
    <div class="segmented">
      <button class:on={tab === 'explore'} onclick={() => (tab = 'explore')}>Explore</button>
      <button class:on={tab === 'biggest'} onclick={() => (tab = 'biggest')} disabled={!status?.done}>Biggest files</button>
      <button class:on={tab === 'clean'} onclick={() => (tab = 'clean')}>Quick clean</button>
    </div>
  </div>

  {#if tab === 'clean'}
    <div in:fly={{ y: 8 }}><QuickClean onreview={(p) => start(p)} /></div>

  {:else if status?.running}
    <!-- ───────── scanning ───────── -->
    <div class="scanning card" in:fade>
      <div class="radar"><Radar size={34} /></div>
      <h2>Looking through {status.root}</h2>
      <p class="found"><span class="num grad-text">{bytes(status.bytes)}</span> found</p>
      <div class="bar big"><span style="width:{progress ?? 100}%" class:indet={progress === null}></span></div>
      <p class="muted num">{count(status.files)} files · {count(status.dirs)} folders · {(status.elapsed / 1000).toFixed(0)}s</p>
      <p class="current mono faint ellipsis">{status.current}</p>
      <button class="btn" onclick={() => post('/api/scan/cancel').then(poll)}><X size={15} /> Stop</button>
    </div>

  {:else if tab === 'explore' && tree}
    <!-- ───────── explore results ───────── -->
    <div class="crumbs">
      {#if tree.crumbs.length > 1}
        <button class="btn sm icon ghost" aria-label="Up one folder" onclick={() => openFolder(tree!.crumbs[tree!.crumbs.length - 2].path)}><ArrowLeft size={15} /></button>
      {/if}
      {#each tree.crumbs as c, i}
        {#if i > 0}<ChevronRight size={14} class="faint" />{/if}
        <button class="crumb" class:last={i === tree.crumbs.length - 1} onclick={() => openFolder(c.path)}>{c.name}</button>
      {/each}
      <span class="spacer"></span>
      <span class="faint num">{bytes(tree.size)} · {count(tree.files)} files</span>
      <button class="btn sm" onclick={() => start(status!.root)}><RefreshCw size={14} /> Rescan</button>
      <button class="btn sm ghost" onclick={() => { tree = null; status = null }}>New scan</button>
    </div>

    <div class="explore">
      <div class="card mapwrap">
        {#key tree.path}
          <div class="mapin" in:fade={{ duration: 250 }}>
            <Treemap items={tree.items} total={tree.size} selected={selSet}
              onopen={(it) => openFolder(it.path)}
              ontoggle={(it) => toggle({ path: it.path, name: it.name, size: it.size })} />
          </div>
        {/key}
      </div>
      <div class="card side">
        <div class="side-head"><strong>In this folder</strong><span class="faint">{tree.items.length} items</span></div>
        <div class="items">
          {#each tree.items as it (it.path || it.name)}
            <div class="it" class:sel={selSet.has(it.path)}>
              {#if !it.other}
                <button class="check" class:on={selSet.has(it.path)} onclick={() => toggle({ path: it.path, name: it.name, size: it.size })} aria-label="Select {it.name}">{#if selSet.has(it.path)}<Check size={12} strokeWidth={3} />{/if}</button>
              {:else}<span style="width:18px"></span>{/if}
              <button class="it-main" onclick={() => (it.isDir ? openFolder(it.path) : !it.other && toggle({ path: it.path, name: it.name, size: it.size }))} disabled={it.other}>
                <FileIcon category={it.category} dir={it.isDir} size={30} />
                <span class="it-txt">
                  <span class="ellipsis">{it.name}</span>
                  <span class="bar"><span style="width:{(it.size / Math.max(1, tree.items[0]?.size)) * 100}%;background:{categoryMeta[it.category]?.color}"></span></span>
                </span>
                <span class="num it-size">{bytes(it.size)}</span>
              </button>
              {#if app.desktop && !it.other}
                <button class="btn sm icon ghost rv" title="Show in File Explorer" onclick={() => reveal(it.path)}><FolderOpen size={14} /></button>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    </div>
    {#if status && status.denied > 0}
      <p class="denied faint"><ShieldAlert size={13} /> {count(status.denied)} folders were off-limits. {app.info?.system.isAdmin ? '' : 'Restart Tidy as admin (Settings) to include them.'}</p>
    {/if}

  {:else if tab === 'biggest' && status?.done}
    <!-- ───────── biggest files ───────── -->
    <div class="catbar card">
      <div class="stack">
        {#each cats.filter((c) => c.size > 0) as c}
          <span style="width:{(c.size / Math.max(1, catTotal)) * 100}%;background:{categoryMeta[c.category]?.color}" title="{categoryMeta[c.category]?.label}: {bytes(c.size)}"></span>
        {/each}
      </div>
      <div class="chips">
        <button class="chip" class:on={catFilter === 'all'} onclick={() => (catFilter = 'all')}>Everything</button>
        {#each cats.filter((c) => c.size > 0) as c}
          <button class="chip" class:on={catFilter === c.category} onclick={() => (catFilter = c.category)}>
            <i style="background:{categoryMeta[c.category]?.color}"></i>{categoryMeta[c.category]?.label} <span class="faint num">{bytes(c.size)}</span>
          </button>
        {/each}
      </div>
    </div>
    <div class="card flist">
      {#each shownBiggest as f (f.path)}
        <div class="frow" class:sel={selSet.has(f.path)}>
          <button class="check" class:on={selSet.has(f.path)} onclick={() => toggle(f)} aria-label="Select {f.name}">{#if selSet.has(f.path)}<Check size={12} strokeWidth={3} />{/if}</button>
          <button class="f-main" onclick={() => toggle(f)}>
            <FileIcon category={f.category} size={34} />
            <span class="f-txt">
              <strong class="ellipsis">{f.name}</strong>
              <span class="faint ellipsis mono">{f.path.slice(0, f.path.length - f.name.length - 1)}</span>
            </span>
            <span class="faint f-age">{ago(f.modified)}</span>
            <span class="num f-size">{bytes(f.size)}</span>
          </button>
          {#if app.desktop}<button class="btn sm icon ghost rv" title="Show in File Explorer" onclick={() => reveal(f.path)}><FolderOpen size={14} /></button>{/if}
        </div>
      {:else}
        <p class="faint" style="padding:30px;text-align:center">No files in this group.</p>
      {/each}
    </div>

  {:else}
    <!-- ───────── pick a drive ───────── -->
    <div class="section-title" style="margin-top:0"><h2>Pick a drive to look through</h2><span class="muted">Scanning only reads — nothing is changed.</span></div>
    <div class="drives">
      {#each drives as d (d.mount)}
        {@const Icon = driveIcon(d.kind)}
        {@const used = ((d.total - d.free) / d.total) * 100}
        <button class="drive card" onclick={() => start(d.mount)}>
          <span class="d-ico"><Icon size={22} /></span>
          <span class="d-name">{d.label || (d.kind === 'fixed' ? 'Local Disk' : 'Drive')} <span class="faint">({d.mount.replace('\\', '')})</span></span>
          <span class="bar"><span style="width:{used}%" class:hot={used > 90}></span></span>
          <span class="d-meta"><strong class="num">{bytes(d.free)}</strong> free of {bytes(d.total)}</span>
          {#if sizeFeel(d.free)}<span class="faint d-feel">Room for {sizeFeel(d.free)}</span>{/if}
          <span class="d-go">Scan <ChevronRight size={14} /></span>
        </button>
      {/each}
    </div>
    <form class="folder card" onsubmit={(e) => { e.preventDefault(); folderInput.trim() && start(folderInput.trim()) }}>
      <FolderSearch size={18} class="faint" />
      <input class="input" placeholder="Or type a folder, e.g. C:\Users\you\Downloads" bind:value={folderInput} />
      <button class="btn" type="submit" disabled={!folderInput.trim()}>Scan folder</button>
    </form>
    {#if status?.error === 'cancelled'}<p class="faint" style="margin-top:12px">The last scan was stopped.</p>{/if}
  {/if}
</div>

{#if selected.size && tab !== 'clean'}
  <div class="selbar" transition:fly={{ y: 80, duration: 300 }}>
    <span class="count num">{selected.size}</span>
    <div class="sel-txt">
      <strong>{selected.size === 1 ? '1 item' : `${selected.size} items`} selected</strong>
      <span class="muted num">{bytes(selTotal)} will be freed</span>
    </div>
    <button class="btn ghost" onclick={() => (selected = new Map())}>Clear</button>
    <button class="btn danger" onclick={removeSelected} title="Hold Shift to delete forever"><Trash2 size={16} /> Move to Recycle Bin</button>
  </div>
{/if}

<style>
  /* scanning */
  .scanning { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 10px; padding: 50px 30px; }
  .radar { width: 76px; height: 76px; border-radius: 50%; display: grid; place-items: center; color: var(--accent); background: var(--accent-soft); position: relative; margin-bottom: 8px; }
  .radar::before, .radar::after { content: ''; position: absolute; inset: 0; border-radius: 50%; border: 2px solid var(--accent); opacity: 0; animation: ping 2s infinite; }
  .radar::after { animation-delay: 1s; }
  @keyframes ping { 0% { transform: scale(1); opacity: .6; } 100% { transform: scale(1.9); opacity: 0; } }
  .found { font-size: 15px; color: var(--muted); }
  .found span { font-size: 40px; font-weight: 700; letter-spacing: -.03em; }
  .bar.big { width: min(460px, 100%); height: 8px; }
  .bar .indet { width: 30% !important; animation: indet 1.4s infinite var(--ease); }
  @keyframes indet { from { transform: translateX(-100%); } to { transform: translateX(340%); } }
  .current { max-width: min(560px, 100%); font-size: 11.5px; margin-bottom: 10px; }

  /* explore */
  .crumbs { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; margin-bottom: 12px; font-size: 13px; }
  .crumb { border: 0; background: none; padding: 4px 8px; border-radius: 7px; color: var(--muted); font-weight: 550; max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .crumb:hover { background: var(--hover); color: var(--text); }
  .crumb.last { color: var(--text); }
  .spacer { flex: 1; }
  .crumbs .faint { margin-right: 6px; }
  .explore { display: grid; grid-template-columns: 1fr 360px; gap: 16px; height: calc(100vh - 250px); min-height: 420px; }
  .mapwrap { padding: 8px; overflow: hidden; }
  .mapin { width: 100%; height: 100%; }
  .side { display: flex; flex-direction: column; min-height: 0; }
  .side-head { display: flex; justify-content: space-between; padding: 14px 16px 10px; font-size: 13px; }
  .items { overflow-y: auto; padding: 0 6px 6px; }
  .it, .frow { display: flex; align-items: center; gap: 8px; padding: 0 6px 0 10px; border-radius: 10px; transition: background .15s; }
  .it:hover, .frow:hover { background: var(--hover); }
  .it.sel, .frow.sel { background: var(--accent-soft); }
  .it-main, .f-main { flex: 1; min-width: 0; display: flex; align-items: center; gap: 10px; padding: 8px 0; border: 0; background: none; text-align: left; }
  .it-txt { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 5px; font-size: 13px; font-weight: 550; }
  .it-txt .bar { height: 3px; }
  .it-size { font-size: 12.5px; color: var(--muted); font-weight: 550; }
  .rv { opacity: 0; }
  .it:hover .rv, .frow:hover .rv { opacity: 1; }
  .denied { display: flex; align-items: center; gap: 6px; margin-top: 10px; font-size: 12px; }

  /* biggest */
  .catbar { padding: 16px; margin-bottom: 14px; }
  .stack { display: flex; height: 12px; border-radius: 99px; overflow: hidden; gap: 2px; margin-bottom: 14px; }
  .stack span { min-width: 3px; transition: width .5s var(--ease); }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chip i { width: 8px; height: 8px; border-radius: 3px; }
  .flist { padding: 4px; }
  .f-txt { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .f-txt strong { font-weight: 600; font-size: 13.5px; }
  .f-txt .mono { font-size: 11.5px; }
  .f-age { font-size: 12px; width: 110px; text-align: right; }
  .f-size { width: 84px; text-align: right; font-weight: 650; }

  /* drives */
  .drives { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; }
  .drive { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; padding: 20px; text-align: left; transition: transform .2s var(--ease), border-color .2s; }
  .drive:hover { transform: translateY(-3px); border-color: color-mix(in srgb, var(--accent) 50%, transparent); }
  .d-ico { width: 44px; height: 44px; border-radius: 13px; display: grid; place-items: center; background: var(--accent-soft); color: var(--accent); }
  .d-name { font-weight: 650; font-size: 15px; }
  .drive .bar { width: 100%; height: 8px; }
  .drive .bar .hot { background: linear-gradient(90deg, var(--warn), var(--bad)); }
  .d-meta { font-size: 13px; color: var(--muted); }
  .d-meta strong { color: var(--text); }
  .d-feel { font-size: 12px; margin-top: -6px; }
  .d-go { display: flex; align-items: center; gap: 2px; color: var(--accent); font-weight: 600; font-size: 13px; margin-top: 4px; }
  .folder { display: flex; align-items: center; gap: 12px; padding: 12px 12px 12px 16px; margin-top: 16px; }
  .folder .input { flex: 1; }

  /* selection bar */
  .selbar {
    position: fixed; z-index: 30; left: 50%; bottom: 24px; translate: -50% 0; margin-left: 114px;
    display: flex; align-items: center; gap: 14px; padding: 10px 10px 10px 12px;
    border-radius: 16px; background: var(--surface-2); box-shadow: var(--shadow-pop); backdrop-filter: blur(20px);
    width: min(620px, calc(100vw - 280px));
  }
  .selbar .count { min-width: 36px; height: 36px; padding: 0 8px; border-radius: 11px; display: grid; place-items: center; background: var(--accent-grad); color: #fff; font-weight: 700; }
  .sel-txt { flex: 1; display: flex; flex-direction: column; line-height: 1.3; }
  .sel-txt span { font-size: 12.5px; }

  @media (max-width: 1000px) {
    .explore { grid-template-columns: 1fr; height: auto; }
    .mapwrap { height: 380px; }
    .side { max-height: 420px; }
    .f-age { display: none; }
  }
  @media (max-width: 720px) {
    .selbar { margin-left: 0; width: calc(100vw - 24px); bottom: 80px; }
    .selbar .ghost { display: none; }
  }
</style>
