<script lang="ts">
  // Print view of a match, replacing the former PDF report: overview with
  // the total target, one block per series (shots with direction arrows and
  // the series target) and the shot distribution. The layout flows, so any
  // paper format chosen in the print dialog works; "Save as PDF" creates the
  // file. With ?print=1 the print dialog opens once the page is ready.
  import { tick } from 'svelte'
  import type { Shot } from '../lib/api'
  import { fmt1, fmt2, meta, result, ringValue } from '../lib/meta.svelte'
  import { href } from '../lib/router.svelte'
  import { loadMatch, matchTitle, sums, type MatchContext } from '../lib/match'
  import { t, tx } from '../lib/i18n.svelte'
  import Target from '../lib/Target.svelte'
  import ShotArrow from '../lib/ShotArrow.svelte'
  import Distribution from '../lib/Distribution.svelte'

  let { id, query = {} }: { id: string; query?: Record<string, string> } = $props()

  let ctx = $state<MatchContext | null>(null)
  let error = $state('')

  $effect(() => {
    loadMatch(id, query.competition)
      .then(async (c) => {
        ctx = c
        if (query.print) {
          await tick()
          // Let the targets and the distribution render first.
          setTimeout(() => window.print(), 300)
        }
      })
      .catch((e) => (error = e.message))
  })

  const info = $derived(ctx ? matchTitle(ctx, query) : { title: '', decimal: false })
  const m = $derived(ctx?.view.match)
  const geometry = $derived(m ? meta.targets[m.type_of_target] : undefined)
  const series = $derived(
    ctx ? ctx.view.series.map((stats, k) => ({ stats, shots: ctx!.view.match.shots.slice(k * 10, k * 10 + 10) })) : [],
  )
  // The browser suggests the title as file name when saving as PDF.
  const fileTitle = $derived.by(() => {
    if (!ctx || !m) return 'Auswertung'
    const [d, mo, y] = m.date.split('.')
    return `${ctx.view.shooter.replaceAll(' ', '_')}_${(y ?? '').slice(2)}${mo ?? ''}${d ?? ''}_${m.type_of_target}`
  })

  // Result in the scoring of the match, the other scoring in brackets.
  const both = (s: { summe: number; summeGanz: number }) => [result(s, info.decimal), result(s, !info.decimal)]
  const average = (s: { summe: number; summeGanz: number }, n: number) =>
    [info.decimal, !info.decimal].map((d) => fmt2((d ? s.summe : s.summeGanz) / n))
  const shotValues = (s: Shot) => [ringValue(s.ringe, info.decimal), ringValue(s.ringe, !info.decimal)]
</script>

<svelte:head><title>{fileTitle}</title></svelte:head>

