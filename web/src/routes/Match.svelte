<script lang="ts">
  import { api, type Competition, type MatchView } from '../lib/api'
  import { fmt1, fmt2, result, ringValue } from '../lib/meta.svelte'
  import { href } from '../lib/router.svelte'
  import Target from '../lib/Target.svelte'
  import Distribution from '../lib/Distribution.svelte'
  import { t, tx } from '../lib/i18n.svelte'

  let { id, query = {} }: { id: string; query?: Record<string, string> } = $props()

  let view = $state<MatchView | null>(null)
  let competition = $state<Competition | null>(null)
  let error = $state('')
  let series = $state(-1) // -1: whole match
  let selected = $state(-1)
  let qrFailed = $state(false)

  // The competition comes from the query or the match itself; a quick
  // analysis has its title and scoring stored with the match.
  const competitionId = $derived(query.competition || view?.match.competition || '')
  $effect(() => {
    api
      .match(id)
      .then(async (v) => {
        view = v
        const cid = query.competition || v.match.competition
        if (cid) competition = (await api.competition(cid)).competition
      })
      .catch((e) => (error = e.message))
  })

  const decimal = $derived(
    competition ? competition.decimal : query.decimal ? query.decimal === 'true' : (view?.match.decimal ?? false),
  )
  const title = $derived(competition?.name ?? query.title ?? (view?.match.title || 'Schnellauswertung'))
  const shots = $derived(
    view ? (series < 0 ? view.match.shots : view.match.shots.slice(series * 10, series * 10 + 10)) : [],
  )
  const stats = $derived(view ? (series < 0 ? view.total : view.series[series]) : null)
  const columns = $derived(
    view ? Array.from({ length: view.series.length }, (_, k) => view!.match.shots.slice(k * 10, k * 10 + 10)) : [],
  )
  const shot = $derived(selected >= 0 ? shots[selected] : undefined)
  // Shots are numbered across the match; the target shows one series or all.
  const offset = $derived(series < 0 ? 0 : series * 10)
  const visible = (k: number) => series < 0 || k === series
  // Hovered ring count, series total or offset, highlighted on the target.
  let hover = $state<{ ring: number } | { series: number } | 'offset' | null>(null)
  const ring = (s: { ringe: number }) => Math.floor(s.ringe)
  const hoveredRing = $derived(hover !== null && hover !== 'offset' && 'ring' in hover ? hover.ring : -1)
  const highlight = $derived.by(() => {
    if (hover === null || hover === 'offset') return null
    const all = shots.map((_, i) => i)
    if ('ring' in hover) return all.filter((i) => ring(shots[i]) === hoveredRing)
    const k = hover.series
    return series < 0 ? all.slice(k * 10, k * 10 + 10) : k === series ? all : []
  })
  // CSV export is hidden for now; the endpoint stays available.
  const csvExport = false
  const reportQuery = $derived<Record<string, string>>(
    competitionId ? { competition: competitionId } : { title, decimal: String(decimal) },
  )
  const qs = (q: Record<string, string>) => new URLSearchParams(q).toString()

  function step(delta: number) {
    if (!view) return
    const n = view.series.length + 1
    series = ((series + 1 + delta + n) % n) - 1
    selected = -1
  }
</script>

