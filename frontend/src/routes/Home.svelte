<script lang="ts" module>
  import type { CleanItem } from '../lib/api'
  let cleanCache: { at: number; items: CleanItem[] } | null = null
</script>

<script lang="ts">
  import { fly } from 'svelte/transition'
  import { Cpu, MemoryStick, HardDrive, Sparkles, ArrowRight, Flame, RotateCcw, PartyPopper, Gauge } from '@lucide/svelte'
  import { app } from '../lib/state.svelte'
  import { get } from '../lib/api'
  import { bytes, cpuMood, memMood, diskMood, duration, greeting, sizeFeel } from '../lib/format'
  import Ring from '../components/Ring.svelte'
  import Sparkline from '../components/Sparkline.svelte'
  import AppIcon from '../components/AppIcon.svelte'

  let clean = $state<CleanItem[] | null>(cleanCache?.items ?? null)
  $effect(() => {
    if (cleanCache && Date.now() - cleanCache.at < 5 * 60_000) return
    get<CleanItem[]>('/api/clean').then((items) => { cleanCache = { at: Date.now(), items }; clean = items }).catch(() => {})
  })

  const m = $derived(app.metrics)
  const sys = $derived(m?.drives?.find((d) => d.mount.toUpperCase().startsWith('C')) ?? m?.drives?.[0])
  const recent = $derived(app.history.slice(-10))
  const cpuAvg = $derived(recent.length ? recent.reduce((a, s) => a + s.cpu, 0) / recent.length : 0)
  const memPct = $derived(m ? (m.memUsed / m.memTotal) * 100 : 0)
  const diskPct = $derived(sys ? ((sys.total - sys.free) / sys.total) * 100 : 0)

  const health = $derived.by(() => {
    if (!m) return 0
    const c = 100 - Math.max(0, cpuAvg - 50) * 2
    const me = 100 - Math.max(0, memPct - 60) * 2.5
    const d = 100 - Math.max(0, diskPct - 70) * 3.3
    const worst = Math.min(c, me, d)
    return Math.max(5, Math.round(0.5 * worst + 0.5 * (0.3 * c + 0.35 * me + 0.35 * d)))
  })
  const healthTone = $derived(health >= 80 ? 'good' : health >= 60 ? 'warn' : 'bad')
  const healthLine = $derived(
    health >= 85 ? 'Your PC is in great shape' : health >= 65 ? 'Doing fine — a couple of things to look at' : 'Your PC could use a little help',
  )

  const junk = $derived(clean ? clean.filter((c) => !c.reviewIn).reduce((a, c) => a + c.size, 0) : 0)

  type Tip = { id: string; icon: any; tone: string; title: string; body: string; cta: string; run: () => void }
  const tips = $derived.by(() => {
    const out: Tip[] = []
    for (const d of m?.drives ?? []) {
      const used = (d.total - d.free) / d.total
      if (d.kind === 'fixed' && used > 0.9)
        out.push({ id: 'disk' + d.mount, icon: HardDrive, tone: 'bad', title: `Drive ${d.mount.replace('\\', '')} is almost full`, body: `Only ${bytes(d.free)} left. Let's find what's taking up the space.`, cta: 'Find big files', run: () => { app.storageIntent = { drive: d.mount }; app.go('storage') } })
    }
    if (junk > 200 * 1024 ** 2)
      out.push({ id: 'junk', icon: Sparkles, tone: 'accent', title: `You can free ${bytes(junk)} of clutter`, body: 'Temporary files, caches and your Recycle Bin. All safe to clear.', cta: 'Clean up', run: () => { app.storageIntent = { tab: 'clean' }; app.go('storage') } })
    const heavy = app.groups.filter((g) => g.kind === 'app' && g.protect !== 'blocked' && g.mem > 1.5 * 1024 ** 3).slice(0, 2)
    for (const g of heavy)
      out.push({ id: 'mem' + g.key, icon: MemoryStick, tone: 'warn', title: `${g.name} is using ${bytes(g.mem)} of memory`, body: g.count > 5 ? `It's running ${g.count} parts — closing tabs or windows you don't need helps.` : 'Close it if you are not using it to free memory for other apps.', cta: 'See app', run: () => (app.selectedApp = g.key) })
    const hot = app.groups.find((g) => g.protect !== 'blocked' && g.cpu > 30 && !heavy.includes(g))
    if (hot)
      out.push({ id: 'cpu' + hot.key, icon: Flame, tone: 'warn', title: `${hot.name} is keeping your processor busy`, body: `It's using ${hot.cpu.toFixed(0)}% of your CPU right now.`, cta: 'See app', run: () => (app.selectedApp = hot.key) })
    if (m && m.uptime > 7 * 86400)
      out.push({ id: 'uptime', icon: RotateCcw, tone: 'accent', title: `Your PC has been on for ${duration(m.uptime)}`, body: 'A restart now and then clears out leftovers and installs updates.', cta: 'Got it', run: () => (dismissed = [...dismissed, 'uptime']) })
    return out
  })
  let dismissed = $state<string[]>([])
  const shownTips = $derived(tips.filter((t) => !dismissed.includes(t.id)).slice(0, 4))

  const heaviest = $derived(app.groups.filter((g) => g.kind !== 'system').slice(0, 5))
  const maxMem = $derived(Math.max(1, ...heaviest.map((g) => g.mem)))

  const cpuM = $derived(cpuMood(cpuAvg))
  const memM = $derived(memMood(memPct))
  const diskM = $derived(diskMood(diskPct))
