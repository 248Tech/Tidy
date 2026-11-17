<script lang="ts">
  import { Cpu, MemoryStick, HardDrive, Wifi } from '@lucide/svelte'
  import { app } from '../lib/state.svelte'
  import { bytes, rate, duration } from '../lib/format'
  import AreaChart from '../components/AreaChart.svelte'

  const h = $derived(app.history)
  const m = $derived(app.metrics)
  const last = $derived(h[h.length - 1])
</script>

<div class="page">
  <div class="page-head">
    <div>
      <h1>Activity</h1>
      <p>Live view of the last two minutes. Hover a graph to look back.</p>
    </div>
    {#if m}<span class="badge accent"><span class="live"></span> Live · up {duration(m.uptime)}</span>{/if}
  </div>

  <div class="grid">
    <section class="card panel">
      <header>
        <span class="ico"><Cpu size={16} /></span>
        <div><h3>Processor</h3><p class="faint">{app.info?.system.cpuModel ?? ''}</p></div>
        <strong class="num big">{last ? last.cpu.toFixed(0) : '–'}<small>%</small></strong>
      </header>
      <AreaChart series={[{ label: 'CPU', color: '#8b7bff', data: h.map((s) => s.cpu) }]} max={100} format={(v) => `${v.toFixed(0)}%`} />
      {#if m?.perCore?.length}
        <div class="cores">
          {#each m.perCore as c, i}
            <div class="core" title="Core {i + 1}: {c.toFixed(0)}%"><span style="height:{Math.max(4, c)}%"></span></div>
          {/each}
        </div>
        <p class="faint small">Each bar is one of your {m.perCore.length} processor cores.</p>
      {/if}
    </section>

    <section class="card panel">
      <header>
        <span class="ico teal"><MemoryStick size={16} /></span>
        <div><h3>Memory</h3><p class="faint">{m ? `${bytes(m.memUsed)} of ${bytes(m.memTotal)}` : ''}</p></div>
        <strong class="num big">{last ? last.mem.toFixed(0) : '–'}<small>%</small></strong>
      </header>
      <AreaChart series={[{ label: 'Memory', color: '#3dd6c4', data: h.map((s) => s.mem) }]} max={100} format={(v) => `${v.toFixed(0)}%`} />
      {#if m}
        <div class="split">
          <div><span class="faint">In use</span><strong class="num">{bytes(m.memUsed)}</strong></div>
          <div><span class="faint">Available</span><strong class="num">{bytes(m.memTotal - m.memUsed)}</strong></div>
          <div title="Memory apps have reserved, including the page file on disk"><span class="faint">Reserved</span><strong class="num">{bytes(m.commitUsed)}</strong></div>
        </div>
      {/if}
    </section>

    <section class="card panel">
      <header>
        <span class="ico warm"><HardDrive size={16} /></span>
        <div><h3>Disk activity</h3><p class="faint">How fast files are being read and saved</p></div>
        <strong class="num big sm">{last ? rate(last.diskR + last.diskW) : '–'}</strong>
      </header>
      <AreaChart series={[{ label: 'Reading', color: '#60a5fa', data: h.map((s) => s.diskR) }, { label: 'Saving', color: '#fb923c', data: h.map((s) => s.diskW) }]} format={rate} />
      <div class="legend"><span><i style="background:#60a5fa"></i>Reading {last ? rate(last.diskR) : ''}</span><span><i style="background:#fb923c"></i>Saving {last ? rate(last.diskW) : ''}</span></div>
    </section>

    <section class="card panel">
      <header>
        <span class="ico pink"><Wifi size={16} /></span>
        <div><h3>Network</h3><p class="faint">Internet and local network traffic</p></div>
        <strong class="num big sm">{last ? rate(last.down) : '–'}</strong>
      </header>
      <AreaChart series={[{ label: 'Download', color: '#c084fc', data: h.map((s) => s.down) }, { label: 'Upload', color: '#f472b6', data: h.map((s) => s.up) }]} format={rate} />
      <div class="legend"><span><i style="background:#c084fc"></i>Download {last ? rate(last.down) : ''}</span><span><i style="background:#f472b6"></i>Upload {last ? rate(last.up) : ''}</span></div>
    </section>
  </div>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .panel { padding: 18px 20px; display: flex; flex-direction: column; gap: 14px; min-width: 0; }
  header { display: flex; align-items: center; gap: 12px; }
  header div { flex: 1; min-width: 0; }
  header p { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .ico { width: 32px; height: 32px; border-radius: 10px; display: grid; place-items: center; background: var(--accent-soft); color: var(--accent); flex: none; }
  .ico.teal { background: color-mix(in srgb, #3dd6c4 16%, transparent); color: #3dd6c4; }
  .ico.warm { background: color-mix(in srgb, #fb923c 16%, transparent); color: #fb923c; }
  .ico.pink { background: color-mix(in srgb, #f472b6 16%, transparent); color: #f472b6; }
  .big { font-size: 26px; font-weight: 700; letter-spacing: -.02em; }
  .big.sm { font-size: 18px; }
  .big small { font-size: 13px; color: var(--muted); }
  .cores { display: flex; gap: 3px; height: 40px; align-items: flex-end; }
  .core { flex: 1; height: 100%; border-radius: 4px; background: var(--surface-2); display: flex; align-items: flex-end; overflow: hidden; }
  .core span { width: 100%; background: var(--accent-grad); border-radius: 4px; transition: height .6s var(--ease); }
  .small { font-size: 12px; margin-top: -6px; }
  .split { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
  .split div { display: flex; flex-direction: column; padding: 10px 12px; border-radius: 10px; background: var(--surface-2); font-size: 12px; }
  .split strong { font-size: 15px; }
  .legend { display: flex; gap: 18px; font-size: 12.5px; color: var(--muted); }
  .legend span { display: flex; align-items: center; gap: 6px; }
  .legend i { width: 9px; height: 9px; border-radius: 3px; }
  .live { width: 7px; height: 7px; border-radius: 50%; background: var(--good); box-shadow: 0 0 0 0 var(--good); animation: pulse 1.6s infinite; }
  @keyframes pulse { 70% { box-shadow: 0 0 0 6px transparent; } 100% { box-shadow: 0 0 0 0 transparent; } }
  @media (max-width: 1000px) { .grid { grid-template-columns: 1fr; } }
</style>