{#if error}
  <p class="error">{tx(error)}</p>
{:else if view && stats}
  <div class="actions head">
    <div>
      <h1>{view.shooter}</h1>
      <p class="muted sub">
        {tx(title)} · {view.match.date} · {t('Scheibe {type}', { type: view.match.type_of_target })}
        {#if view.club}· {view.club}{/if}
        {#if view.team}· {view.team}{/if}
        {#if view.match.active}<span class="badge active">{t('unvollständig')}</span>{/if}
      </p>
    </div>
    <span class="spacer"></span>
    <a
      class="button primary"
      href="/api/matches/{id}/report.pdf?{qs(reportQuery)}"
      target="_blank"
      title={t('Wird zusätzlich im Ausgabeordner gespeichert')}>{t('PDF erzeugen')}</a
    >
    {#if csvExport}
      <a class="button" href="/api/matches/{id}/export.csv">{t('CSV erzeugen')}</a>
    {/if}
    {#if query.shooter}
      <a class="button" href={href(`/people/${query.shooter}`)}>{t('Zum Schützen')}</a>
    {/if}
    {#if competitionId}
      <a class="button" href={href(`/competitions/${competitionId}`)}>{t('Zum Wettkampf')}</a>
      {#if competition?.active && !query.shooter}
        <a class="button" href={href(`/competitions/${competitionId}/entry`)}>{t('+ Nächster Beitrag')}</a>
      {/if}
    {:else if !query.shooter}
      <a class="button" href={href('/')}>{t('Zur Übersicht')}</a>
    {/if}
  </div>

  <div class="layout">
    <section class="card target-card">
      <div class="series-nav">
        <button onclick={() => step(-1)} aria-label={t('Vorherige Serie')}>‹</button>
        <strong>{series < 0 ? t('Gesamt') : t('Serie {n}', { n: series + 1 })}</strong>
        <button onclick={() => step(1)} aria-label={t('Nächste Serie')}>›</button>
      </div>
      <Target
        type={view.match.type_of_target}
        {shots}
        zoom
        numbered
        {selected}
        onselect={(i) => (selected = i)}
        {highlight}
        offset={{ x: stats.ablageRL, y: stats.ablageHT }}
        offsetActive={hover === 'offset'}
        onoffsethover={(on) => (hover = on ? 'offset' : null)}
      />
      <p class="shot-detail">
        {#if shot}
          {t('Schuss {n}:', { n: offset + selected + 1 })} <strong>{ringValue(shot.ringe, decimal)}</strong>
          <span class="muted">({fmt1(shot.ringe)})</span> · {t('Teiler')} {fmt1(shot.teiler)} · x {fmt2(shot.x)} · y {fmt2(shot.y)}
        {:else}
          <span class="muted">{t('Schuss anklicken für Details')}</span>
        {/if}
      </p>
    </section>

    <div class="side">
      <section class="card stats">
        <h2>{series < 0 ? t('Gesamtergebnis') : t('Serie {n}', { n: series + 1 })}</h2>
        <dl>
          <dt>{t('Ergebnis')}</dt>
          <dd class="result">{result(stats, decimal)}&nbsp;<small class="muted">({result(stats, !decimal)})</small></dd>
          <dt>{t('Schnitt')}</dt>
          <dd>{fmt2((decimal ? stats.summe : stats.summeGanz) / stats.count)}</dd>
          <dt>{t('Bester Teiler')}</dt>
          <dd>{fmt1(stats.best.teiler)}</dd>
          {#each [{ label: t('Ablage R/L'), value: stats.ablageRL }, { label: t('Ablage H/T'), value: stats.ablageHT }] as row}
            <div
              class="row"
              class:hl-row={hover === 'offset'}
              role="presentation"
              onmouseenter={() => (hover = 'offset')}
              onmouseleave={() => (hover = null)}
            >
              <dt>{row.label}</dt>
              <dd>{fmt1(row.value)}</dd>
            </div>
          {/each}
        </dl>
        <div class="rings">
          {#each stats.rings.slice(0, 6) as n, i}
            <span
              class="ring-count"
              class:hl-row={hoveredRing === 10 - i}
              role="presentation"
              onmouseenter={() => (hover = { ring: 10 - i })}
              onmouseleave={() => (hover = null)}><small class="muted">{t('{n}er', { n: 10 - i })}</small><strong>{n}</strong></span
            >
          {/each}
        </div>
      </section>

      <section class="card">
        <h2>{t('Schüsse')}</h2>
        <div class="columns">
          {#each columns as col, k}
            <button class="column" class:current={k === series} onclick={() => ((series = k), (selected = -1))}>
              {#each col as s, j}
                <span
                  role="presentation"
                  class:hl={(selected >= 0 && k * 10 + j === offset + selected) || ring(s) === hoveredRing}
                  onmouseenter={() => visible(k) && (selected = k * 10 + j - offset)}
                  >{ringValue(s.ringe, decimal)}</span
                >
              {/each}
              <strong
                class="sum"
                role="presentation"
                onmouseenter={() => (hover = { series: k })}
                onmouseleave={() => (hover = null)}>{result(view.series[k], decimal)}</strong
              >
            </button>
          {/each}
        </div>
      </section>
    </div>
  </div>

  <section class="card below">
    <h2>{series < 0 ? t('Schussverteilung gesamt') : t('Schussverteilung Serie {n}', { n: series + 1 })}</h2>
    <Distribution
      {shots}
      type={view.match.type_of_target}
      filename="{view.shooter.replaceAll(' ', '_')}_{view.match.date}_verteilung.svg"
    />
  </section>

  <section class="card below qr">
    <h2>{t('QR-Code')}</h2>
    {#if qrFailed}
      <p class="muted">{t('Für dieses Ergebnis kann kein QR-Code erzeugt werden.')}</p>
    {:else}
      <!-- The enlarged copy opens beside the thumbnail, never under the pointer. -->
      <div class="qr-box">
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <img
          class="qr-thumb"
          tabindex="0"
          src="/api/matches/{id}/qr.png"
          alt={t('QR-Code mit allen Schüssen')}
          onerror={() => (qrFailed = true)}
        />
        <img class="qr-zoom" src="/api/matches/{id}/qr.png" alt="" aria-hidden="true" />
      </div>
      <p class="muted small">
        {t('Enthält alle Schüsse und kann unter „{machine}“ wieder eingelesen werden.', { machine: t('QR-Code aus Eingabe') })}
      </p>
    {/if}
  </section>
{:else}
  <p class="muted">{t('Lade…')}</p>
{/if}

<style>
  .below {
    margin-top: 1rem;
  }
  .qr-box {
    position: relative;
    width: 12rem;
    max-width: 100%;
  }
  .qr-box img {
    display: block;
    image-rendering: pixelated;
  }
  .qr-thumb {
    width: 100%;
    cursor: zoom-in;
  }
  .qr-zoom {
    position: absolute;
    left: calc(100% + 1rem);
    bottom: 0;
    width: 26rem;
    max-width: calc(100vw - 18rem);
    padding: 0.75rem;
    background: #fff;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.25);
    opacity: 0;
    transform: scale(0.96);
    transform-origin: left bottom;
    transition:
      opacity 0.12s,
      transform 0.12s;
    pointer-events: none;
    z-index: 10;
  }
  .qr-thumb:hover + .qr-zoom,
  .qr-thumb:focus-visible + .qr-zoom {
    opacity: 1;
    transform: none;
  }
  @media (max-width: 760px) {
    /* No room beside the thumbnail: open above it. */
    .qr-zoom {
      left: 0;
      bottom: calc(100% + 0.75rem);
      width: calc(100vw - 2rem);
      max-width: 26rem;
    }
  }
  .row {
    display: contents;
  }
  .row dt,
  .row dd,
  .ring-count,
  .column .sum {
    cursor: default;
  }
  .hl-row dt,
  .hl-row dd,
  .ring-count.hl-row,
  .ring-count.hl-row small {
    color: var(--accent);
  }
  .ring-count:hover,
  .column .sum:hover {
    color: var(--accent);
  }
  .head {
    align-items: flex-start;
    margin-bottom: 1rem;
  }
  .head h1 {
    margin: 0;
  }
  .sub {
    margin: 0.2rem 0 0;
  }
  .layout {
    display: grid;
    grid-template-columns: minmax(16rem, 30rem) 1fr;
    gap: 1rem;
    align-items: start;
  }
  @media (max-width: 760px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
  .target-card {
    padding: 0.75rem;
  }
  .series-nav {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
  }
  .series-nav button {
    padding: 0.2rem 0.8rem;
    font-size: 1.2rem;
  }
  .shot-detail {
    margin: 0.5rem 0 0;
    min-height: 1.5em;
    font-size: 0.9rem;
  }
  .side {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 0.25rem 1.25rem;
    margin: 0;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }
  .result {
    font-size: 1.6rem;
    font-weight: 700;
    line-height: 1.1;
  }
  .result small {
    font-size: 0.9rem;
    font-weight: 400;
  }
  .rings {
    display: flex;
    gap: 1.25rem;
    margin-top: 0.9rem;
  }
  .rings span {
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .columns {
    display: flex;
    gap: 0.4rem;
    overflow-x: auto;
  }
  .column {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 3.2rem;
    padding: 0.3rem;
    gap: 0.1rem;
    font-variant-numeric: tabular-nums;
  }
  .column.current {
    border-color: var(--accent);
  }
  .column .sum {
    border-top: 1px solid var(--border);
    margin-top: 0.2rem;
    padding-top: 0.2rem;
    width: 100%;
    text-align: center;
  }
  .hl {
    color: var(--accent);
    font-weight: 700;
  }
  .small {
    font-size: 0.85rem;
    margin: 0.6rem 0 0;
  }
</style>
