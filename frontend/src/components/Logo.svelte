<script lang="ts">
  let { size = 28 }: { size?: number } = $props()
  const id = `lg${Math.random().toString(36).slice(2, 7)}`
</script>

<svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
  <defs>
    <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#8b7bff" />
      <stop offset="1" stop-color="#3dd6c4" />
    </linearGradient>
  </defs>
  <rect width="64" height="64" rx="18" fill="url(#{id})" />
  <path d="M20 34l8 8 16-18" fill="none" stroke="#fff" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" />
</svg>
