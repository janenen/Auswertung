<script lang="ts">
  // Shot distribution: filled 2D kernel density with target rings and box
  // plots of x (top) and y (right), next to a table of the box plot values.
  // Recreates the seaborn JointGrid of the Python version (kdeplot fill,
  // bw_adjust=0.5, boxplot with median and mean line) in the app's colors.
  import { contours as d3contours } from 'd3-contour'
  import type { Shot } from './api'
  import { meta, fmt2 } from './meta.svelte'
  import { t } from './i18n.svelte'

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

  // Theme colors, resolved to literals so a downloaded SVG looks the same.
  const fallback = { surface: '#ffffff', text: '#1c2430', muted: '#5f6b7a', border: '#d6dce3', accent: '#1f6feb', danger: '#c62828' }
  let theme = $state(fallback)
  $effect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const read = () => {
      const css = getComputedStyle(document.documentElement)
      const v = (name: keyof typeof fallback) => {
        const raw = css.getPropertyValue(`--${name}`).trim()
        return parseColor(raw) ? raw : fallback[name]
      }
      theme = { surface: v('surface'), text: v('text'), muted: v('muted'), border: v('border'), accent: v('accent'), danger: v('danger') }
    }
    read()
    mq.addEventListener('change', read)
    return () => mq.removeEventListener('change', read)
  })

  // Colors are interpolated in OKLab so the density levels get even steps
  // of lightness, which keeps groupings of shots distinguishable.
  type Lab = [number, number, number]
  function parseColor(css: string): [number, number, number] | null {
    const s = css.trim()
    const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(s)?.[1]
    if (hex) {
      const full = hex.length === 3 ? [...hex].map((c) => c + c).join('') : hex
      return [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255) as [number, number, number]
    }
    const rgb = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)/.exec(s)
    return rgb ? [Number(rgb[1]) / 255, Number(rgb[2]) / 255, Number(rgb[3]) / 255] : null
  }
  function toLab(css: string): Lab {
    const [r, g, b] = (parseColor(css) ?? [0, 0, 0]).map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
    const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
    const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
    const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
    return [
      0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
      1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
      0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
    ]
  }
  function fromLab([L, A, B]: Lab): string {
    const l = (L + 0.3963377774 * A + 0.2158037573 * B) ** 3
    const m = (L - 0.1055613458 * A - 0.0638541728 * B) ** 3
    const s = (L - 0.0894841775 * A - 1.291485548 * B) ** 3
    const rgb = [
      4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
      -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
      -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
    ]
    const hex = (c: number) => {
      const v = c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055
      return Math.round(Math.min(1, Math.max(0, v)) * 255)
        .toString(16)
        .padStart(2, '0')
    }
    return '#' + rgb.map(hex).join('')
  }
  const mix = (a: Lab, b: Lab, f: number): Lab => [a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f, a[2] + (b[2] - a[2]) * f]

  // Sequential ramp: a faint accent tint for low density, the accent in the
  // middle and a deep (light theme) or bright (dark theme) accent for the
  // densest groups – the text color gives the direction in both themes.
  const ramp = $derived.by(() => {
    const surface = toLab(theme.surface)
    const accent = toLab(theme.accent)
    const low = mix(surface, accent, 0.3)
    const high = mix(accent, toLab(theme.text), 0.6)
    return (x: number) => fromLab(x < 0.5 ? mix(low, accent, x * 2) : mix(accent, high, (x - 0.5) * 2))
  })
  const legendRamp = $derived(Array.from({ length: 9 }, (_, i) => ramp(i / 8)).join(', '))
  const boxFill = $derived(fromLab(mix(toLab(theme.surface), toLab(theme.accent), 0.35)))

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

  // Density as scipy's gaussian_kde: full covariance scaled by Scott's
  // factor n^(-1/6) and seaborn's bw_adjust 0.5, evaluated on a 200² grid
  // reaching 3 bandwidths beyond the data (seaborn's cut=3).
  const N = 200
  const density = $derived.by(() => {
    if (!enough) return null
    const n = shots.length
    const mx = shots.reduce((a, s) => a + s.x, 0) / n
    const my = shots.reduce((a, s) => a + s.y, 0) / n
    let cxx = 0
    let cyy = 0
    let cxy = 0
    for (const s of shots) {
      cxx += (s.x - mx) ** 2
      cyy += (s.y - my) ** 2
      cxy += (s.x - mx) * (s.y - my)
    }
    const f2 = (0.5 * Math.pow(n, -1 / 6)) ** 2
    // A minimum spread keeps identical shots from collapsing the kernel.
    const floor = 0.01
    let a = Math.max((cxx / (n - 1)) * f2, floor)
    let d = Math.max((cyy / (n - 1)) * f2, floor)
    let b = (cxy / (n - 1)) * f2
    let det = a * d - b * b
    if (det <= floor * floor) {
      b = 0
      det = a * d
    }
    const ia = d / det
    const ib = -b / det
    const id = a / det
    const reach = 3 * Math.sqrt(Math.max(a, d))
    const L = Math.max(lim, ...shots.map((s) => Math.max(Math.abs(s.x), Math.abs(s.y)) + reach))
    const cell = (2 * L) / N
    const values = new Float64Array(N * N)
    for (let j = 0; j < N; j++) {
      const y = L - (j + 0.5) * cell
      for (let i = 0; i < N; i++) {
        const x = -L + (i + 0.5) * cell
        let v = 0
        for (const s of shots) {
          const dx = x - s.x
          const dy = y - s.y
          v += Math.exp(-0.5 * (ia * dx * dx + 2 * ib * dx * dy + id * dy * dy))
        }
        values[j * N + i] = v
      }
    }
    return { values, L }
  })

  // seaborn's levels=10, thresh=0.05: density levels enclosing 95 % … 0 %
  // of the probability mass, filled from the lowest to the highest color.
  const layers = $derived.by(() => {
    if (!density) return []
    const sorted = Array.from(density.values).sort((p, q) => q - p)
    const total = sorted.reduce((p, q) => p + q, 0)
    const cum: number[] = []
    let acc = 0
    for (const v of sorted) cum.push((acc += v) / total)
    const levels = Array.from({ length: 10 }, (_, k) => {
      const iso = 0.05 + (k * 0.95) / 9
      const idx = cum.findIndex((c) => c >= 1 - iso)
      return sorted[Math.max(idx, 0)]
    })
    const bands = levels.length - 1
    return d3contours()
      .size([N, N])
      .thresholds(levels.slice(0, bands))(Array.from(density.values))
      .map((c, k) => ({ d: path(c), color: ramp(bands > 1 ? k / (bands - 1) : 1) }))
  })

  const rings = $derived(
    g
      ? Array.from({ length: 10 }, (_, i) => (g.radiusTen + i * g.ringIncrement) / 100).filter((r) => r < lim * 1.42)
      : [],
  )

  // Grid coordinates [0, N] → plot pixels.
  const gridTransform = $derived(
    density ? `translate(${sx(-density.L)},${sy(density.L)}) scale(${((2 * density.L) / N) * unit})` : '',
  )

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

  // Tooltip
  let tip = $state<{ x: number; y: number; lines: string[] } | null>(null)
  let wrap: HTMLDivElement | undefined = $state()
  function show(e: MouseEvent | FocusEvent, lines: string[]) {
    const box = wrap!.getBoundingClientRect()
    const target = (e.currentTarget as Element).getBoundingClientRect()
    tip = { x: target.left + target.width / 2 - box.left, y: target.top - box.top, lines }
  }
  const boxLines = (axis: string, s: Summary) => [
    axis,
    t('Median {a} mm · Mittelwert {b} mm', { a: fmt2(s.median), b: fmt2(s.mean) }),
    t('Quartile {a} – {b} mm', { a: fmt2(s.q1), b: fmt2(s.q3) }),
    t('Min {a} · Max {b} mm', { a: fmt2(s.min), b: fmt2(s.max) }),
  ]

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
  <p class="muted">{t('Für die Verteilung werden mindestens 3 Schüsse benötigt.')}</p>
{:else if xs && ys}
  <div class="toolbar">
    <ul class="legend" aria-label={t('Legende')}>
      <li>
        <span class="gradient" style="background: linear-gradient(90deg, {legendRamp})"></span>
        {t('Schussdichte niedrig → hoch')}
      </li>
      <li><span class="line dotted" style="border-color: {theme.muted}"></span> {t('Ringe')}</li>
      <li><span class="line" style="border-color: {theme.text}"></span> {t('Median')}</li>
      <li><span class="line dashed" style="border-color: {theme.danger}"></span> {t('Mittelwert')}</li>
    </ul>
    <span class="spacer"></span>
    <button onclick={download}>{t('SVG speichern')}</button>
  </div>

  <div class="split">
    <div class="chart" bind:this={wrap}>
      <svg
        bind:this={svg}
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 {W} {H}"
        role="img"
        aria-label={t('Schussverteilung mit {n} Schüssen', { n: shots.length })}
        font-family="system-ui, sans-serif"
      >
        <rect width={W} height={H} fill={theme.surface} />

        <g transform="translate({left},{top})">
          <clipPath id="plot-clip"><rect width={S} height={S} /></clipPath>
          <g clip-path="url(#plot-clip)">
            <g transform={gridTransform}>
              {#each layers as layer}
                <path d={layer.d} fill={layer.color} />
              {/each}
            </g>
            {#each rings as r}
              <circle cx={sx(0)} cy={sy(0)} r={r * unit} fill="none" stroke={theme.muted} stroke-opacity="0.6" stroke-dasharray="3 4" />
            {/each}
          </g>
          <rect width={S} height={S} fill="none" stroke={theme.border} />

          <!-- axes -->
          {#each ticks as tick}
            <line x1={sx(tick)} x2={sx(tick)} y1={S} y2={S + 5} stroke={theme.border} />
            <line x1="-5" x2="0" y1={sy(tick)} y2={sy(tick)} stroke={theme.border} />
            <text x={sx(tick)} y={S + 18} text-anchor="middle" font-size="11" fill={theme.muted}>{tick}</text>
            <text x="-8" y={sy(tick)} text-anchor="end" dominant-baseline="central" font-size="11" fill={theme.muted}>{tick}</text>
          {/each}
          <text x={S / 2} y={S + 36} text-anchor="middle" font-size="12" fill={theme.muted}>{t('x in mm (rechts / links)')}</text>
          <text transform="translate({-36},{S / 2}) rotate(-90)" text-anchor="middle" font-size="12" fill={theme.muted}
            >{t('y in mm (hoch / tief)')}</text
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
            <line x1={a(s.whiskerLo, mid).x} y1={a(s.whiskerLo, mid).y} x2={a(s.q1, mid).x} y2={a(s.q1, mid).y} stroke={theme.muted} stroke-width="1.5" />
            <line x1={a(s.q3, mid).x} y1={a(s.q3, mid).y} x2={a(s.whiskerHi, mid).x} y2={a(s.whiskerHi, mid).y} stroke={theme.muted} stroke-width="1.5" />
            {#each [s.whiskerLo, s.whiskerHi] as v}
              <line x1={a(v, mid - 7).x} y1={a(v, mid - 7).y} x2={a(v, mid + 7).x} y2={a(v, mid + 7).y} stroke={theme.muted} stroke-width="1.5" />
            {/each}
            <!-- box -->
            <rect
              x={horizontal ? pos(s.q1) : 6}
              y={horizontal ? 6 : pos(s.q3)}
              width={horizontal ? Math.max(pos(s.q3) - pos(s.q1), 1) : band - 12}
              height={horizontal ? band - 12 : Math.max(pos(s.q1) - pos(s.q3), 1)}
              rx="3"
              fill={boxFill}
              stroke={theme.muted}
              stroke-width="1.5"
            />
            <line x1={a(s.median, 6).x} y1={a(s.median, 6).y} x2={a(s.median, band - 6).x} y2={a(s.median, band - 6).y} stroke={theme.text} stroke-width="2" />
            <line x1={a(s.mean, 3).x} y1={a(s.mean, 3).y} x2={a(s.mean, band - 3).x} y2={a(s.mean, band - 3).y} stroke={theme.danger} stroke-width="2" stroke-dasharray="4 3" />
            {#each s.outliers as v}
              {@const p = a(v, mid)}
              <path d="M{p.x},{p.y - 4}L{p.x + 3},{p.y}L{p.x},{p.y + 4}L{p.x - 3},{p.y}Z" fill={theme.muted} />
            {/each}
          </g>
        {/snippet}
        <g transform="translate({left},{top - band - 14})">{@render boxplot(xs, true, t('x (rechts / links)'))}</g>
        <g transform="translate({left + S + 14},{top})">{@render boxplot(ys, false, t('y (hoch / tief)'))}</g>
      </svg>
      {#if tip}
        <div class="tooltip" style="left: {tip.x}px; top: {tip.y}px">
          {#each tip.lines as line, i}<div class:first={i === 0}>{line}</div>{/each}
        </div>
      {/if}
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th></th>
            <th class="num">{t('x (R/L)')}</th>
            <th class="num">{t('y (H/T)')}</th>
          </tr>
        </thead>
        <tbody>
          {#each [['Min', 'min'], ['Q1', 'q1'], ['Median', 'median'], ['Mittelwert', 'mean'], ['Q3', 'q3'], ['Max', 'max']] as const as [label, key]}
            <tr>
              <td>{t(label)}</td>
              <td class="num">{fmt2(xs[key])}</td>
              <td class="num">{fmt2(ys[key])}</td>
            </tr>
          {/each}
        </tbody>
      </table>
      <p class="muted small">{t('Alle Werte in mm, {n} Schüsse.', { n: shots.length })}</p>
    </div>
  </div>
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
  .line {
    width: 1.2rem;
    border-top: 2px solid;
  }
  .line.dashed {
    border-top-style: dashed;
  }
  .line.dotted {
    border-top: 1px dashed;
  }
  .split {
    display: flex;
    flex-wrap: wrap;
    gap: 1.5rem;
    align-items: flex-start;
  }
  .chart {
    position: relative;
    flex: 1 1 22rem;
    max-width: 40rem;
  }
  .table-wrap {
    flex: 0 1 auto;
  }
  .table-wrap th {
    white-space: nowrap;
  }
  svg {
    width: 100%;
    height: auto;
    display: block;
    border-radius: var(--radius);
  }
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
