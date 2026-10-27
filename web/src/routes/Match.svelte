<script lang="ts">
  import { api, type Competition, type MatchView } from '../lib/api'
  import { fmt1, fmt2, result, ringValue } from '../lib/meta.svelte'
  import { href } from '../lib/router.svelte'
  import Target from '../lib/Target.svelte'
  import Distribution from '../lib/Distribution.svelte'

  let { id, query = {} }: { id: string; query?: Record<string, string> } = $props()

  let view = $state<MatchView | null>(null)
  let competition = $state<Competition | null>(null)
  let error = $state('')
  let series = $state(-1) // -1: whole match
  let selected = $state(-1)

  $effect(() => {
    api
      .match(id)
      .then((v) => (view = v))
      .catch((e) => (error = e.message))
    if (query.competition) {
      api
        .competition(query.competition)
        .then((v) => (competition = v.competition))
        .catch((e) => (error = e.message))
    }
  })

  const decimal = $derived(competition ? competition.decimal : query.decimal === 'true')
  const title = $derived(competition?.name ?? query.title ?? 'Schnellauswertung')
  const shots = $derived(
    view ? (series < 0 ? view.match.shots : view.match.shots.slice(series * 10, series * 10 + 10)) : [],
  )
  const stats = $derived(view ? (series < 0 ? view.total : view.series[series]) : null)
  const columns = $derived(
    view ? Array.from({ length: view.series.length }, (_, k) => view!.match.shots.slice(k * 10, k * 10 + 10)) : [],
  )
  const shot = $derived(selected >= 0 ? shots[selected] : undefined)
  const reportQuery = $derived<Record<string, string>>(
    query.competition ? { competition: query.competition } : { title, decimal: String(decimal) },
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
  <p class="error">{error}</p>
{:else if view && stats}
  <div class="actions head">
    <div>
      <h1>{view.shooter}</h1>
      <p class="muted sub">
        {title} · {view.match.date} · Scheibe {view.match.type_of_target}
        {#if view.club}· {view.club}{/if}
        {#if view.team}· {view.team}{/if}
        {#if view.match.active}<span class="badge active">unvollständig</span>{/if}
      </p>
    </div>
    <span class="spacer"></span>
    {#if query.competition}
      <a class="button" href={href(`/competitions/${query.competition}`)}>Zum Wettkampf</a>
      {#if competition?.active}
        <a class="button" href={href(`/competitions/${query.competition}/entry`)}>+ Nächster Beitrag</a>
      {/if}
    {:else}
      <a class="button" href={href('/')}>Zur Übersicht</a>
    {/if}
  </div>

  <div class="layout">
    <section class="card target-card">
      <div class="series-nav">
        <button onclick={() => step(-1)} aria-label="Vorherige Serie">‹</button>
        <strong>{series < 0 ? 'Gesamt' : `Serie ${series + 1}`}</strong>
        <button onclick={() => step(1)} aria-label="Nächste Serie">›</button>
      </div>
      <Target type={view.match.type_of_target} {shots} zoom numbered {selected} onselect={(i) => (selected = i)} />
      <p class="shot-detail">
        {#if shot}
          Schuss {(series < 0 ? 0 : series * 10) + selected + 1}: <strong>{ringValue(shot.ringe, decimal)}</strong>
          <span class="muted">({fmt1(shot.ringe)})</span> · Teiler {fmt1(shot.teiler)} · x {fmt2(shot.x)} · y {fmt2(shot.y)}
        {:else}
          <span class="muted">Schuss anklicken für Details</span>
        {/if}
      </p>
    </section>

    <div class="side">
      <section class="card stats">
        <h2>{series < 0 ? 'Gesamtergebnis' : `Serie ${series + 1}`}</h2>
        <dl>
          <dt>Ergebnis</dt>
          <dd class="result">{result(stats, decimal)}&nbsp;<small class="muted">({result(stats, !decimal)})</small></dd>
          <dt>Schnitt</dt>
          <dd>{fmt2((decimal ? stats.summe : stats.summeGanz) / stats.count)}</dd>
          <dt>Bester Teiler</dt>
          <dd>{fmt1(stats.best.teiler)}</dd>
          <dt>Ablage R/L</dt>
          <dd>{fmt1(stats.ablageRL)}</dd>
          <dt>Ablage H/T</dt>
          <dd>{fmt1(stats.ablageHT)}</dd>
        </dl>
        <div class="rings">
          {#each stats.rings.slice(0, 6) as n, i}
            <span><small class="muted">{10 - i}er</small><strong>{n}</strong></span>
          {/each}
        </div>
      </section>

      <section class="card">
        <h2>Schüsse</h2>
        <div class="columns">
          {#each columns as col, k}
            <button class="column" class:current={k === series} onclick={() => ((series = k), (selected = -1))}>
              {#each col as s, j}
                <span class:hl={k === series && j === selected}>{ringValue(s.ringe, decimal)}</span>
              {/each}
              <strong class="sum">{result(view.series[k], decimal)}</strong>
            </button>
          {/each}
        </div>
      </section>

      <section class="card">
        <h2>Export</h2>
        <div class="actions">
          <a class="button primary" href="/api/matches/{id}/report.pdf?{qs(reportQuery)}" target="_blank">PDF erzeugen</a>
          <a class="button" href="/api/matches/{id}/export.csv">CSV erzeugen</a>
          <a class="button" href="/api/matches/{id}/qr.png" target="_blank">QR-Code erzeugen</a>
        </div>
        <p class="muted small">Dateien werden zusätzlich im Ausgabeordner gespeichert.</p>
      </section>
    </div>
  </div>

  <section class="card distribution">
    <h2>Schussverteilung {series < 0 ? 'gesamt' : `Serie ${series + 1}`}</h2>
    <Distribution
      {shots}
      type={view.match.type_of_target}
      filename="{view.shooter.replaceAll(' ', '_')}_{view.match.date}_verteilung.svg"
    />
  </section>
{:else}
  <p class="muted">Lade…</p>
{/if}

<style>
  .distribution {
    margin-top: 1rem;
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
