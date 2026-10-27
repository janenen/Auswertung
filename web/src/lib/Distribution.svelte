<script lang="ts">
  // Shot distribution: 2D kernel density (filled contours, one-hue ramp)
  // with shot positions, and box plots of x (top) and y (right).
  // Port of the seaborn JointGrid of the Python version.
  import { contourDensity } from 'd3-contour'
  import type { Shot } from './api'
  import { meta, fmt2 } from './meta.svelte'

  let { shots, type, filename = 'verteilung.svg' }: { shots: Shot[]; type: string; filename?: string } = $props()

  // Layout in SVG units.
  const S = 420 // plot square
  const left = 46
  const top = 64 // includes the x box plot
  const right = 64 // y box plot
  const bottom = 40
  const W = left + S + right
  const H = top + S + bottom
  const band = 34 // box plot height

  // Sequential blue ramp (low → high density); dark mode uses its own steps.
  const lightRamp = ['#cde2fb', '#b7d3f6', '#9ec5f4', '#86b6ef', '#6da7ec', '#5598e7', '#3987e5', '#2a78d6', '#256abf', '#1c5cab', '#184f95', '#104281']
  const darkRamp = ['#104281', '#184f95', '#1c5cab', '#256abf', '#2a78d6', '#3987e5', '#5598e7', '#6da7ec', '#86b6ef', '#9ec5f4', '#b7d3f6', '#cde2fb']

  // Colors are resolved to literals so a downloaded SVG looks the same.
  let colors = $state({ ink: '#1c2430', muted: '#5f6b7a', grid: '#d6dce3', surface: '#ffffff', dark: false })
  $effect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const read = () => {
      const css = getComputedStyle(document.documentElement)
      const v = (name: string) => css.getPropertyValue(name).trim()
      colors = { ink: v('--text'), muted: v('--muted'), grid: v('--border'), surface: v('--surface'), dark: mq.matches }
    }
    read()
    mq.addEventListener('change', read)
    return () => mq.removeEventListener('change', read)
  })

  const g = $derived(meta.targets[type])
  const lim = $derived(
    Math.max(...shots.map((s) => Math.max(Math.abs(s.x), Math.abs(s.y))), 0.5) + (g ? g.radiusCalibre / 100 : 2),
  )
  const sx = (x: number) => ((x + lim) / (2 * lim)) * S
  const sy = (y: number) => ((lim - y) / (2 * lim)) * S
  const unit = $derived(S / (2 * lim)) // px per mm

  function quantile(sorted: number[], q: number) {
    const pos = (sorted.length - 1) * q
    const lo = Math.floor(pos)
    const hi = Math.ceil(pos)
    return sorted[lo] + (sorted[hi] - sorted[lo]) * (pos - lo)
  }

  interface Summary {
    min: number
    q1: number
    median: number
    mean: number
    q3: number
    max: number
    whiskerLo: number
    whiskerHi: number
    outliers: number[]
  }

  function summarize(values: number[]): Summary {
    const s = [...values].sort((a, b) => a - b)
    const q1 = quantile(s, 0.25)
    const q3 = quantile(s, 0.75)
    const iqr = q3 - q1
    const inside = s.filter((v) => v >= q1 - 1.5 * iqr && v <= q3 + 1.5 * iqr)
    return {
      min: s[0],
      q1,
      median: quantile(s, 0.5),
      mean: s.reduce((a, b) => a + b, 0) / s.length,
      q3,
      max: s[s.length - 1],
      whiskerLo: inside[0],
      whiskerHi: inside[inside.length - 1],
      outliers: s.filter((v) => v < q1 - 1.5 * iqr || v > q3 + 1.5 * iqr),
    }
  }

  const enough = $derived(shots.length >= 3)
  const xs = $derived(enough ? summarize(shots.map((s) => s.x)) : null)
  const ys = $derived(enough ? summarize(shots.map((s) => s.y)) : null)

  const contours = $derived.by(() => {
    if (!enough) return []
    // Scott's rule with seaborn's bw_adjust = 0.5.
    const sd = (v: number[]) => {
      const m = v.reduce((a, b) => a + b, 0) / v.length
      return Math.sqrt(v.reduce((a, b) => a + (b - m) ** 2, 0) / (v.length - 1))
    }
    const sigma = (sd(shots.map((s) => s.x)) + sd(shots.map((s) => s.y))) / 2
    const bandwidth = Math.max(0.5 * Math.pow(shots.length, -1 / 6) * sigma * unit, 6)
    const result = contourDensity<Shot>()
      .x((s) => sx(s.x))
      .y((s) => sy(s.y))
      .size([S, S])
      .cellSize(2)
      .bandwidth(bandwidth)
      .thresholds(8)(shots)
    return result.slice(1) // drop the faint outermost level (seaborn thresh)
  })

  const ramp = $derived(colors.dark ? darkRamp : lightRamp)
  const levelColor = (i: number, n: number) => ramp[Math.round((n <= 1 ? 1 : i / (n - 1)) * (ramp.length - 1))]

  function path(geometry: { coordinates: number[][][][] }) {
    return geometry.coordinates
      .flatMap((polygon) => polygon.map((ring) => 'M' + ring.map(([x, y]) => `${x},${y}`).join('L') + 'Z'))
      .join('')
  }

  const ticks = $derived.by(() => {
    const step = [0.5, 1, 2, 2.5, 5, 10, 20, 50].find((s) => (2 * lim) / s <= 8) ?? 100
    const out: number[] = []
    for (let t = Math.ceil(-lim / step) * step; t <= lim + 1e-9; t += step) out.push(Math.round(t * 100) / 100)
    return out
  })

  const rings = $derived(
    g
      ? Array.from({ length: 10 }, (_, i) => (g.radiusTen + i * g.ringIncrement) / 100).filter((r) => r < lim * 1.42)
      : [],
  )

  // Tooltip
  let tip = $state<{ x: number; y: number; lines: string[] } | null>(null)
  let wrap: HTMLDivElement | undefined = $state()
  function show(e: MouseEvent | FocusEvent, lines: string[]) {
    const box = wrap!.getBoundingClientRect()
    const target = (e.currentTarget as Element).getBoundingClientRect()
    tip = { x: target.left + target.width / 2 - box.left, y: target.top - box.top, lines }
  }
  const boxLines = (axis: string, s: Summary) => [
    `${axis}`,
    `Median ${fmt2(s.median)} mm · Mittelwert ${fmt2(s.mean)} mm`,
    `Quartile ${fmt2(s.q1)} – ${fmt2(s.q3)} mm`,
    `Min ${fmt2(s.min)} · Max ${fmt2(s.max)} mm`,
  ]

  let showTable = $state(false)
  let svg: SVGSVGElement | undefined = $state()

  function download() {
    const data = new XMLSerializer().serializeToString(svg!)
    const url = URL.createObjectURL(new Blob([data], { type: 'image/svg+xml' }))
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }
</script>

