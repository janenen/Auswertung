<script lang="ts">
  // SVG rendering of a target with shots. Geometry is in 1/100 mm, shot
  // coordinates in mm (y up).
  import type { Shot } from './api'
  import { meta } from './meta.svelte'

  let {
    type,
    shots = [],
    zoom = false,
    numbered = false,
    selected = -1,
    onselect,
  }: {
    type: string
    shots?: Shot[]
    /** Fit the view to the shots instead of the whole target. */
    zoom?: boolean
    numbered?: boolean
    selected?: number
    onselect?: (index: number) => void
  } = $props()

  const g = $derived(meta.targets[type])
  const width = $derived(g ? 2 * (g.radiusTen + 9 * g.ringIncrement) : 1)

  const half = $derived.by(() => {
    if (!g) return 1
    const extent = Math.max(0, ...shots.map((s) => Math.max(Math.abs(s.x), Math.abs(s.y)) * 100))
    if (zoom && shots.length) return Math.min(Math.max(extent + g.radiusCalibre * 1.3, g.radiusCalibre * 4), width / 2)
    return Math.max(width / 2, extent + g.radiusCalibre)
  })

  const rings = $derived(g ? Array.from({ length: 10 }, (_, i) => g.radiusTen + i * g.ringIncrement) : [])
  const numbers = $derived(
    g
      ? Array.from({ length: 8 }, (_, i) => {
          const ring = i + 1
          return { ring, r: (9 - ring) * g.ringIncrement + g.radiusTen + 0.5 * g.ringIncrement }
        })
      : [],
  )

  function fill(shot: Shot, index: number) {
    if (index !== shots.length - 1) return 'var(--shot)'
    const ring = Math.floor(shot.ringe)
    return ring >= 10 ? 'var(--shot-10)' : ring === 9 ? 'var(--shot-9)' : 'var(--shot-last)'
  }
</script>

{#if g}
  <svg
    viewBox="{-half} {-half} {2 * half} {2 * half}"
    class="target"
    role="img"
    aria-label="Scheibe {type} mit {shots.length} Schüssen"
  >
    <rect x={-half} y={-half} width={2 * half} height={2 * half} fill="#fff" />
    <circle r={g.radiusBlack} fill="#111" />
    {#if g.radiusInnerTen > 0}
      <circle r={g.radiusInnerTen} fill="none" stroke="#fff" stroke-width={width / 450} />
    {/if}
    {#each rings as r}
      <circle {r} fill="none" stroke={r < g.radiusBlack ? '#fff' : '#111'} stroke-width={width / 450} />
    {/each}
    {#each numbers as n}
      {#each [[0, n.r], [0, -n.r], [n.r, 0], [-n.r, 0]] as [x, y]}
        <text
          {x}
          {y}
          font-size={g.ringIncrement * 0.45}
          fill={n.r < g.radiusBlack ? '#fff' : '#111'}
          text-anchor="middle"
          dominant-baseline="central">{n.ring}</text
        >
      {/each}
    {/each}
    {#each shots as shot, i}
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <g
        class="shot"
        class:selected={i === selected}
        role={onselect ? 'button' : undefined}
        tabindex={onselect ? 0 : undefined}
        onclick={() => onselect?.(i)}
        onkeydown={(e) => e.key === 'Enter' && onselect?.(i)}
        onmouseenter={() => onselect?.(i)}
      >
        <circle
          cx={shot.x * 100}
          cy={-shot.y * 100}
          r={g.radiusCalibre}
          fill={fill(shot, i)}
          stroke="#000"
          stroke-width={g.radiusCalibre / 12}
        />
        {#if numbered}
          <text
            x={shot.x * 100}
            y={-shot.y * 100}
            font-size={g.radiusCalibre}
            text-anchor="middle"
            dominant-baseline="central"
            fill="#000">{i + 1}</text
          >
        {/if}
      </g>
    {/each}
  </svg>
{/if}

<style>
  .target {
    width: 100%;
    height: auto;
    display: block;
    border-radius: var(--radius);
    background: #fff;
  }
  text {
    font-family: system-ui, sans-serif;
    pointer-events: none;
    user-select: none;
  }
  .shot {
    cursor: default;
    outline: none;
  }
  .shot[role='button'] {
    cursor: pointer;
  }
  .shot.selected circle {
    stroke: var(--accent);
    stroke-width: 30;
  }
</style>