</script>

<div class="page">
  <section class="hero card">
    <div class="glow" aria-hidden="true"></div>
    <Ring value={health} size={168} stroke={14} tone={healthTone}>
      <div class="score">
        <span class="num big">{m ? health : '–'}</span>
        <span class="faint">health</span>
      </div>
    </Ring>
    <div class="hero-text">
      <span class="eyebrow">{greeting()}{app.info ? ` · ${app.info.system.hostname}` : ''}</span>
      <h1>{m ? healthLine : 'Checking your PC…'}</h1>
      <p class="muted">
        {#if m}
          {app.groups.filter((g) => g.kind === 'app').length || '–'} apps open · {m.procs} processes · on for {duration(m.uptime)}
        {:else}&nbsp;{/if}
      </p>
      <div class="hero-actions">
        <button class="btn primary lg" onclick={() => { app.storageIntent = { tab: 'clean' }; app.go('storage') }}><Sparkles size={17} /> Quick clean</button>
        <button class="btn lg" onclick={() => app.go('apps')}>See running apps</button>
      </div>
    </div>
  </section>

  <div class="gauges">
    <button class="gauge card" onclick={() => app.go('activity')}>
      <div class="g-head"><span class="g-ico"><Cpu size={16} /></span>Processor<span class="badge {cpuM.tone}">{cpuM.label}</span></div>
      <div class="g-body">
        <Ring value={cpuAvg} size={84} stroke={8} tone={cpuM.tone === 'good' ? 'accent' : cpuM.tone}><span class="num pct">{cpuAvg.toFixed(0)}<small>%</small></span></Ring>
        <div class="g-side">
          <p>{cpuM.line}</p>
          <Sparkline data={app.history.map((s) => s.cpu).slice(-40)} max={100} width={150} height={34} />
        </div>
      </div>
    </button>
    <button class="gauge card" onclick={() => app.go('apps')}>
      <div class="g-head"><span class="g-ico"><MemoryStick size={16} /></span>Memory<span class="badge {memM.tone}">{memM.label}</span></div>
      <div class="g-body">
        <Ring value={memPct} size={84} stroke={8} tone={memM.tone === 'good' ? 'accent' : memM.tone}><span class="num pct">{memPct.toFixed(0)}<small>%</small></span></Ring>
        <div class="g-side">
          <p>{m ? `${bytes(m.memUsed)} of ${bytes(m.memTotal)} in use.` : ''} {memM.line}</p>
          <Sparkline data={app.history.map((s) => s.mem).slice(-40)} max={100} width={150} height={34} color="var(--accent-2)" />
        </div>
      </div>
    </button>
    <button class="gauge card" onclick={() => { app.storageIntent = { drive: sys?.mount }; app.go('storage') }}>
      <div class="g-head"><span class="g-ico"><HardDrive size={16} /></span>Storage {sys ? sys.mount.replace('\\', '') : ''}<span class="badge {diskM.tone}">{diskM.label}</span></div>
      <div class="g-body">
        <Ring value={diskPct} size={84} stroke={8} tone={diskM.tone === 'good' ? 'accent' : diskM.tone}><span class="num pct">{diskPct.toFixed(0)}<small>%</small></span></Ring>
        <div class="g-side">
          <p><strong class="num">{sys ? bytes(sys.free) : '–'}</strong> free of {sys ? bytes(sys.total) : '–'}</p>
          <p class="faint small">{sys ? sizeFeel(sys.free) ? `That's ${sizeFeel(sys.free)}.` : diskM.line : ''}</p>
        </div>
      </div>
    </button>
  </div>

  <div class="cols">
    <section>
      <div class="section-title"><h2>Suggestions</h2><span class="muted">Things that would help</span></div>
      <div class="tips">
        {#each shownTips as t (t.id)}
          {@const Icon = t.icon}
          <div class="tip card" in:fly={{ y: 8, duration: 300 }}>
            <span class="t-ico {t.tone}"><Icon size={18} /></span>
            <div class="t-txt"><strong>{t.title}</strong><p class="muted">{t.body}</p></div>
            <button class="btn sm" onclick={t.run}>{t.cta} <ArrowRight size={14} /></button>
          </div>
        {:else}
          <div class="tip card calm">
            <span class="t-ico good"><PartyPopper size={18} /></span>
            <div class="t-txt"><strong>{m ? 'All clear!' : 'Looking around…'}</strong><p class="muted">{m ? "Nothing needs your attention right now. Nice." : 'Hang on a second.'}</p></div>
          </div>
        {/each}
      </div>
    </section>

    <section>
      <div class="section-title"><h2>Using the most memory</h2><button class="link-btn" onclick={() => app.go('apps')}>All apps <ArrowRight size={13} /></button></div>
      <div class="card heavy">
        {#each heaviest as g (g.key)}
          <button class="h-row" onclick={() => (app.selectedApp = g.key)}>
            <AppIcon exe={g.exe} name={g.name} size={30} />
            <div class="h-main">
              <div class="h-top"><span class="ellipsis">{g.name}</span><span class="num muted">{bytes(g.mem)}</span></div>
              <div class="bar"><span style="width:{(g.mem / maxMem) * 100}%"></span></div>
            </div>
          </button>
        {:else}
          {#each Array(5) as _}<div class="h-row"><div class="skeleton" style="width:30px;height:30px"></div><div class="h-main"><div class="skeleton" style="height:10px;width:60%"></div></div></div>{/each}
        {/each}
      </div>
    </section>
  </div>
</div>

<style>
  .hero {
    position: relative; overflow: hidden;
    display: flex; align-items: center; gap: 40px; padding: 34px 40px;
    border-radius: var(--radius-lg);
  }
  .glow {
    position: absolute; inset: -40% -10% auto auto; width: 520px; height: 520px; pointer-events: none;
    background: radial-gradient(circle, var(--bg-glow-1), transparent 65%);
  }
  .score { display: flex; flex-direction: column; align-items: center; line-height: 1; gap: 6px; }
  .score .big { font-size: 46px; font-weight: 700; letter-spacing: -.03em; }
  .score .faint { font-size: 12px; text-transform: uppercase; letter-spacing: .12em; }
  .hero-text { position: relative; flex: 1; min-width: 0; }
  .eyebrow { font-size: 12.5px; font-weight: 600; color: var(--accent); letter-spacing: .02em; }
  .hero-text h1 { font-size: 30px; margin: 6px 0 6px; line-height: 1.15; }
  .hero-actions { display: flex; gap: 10px; margin-top: 22px; flex-wrap: wrap; }

  .gauges { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; margin-top: 16px; }
  .gauge { text-align: left; padding: 18px; transition: transform .2s var(--ease), border-color .2s; }
  .gauge:hover { transform: translateY(-2px); border-color: var(--border-strong); }
  .g-head { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 13.5px; }
  .g-head .badge { margin-left: auto; }
  .g-ico { display: grid; place-items: center; width: 28px; height: 28px; border-radius: 8px; background: var(--accent-soft); color: var(--accent); }
  .g-body { display: flex; align-items: center; gap: 16px; margin-top: 14px; }
  .g-side { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; font-size: 12.5px; color: var(--muted); }
  .g-side strong { color: var(--text); font-size: 15px; }
  .small { font-size: 12px; }
  .pct { font-size: 20px; font-weight: 700; letter-spacing: -.02em; }
  .pct small { font-size: 11px; color: var(--muted); margin-left: 1px; }

  .cols { display: grid; grid-template-columns: 1.25fr 1fr; gap: 24px; }
  .tips { display: flex; flex-direction: column; gap: 10px; }
  .tip { display: flex; align-items: center; gap: 14px; padding: 14px 16px; }
  .t-ico { flex: none; width: 38px; height: 38px; border-radius: 12px; display: grid; place-items: center; }
  .t-ico.accent { background: var(--accent-soft); color: var(--accent); }
  .t-ico.warn { background: var(--warn-soft); color: var(--warn); }
  .t-ico.bad { background: var(--bad-soft); color: var(--bad); }
  .t-ico.good { background: var(--good-soft); color: var(--good); }
  .t-txt { flex: 1; min-width: 0; }
  .t-txt strong { font-size: 13.5px; font-weight: 600; }
  .t-txt p { font-size: 12.5px; }

  .link-btn { margin-left: auto; display: inline-flex; align-items: center; gap: 4px; border: 0; background: none; color: var(--accent); font-weight: 550; font-size: 12.5px; }
  .heavy { padding: 6px; }
  .h-row { width: 100%; display: flex; align-items: center; gap: 12px; padding: 10px; border: 0; border-radius: 10px; background: transparent; text-align: left; transition: background .15s; }
  .h-row:hover { background: var(--hover); }
  .h-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
  .h-top { display: flex; justify-content: space-between; gap: 10px; font-size: 13px; font-weight: 550; }
  .h-row .bar { height: 4px; }

  @media (max-width: 1080px) {
    .gauges { grid-template-columns: 1fr 1fr; }
    .cols { grid-template-columns: 1fr; }
  }
  @media (max-width: 720px) {
    .hero { flex-direction: column; text-align: center; padding: 26px 20px; gap: 18px; }
    .hero-text h1 { font-size: 22px; }
    .hero-actions { justify-content: center; }
    .gauges { grid-template-columns: 1fr; }
  }
</style>