{#if !enough}
  <p class="muted">Für die Verteilung werden mindestens 3 Schüsse benötigt.</p>
{:else if xs && ys}
  <div class="toolbar">
    <ul class="legend" aria-label="Legende">
      <li>
        <span class="gradient" style="background: linear-gradient(90deg, {ramp[0]}, {ramp[ramp.length - 1]})"></span>
        Schussdichte niedrig → hoch
      </li>
      <li><span class="dot" style="background: {colors.ink}"></span> Schuss</li>
      <li><span class="line" style="border-color: {colors.ink}"></span> Median</li>
      <li><span class="line dashed" style="border-color: {colors.ink}"></span> Mittelwert</li>
    </ul>
    <span class="spacer"></span>
    <button onclick={() => (showTable = !showTable)}>{showTable ? 'Diagramm' : 'Tabelle'}</button>
    <button onclick={download} disabled={showTable}>SVG speichern</button>
  </div>

  {#if showTable}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Achse</th>
            <th class="num">Min</th>
            <th class="num">Q1</th>
            <th class="num">Median</th>
            <th class="num">Mittelwert</th>
            <th class="num">Q3</th>
            <th class="num">Max</th>
          </tr>
        </thead>
        <tbody>
          {#each [['x (rechts/links)', xs], ['y (hoch/tief)', ys]] as const as [label, s]}
            <tr>
              <td>{label}</td>
              {#each [s.min, s.q1, s.median, s.mean, s.q3, s.max] as v}<td class="num">{fmt2(v)}</td>{/each}
            </tr>
          {/each}
        </tbody>
      </table>
      <p class="muted small">Alle Werte in mm, {shots.length} Schüsse.</p>
    </div>
  {:else}
    <div class="chart" bind:this={wrap}>
      <svg
        bind:this={svg}
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 {W} {H}"
        role="img"
        aria-label="Schussverteilung mit {shots.length} Schüssen"
        font-family="system-ui, sans-serif"
      >
        <rect width={W} height={H} fill={colors.surface} />

        <g transform="translate({left},{top})">
          <!-- grid and target rings for orientation -->
          {#each ticks as t}
            <line x1={sx(t)} x2={sx(t)} y1="0" y2={S} stroke={colors.grid} stroke-width="1" />
            <line y1={sy(t)} y2={sy(t)} x1="0" x2={S} stroke={colors.grid} stroke-width="1" />
          {/each}
          <clipPath id="plot-clip"><rect width={S} height={S} /></clipPath>
          <g clip-path="url(#plot-clip)">
            {#each contours as c, i}
              <path d={path(c)} fill={levelColor(i, contours.length)} />
            {/each}
            {#each rings as r}
              <circle cx={sx(0)} cy={sy(0)} r={r * unit} fill="none" stroke={colors.muted} stroke-opacity="0.45" stroke-dasharray="3 4" />
            {/each}
            {#each shots as s, i}
              <circle cx={sx(s.x)} cy={sy(s.y)} r="4" fill={colors.ink} stroke={colors.surface} stroke-width="2" />
              <circle
                class="hit"
                cx={sx(s.x)}
                cy={sy(s.y)}
                r="10"
                fill="transparent"
                role="img"
                aria-label="Schuss {i + 1}"
                onmouseenter={(e) => show(e, [`Schuss ${i + 1}: ${s.ringe.toFixed(1)}`, `x ${fmt2(s.x)} mm · y ${fmt2(s.y)} mm`, `Teiler ${s.teiler.toFixed(1)}`])}
                onmouseleave={() => (tip = null)}
              />
            {/each}
          </g>
          <rect width={S} height={S} fill="none" stroke={colors.grid} />

          <!-- axes -->
          {#each ticks as t}
            <text x={sx(t)} y={S + 16} text-anchor="middle" font-size="11" fill={colors.muted}>{t}</text>
            <text x="-8" y={sy(t)} text-anchor="end" dominant-baseline="central" font-size="11" fill={colors.muted}>{t}</text>
          {/each}
          <text x={S / 2} y={S + 34} text-anchor="middle" font-size="12" fill={colors.muted}>x in mm (rechts / links)</text>
          <text transform="translate({-36},{S / 2}) rotate(-90)" text-anchor="middle" font-size="12" fill={colors.muted}
            >y in mm (hoch / tief)</text
          >
        </g>

        <!-- x box plot (top) -->
        {#snippet boxplot(s: Summary, horizontal: boolean, label: string)}
          {@const pos = (v: number) => (horizontal ? sx(v) : sy(v))}
          {@const mid = band / 2}
          {@const a = (v: number, off: number) => (horizontal ? { x: pos(v), y: off } : { x: off, y: pos(v) })}
          <g
            class="box"
            role="img"
            aria-label="{label}: Median {fmt2(s.median)} mm"
            onmouseenter={(e) => show(e, boxLines(label, s))}
            onmouseleave={() => (tip = null)}
          >
            <rect
              x={horizontal ? pos(s.whiskerLo) - 6 : 0}
              y={horizontal ? 0 : pos(s.whiskerHi) - 6}
              width={horizontal ? pos(s.whiskerHi) - pos(s.whiskerLo) + 12 : band}
              height={horizontal ? band : pos(s.whiskerLo) - pos(s.whiskerHi) + 12}
              fill="transparent"
            />
            <!-- whiskers -->
            <line x1={a(s.whiskerLo, mid).x} y1={a(s.whiskerLo, mid).y} x2={a(s.q1, mid).x} y2={a(s.q1, mid).y} stroke={colors.muted} stroke-width="1.5" />
            <line x1={a(s.q3, mid).x} y1={a(s.q3, mid).y} x2={a(s.whiskerHi, mid).x} y2={a(s.whiskerHi, mid).y} stroke={colors.muted} stroke-width="1.5" />
            {#each [s.whiskerLo, s.whiskerHi] as v}
              <line x1={a(v, mid - 7).x} y1={a(v, mid - 7).y} x2={a(v, mid + 7).x} y2={a(v, mid + 7).y} stroke={colors.muted} stroke-width="1.5" />
            {/each}
            <!-- box -->
            <rect
              x={horizontal ? pos(s.q1) : 6}
              y={horizontal ? 6 : pos(s.q3)}
              width={horizontal ? Math.max(pos(s.q3) - pos(s.q1), 1) : band - 12}
              height={horizontal ? band - 12 : Math.max(pos(s.q1) - pos(s.q3), 1)}
              rx="4"
              fill={ramp[3]}
              stroke={colors.surface}
              stroke-width="2"
            />
            <line x1={a(s.median, 6).x} y1={a(s.median, 6).y} x2={a(s.median, band - 6).x} y2={a(s.median, band - 6).y} stroke={colors.ink} stroke-width="2" />
            <line x1={a(s.mean, 2).x} y1={a(s.mean, 2).y} x2={a(s.mean, band - 2).x} y2={a(s.mean, band - 2).y} stroke={colors.ink} stroke-width="2" stroke-dasharray="4 3" />
            {#each s.outliers as v}
              <circle cx={a(v, mid).x} cy={a(v, mid).y} r="3.5" fill="none" stroke={colors.muted} stroke-width="1.5" />
            {/each}
          </g>
        {/snippet}
        <g transform="translate({left},{top - band - 14})">{@render boxplot(xs, true, 'x (rechts / links)')}</g>
        <g transform="translate({left + S + 14},{top})">{@render boxplot(ys, false, 'y (hoch / tief)')}</g>
      </svg>
      {#if tip}
        <div class="tooltip" style="left: {tip.x}px; top: {tip.y}px">
          {#each tip.lines as line, i}<div class:first={i === 0}>{line}</div>{/each}
        </div>
      {/if}
    </div>
  {/if}
{/if}

<style>
  .toolbar {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
    margin-bottom: 0.5rem;
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem 1rem;
    list-style: none;
    padding: 0;
    margin: 0;
    font-size: 0.85rem;
    color: var(--muted);
  }
  .legend li {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }
  .gradient {
    width: 3rem;
    height: 0.6rem;
    border-radius: 2px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .line {
    width: 1.2rem;
    border-top: 2px solid;
  }
  .line.dashed {
    border-top-style: dashed;
  }
  .chart {
    position: relative;
    max-width: 40rem;
  }
  svg {
    width: 100%;
    height: auto;
    display: block;
  }
  .hit,
  .box {
    cursor: default;
  }
  .tooltip {
    position: absolute;
    transform: translate(-50%, calc(-100% - 8px));
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.35rem 0.6rem;
    font-size: 0.8rem;
    white-space: nowrap;
    pointer-events: none;
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.12);
    font-variant-numeric: tabular-nums;
  }
  .tooltip .first {
    font-weight: 600;
  }
  .small {
    font-size: 0.85rem;
  }
</style>
