<script lang="ts">
  import { scale } from 'svelte/transition'
  import { Lock } from '@lucide/svelte'
  import { post } from '../lib/api'
  import { afterLogin } from '../lib/state.svelte'
  import Logo from './Logo.svelte'

  let digits = $state(['', '', '', '', '', ''])
  let inputs: HTMLInputElement[] = []
  let error = $state('')
  let busy = $state(false)
  let shake = $state(false)

  async function submit() {
    const pin = digits.join('')
    if (pin.length !== 6 || busy) return
    busy = true
    error = ''
    try {
      await post('/api/login', { pin })
      afterLogin()
    } catch (e: any) {
      error = e.message
      digits = ['', '', '', '', '', '']
      shake = true
      setTimeout(() => { shake = false; inputs[0]?.focus() }, 400)
    } finally {
      busy = false
    }
  }

  function onInput(i: number, e: Event) {
    const el = e.target as HTMLInputElement
    const v = el.value.replace(/\D/g, '')
    if (v.length > 1) { // pasted
      v.slice(0, 6).split('').forEach((d, j) => { if (i + j < 6) digits[i + j] = d })
      inputs[Math.min(5, i + v.length)]?.focus()
    } else {
      digits[i] = v
      if (v && i < 5) inputs[i + 1]?.focus()
    }
    if (digits.every((d) => d)) submit()
  }

  function onKey(i: number, e: KeyboardEvent) {
    if (e.key === 'Backspace' && !digits[i] && i > 0) inputs[i - 1]?.focus()
    if (e.key === 'Enter') submit()
  }
</script>

<div class="wrap">
  <div class="card login" class:shake in:scale={{ start: 0.96, duration: 300 }}>
    <Logo size={52} />
    <h1>Connect to Tidy</h1>
    <p class="muted">Type the 6-digit code shown in Tidy on your PC<br /><span class="faint">Settings → Remote access</span></p>
    <div class="pin">
      {#each digits as _, i}
        <input
          bind:this={inputs[i]}
          value={digits[i]}
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          aria-label={`Digit ${i + 1}`}
          oninput={(e) => onInput(i, e)}
          onkeydown={(e) => onKey(i, e)}
          disabled={busy}
          autofocus={i === 0}
        />
      {/each}
    </div>
    {#if error}<p class="err">{error}</p>{/if}
    <p class="hint"><Lock size={12} /> Your connection is encrypted. Only devices with the code can connect.</p>
  </div>
</div>

<style>
  .wrap { height: 100%; display: grid; place-items: center; padding: 16px; }
  .login { width: min(420px, 100%); padding: 40px 32px 28px; text-align: center; display: flex; flex-direction: column; align-items: center; gap: 10px; }
  h1 { font-size: 22px; margin-top: 10px; }
  .pin { display: flex; gap: 8px; margin: 18px 0 6px; }
  .pin input {
    width: 48px; height: 58px; text-align: center; font-size: 24px; font-weight: 650;
    border-radius: 12px; border: 1px solid var(--border-strong); background: var(--surface-2); outline: none;
    transition: border-color .15s, box-shadow .15s, transform .15s; user-select: text;
  }
  .pin input:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); transform: translateY(-1px); }
  .err { color: var(--bad); font-size: 13px; }
  .hint { display: flex; align-items: center; gap: 6px; color: var(--faint); font-size: 12px; margin-top: 12px; }
  .shake { animation: shake .4s; }
  @keyframes shake { 20%, 60% { transform: translateX(-8px); } 40%, 80% { transform: translateX(8px); } }
  @media (max-width: 400px) { .pin input { width: 40px; height: 50px; } }
</style>