{#if error}
  <p class="error">{tx(error)}</p>
{:else if ctx && m && geometry}
  {@const total = ctx.view.total}
  {@const [totalMain, totalOther] = both(total)}
  {@const [avg, avg2] = average(total, total.count)}
  <div class="actions toolbar no-print">
    <button class="primary" onclick={() => window.print()}>{t('Drucken / PDF')}</button>
    <a class="button" href={href(`/matches/${id}`, query.competition ? { competition: query.competition } : undefined)}
      >{t('Zurück')}</a
    >
    <span class="muted small">{t('Im Druckdialog Papierformat wählen oder „Als PDF speichern“.')}</span>
  </div>

  <article class="paper">
    <header class="overview block">
      <div class="facts">
        <h1>{tx(info.title)} {m.date}</h1>
        <dl>
          <dt>{t('Name')}</dt>
          <dd>{ctx.view.shooter}</dd>
          {#if ctx.view.club}<dt>{t('Verein')}</dt><dd>{ctx.view.club}</dd>{/if}
          {#if ctx.view.team}<dt>{t('Mannschaft')}</dt><dd>{ctx.view.team}</dd>{/if}
          <dt>{t('Scheibe')}</dt>
          <dd>{m.type_of_target}</dd>
        </dl>
        <dl class="figures">
          <dt>{t('Ergebnis')}</dt>
          <dd><strong class="big">{totalMain}</strong> <span class="muted">({totalOther})</span></dd>
          <dt>{t('Schnitt')}</dt>
          <dd><strong>{avg}</strong> <span class="muted">({avg2})</span></dd>
          <dt>{t('Bester Teiler')}</dt>
          <dd>{fmt1(total.best.teiler)}</dd>
          <dt>{t('Ablage R/L')}</dt>
          <dd>{fmt1(total.ablageRL)}</dd>
          <dt>{t('Ablage H/T')}</dt>
          <dd>{fmt1(total.ablageHT)}</dd>
        </dl>
        <div class="rings">
          {#each total.rings.slice(0, 6) as n, i}
            <span><small class="muted">{t('{n}er', { n: 10 - i })}</small><strong>{n}</strong></span>
          {/each}
        </div>
      </div>
      <div class="target total">
        <Target type={m.type_of_target} shots={m.shots} zoom offset={{ x: total.ablageRL, y: total.ablageHT }} />
      </div>
    </header>

    {#each series as s, k}
      {@const [main, other] = both(s.stats)}
      <section class="series block">
        <div class="series-facts">
          <strong>{t('Serie {n}', { n: k + 1 })}</strong>
          <dl>
            <dt>{t('Ergebnis')}</dt>
            <dd><strong>{main}</strong> <span class="muted">({other})</span></dd>
            <dt>{t('Bester Teiler')}</dt>
            <dd>{fmt1(s.stats.best.teiler)}</dd>
            <dt>{t('Ablage R/L')}</dt>
            <dd>{fmt1(s.stats.ablageRL)}</dd>
            <dt>{t('Ablage H/T')}</dt>
            <dd>{fmt1(s.stats.ablageHT)}</dd>
          </dl>
          {#if (k + 1) % 4 === 0 && k + 1 < series.length}
            {@const sofar = m.shots.slice(0, (k + 1) * 10)}
            {@const sum = sums(sofar)}
            <dl class="intermediate">
              <dt>{t('Zwischenergebnis')}</dt>
              <dd><strong>{result(sum, info.decimal)}</strong></dd>
              <dt>{t('Schnitt')}</dt>
              <dd>{average(sum, sofar.length)[0]}</dd>
            </dl>
          {/if}
        </div>
        <div class="shots-box">
          <ol class="shots">
            {#each s.shots as shot, j}
              {@const [first, second] = shotValues(shot)}
              <li>
                <small class="muted">{k * 10 + j + 1}</small>
                <strong>{first}</strong>
                <span class="muted">{second}</span>
                <ShotArrow {shot} {geometry} />
              </li>
            {/each}
          </ol>
        </div>
        <div class="target">
          <Target type={m.type_of_target} shots={s.shots} zoom offset={{ x: s.stats.ablageRL, y: s.stats.ablageHT }} />
        </div>
      </section>
    {/each}

    {#if m.shots.length >= 3}
      <section class="distribution block">
        <h2>{t('Schussverteilung gesamt')}</h2>
        <Distribution shots={m.shots} type={m.type_of_target} light />
      </section>
    {/if}
  </article>
{:else}
  <p class="muted">{t('Lade…')}</p>
{/if}

<style>
  .toolbar {
    margin-bottom: 1rem;
  }
  .small {
    font-size: 0.85rem;
  }
  /* A white sheet on screen in both themes; sizes are relative, so the
     layout follows the paper format. */
  .paper {
    --text: #1c2430;
    --muted: #5f6b7a;
    --border: #c9d0d8;
    --surface: #ffffff;
    --surface-2: #eef1f4;
    color: var(--text);
    background: #fff;
    padding: 1.5rem;
    border-radius: var(--radius);
    box-shadow: 0 1px 6px rgb(0 0 0 / 0.15);
    font-size: 0.95rem;
  }
  .block {
    break-inside: avoid;
    page-break-inside: avoid;
  }
  h1 {
    font-size: 1.4rem;
    margin: 0 0 0.75rem;
  }
  h2 {
    font-size: 1.05rem;
    margin: 0 0 0.5rem;
  }
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 0.15rem 1rem;
    margin: 0 0 0.6rem;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }
  .big {
    font-size: 1.4em;
  }
  .overview {
    display: flex;
    gap: 1.5rem;
    align-items: flex-start;
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--border);
  }
  .facts {
    flex: 1;
  }
  .target {
    flex: 0 0 auto;
  }
  .target.total {
    width: 38%;
    max-width: 75mm;
  }
  .rings {
    display: flex;
    gap: 1.1rem;
  }
  .rings span {
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .series {
    display: grid;
    grid-template-columns: minmax(9.5em, 22%) 1fr 18%;
    gap: 1rem;
    align-items: start;
    padding: 0.75rem 0;
    border-bottom: 1px solid var(--border);
  }
  .series .target {
    max-width: 40mm;
    justify-self: end;
    width: 100%;
  }
  .series dl {
    margin: 0.3rem 0 0;
    font-size: 0.9em;
  }
  .intermediate {
    padding-top: 0.3rem;
    border-top: 1px dashed var(--border);
  }
  /* The shots of a series in one row of 10, or two rows of 5 on narrow paper. */
  .shots-box {
    container-type: inline-size;
  }
  .shots {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(10, minmax(0, 1fr));
    gap: 0.4rem 0.2rem;
    font-variant-numeric: tabular-nums;
  }
  @container (max-width: 26em) {
    .shots {
      grid-template-columns: repeat(5, minmax(0, 1fr));
    }
  }
  .shots li {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.05rem;
    line-height: 1.15;
  }
  .shots small {
    font-size: 0.7em;
  }
  .distribution {
    padding-top: 1rem;
  }
  @media print {
    .paper {
      padding: 0;
      box-shadow: none;
      font-size: 10pt;
    }
  }
</style>
