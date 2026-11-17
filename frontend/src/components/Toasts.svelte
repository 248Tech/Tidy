<script lang="ts">
  import { fly } from 'svelte/transition'
  import { flip } from 'svelte/animate'
  import { CircleCheck, CircleAlert, Info, TriangleAlert, X } from '@lucide/svelte'
  import { ui, dismiss } from '../lib/ui.svelte'

  const icons = { good: CircleCheck, bad: CircleAlert, info: Info, warn: TriangleAlert }
</script>

<div class="toasts" aria-live="polite">
  {#each ui.toasts as t (t.id)}
    {@const Icon = icons[t.tone]}
    <div class="toast {t.tone}" animate:flip={{ duration: 250 }} in:fly={{ y: 24, duration: 350 }} out:fly={{ x: 40, duration: 200 }}>
      <span class="ic"><Icon size={18} /></span>
      <div class="txt">
        <strong>{t.title}</strong>
        {#if t.body}<p>{t.body}</p>{/if}
      </div>
      {#if t.action}
        <button class="btn sm" onclick={async () => { dismiss(t.id); await t.action?.run() }}>{t.action.label}</button>
      {/if}
      <button class="x" aria-label="Dismiss" onclick={() => dismiss(t.id)}><X size={14} /></button>
      <span class="ttl" style="animation-duration:{t.ttl}ms"></span>
    </div>
  {/each}
</div>

<style>
  .toasts { position: fixed; right: 20px; bottom: 20px; z-index: 100; display: flex; flex-direction: column; gap: 10px; width: min(400px, calc(100vw - 32px)); }
  .toast {
    position: relative; overflow: hidden;
    display: flex; align-items: center; gap: 12px; padding: 13px 12px 13px 14px;
    border-radius: 14px; background: var(--surface-2); box-shadow: var(--shadow-pop);
    backdrop-filter: blur(20px);
  }
  .ic { flex: none; display: grid; place-items: center; width: 32px; height: 32px; border-radius: 10px; }
  .good .ic { background: var(--good-soft); color: var(--good); }
  .bad .ic { background: var(--bad-soft); color: var(--bad); }
  .warn .ic { background: var(--warn-soft); color: var(--warn); }
  .info .ic { background: var(--accent-soft); color: var(--accent); }
  .txt { flex: 1; min-width: 0; }
  strong { font-size: 13.5px; font-weight: 600; display: block; }
  p { font-size: 12.5px; color: var(--muted); }
  .x { border: 0; background: none; color: var(--faint); padding: 4px; border-radius: 6px; display: grid; }
  .x:hover { color: var(--text); background: var(--hover); }
  .ttl { position: absolute; left: 0; bottom: 0; height: 2px; width: 100%; background: var(--accent-grad); opacity: .5; transform-origin: left; animation: shrink linear forwards; }
  @keyframes shrink { from { transform: scaleX(1); } to { transform: scaleX(0); } }
  @media (max-width: 720px) { .toasts { bottom: 84px; right: 16px; } }
</style>
