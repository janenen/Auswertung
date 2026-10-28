<script lang="ts">
  import { api, type Entry, type Results } from '../lib/api'
  import { href, navigate } from '../lib/router.svelte'
  import Confirm from '../lib/Confirm.svelte'
  import { t, tx } from '../lib/i18n.svelte'

  let { id }: { id: string } = $props()

  let res = $state<Results | null>(null)
  let error = $state('')
  let confirmFinish = $state(false)
  $effect(() => {
    api
      .results(id)
      .then((r) => (res = r))
      .catch((e) => (error = e.message))
  })

  const c = $derived(res?.competition)

  async function finish() {
    try {
      // A competition without entries is discarded instead of archived.
      if (res!.entries.length === 0) await api.deleteCompetition(id)
      else await api.setActive(id, false)
      navigate('/')
    } catch (e) {
      error = (e as Error).message
    }
  }

  // Fetched instead of linked so a missing template shows a message.
  async function downloadLeague() {
    error = ''
    try {
      const res = await fetch(`/api/competitions/${id}/league.xlsx`)
      if (!res.ok) throw new Error((await res.json()).error ?? res.statusText)
      const name = /filename="([^"]+)"/.exec(res.headers.get('Content-Disposition') ?? '')?.[1] ?? 'Liga.xlsx'
      const url = URL.createObjectURL(await res.blob())
      const a = document.createElement('a')
      a.href = url
      a.download = name
      a.click()
      URL.revokeObjectURL(url)
    } catch (e) {
      error = (e as Error).message
    }
  }

  async function reactivate() {
    try {
      await api.setActive(id, true)
      navigate(`/competitions/${id}`)
    } catch (e) {
      error = (e as Error).message
    }
  }
</script>

{#snippet table(entries: Entry[], ranked: boolean)}
  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th class="num">#</th>
          <th>{t('Schütze')}</th>
          {#if ranked}<th>{t('Verein')}</th>{/if}
          <th>{t('Serien')}</th>
          <th class="num">{t(ranked ? 'Wertung' : 'Ergebnis')}</th>
        </tr>
      </thead>
      <tbody>
        {#each entries as e, i (e.matchId)}
          <tr class="clickable" onclick={() => navigate(`/matches/${e.matchId}`, { competition: id })}>
            <td class="num">{i + 1}.</td>
            <td>{e.shooter} {#if e.active}<span class="badge active">{e.shots}/{c?.count}</span>{/if}</td>
            {#if ranked}<td>{e.club}</td>{/if}
            <td class="muted">{e.series.join(' · ')}</td>
            <td class="num"><strong>{ranked ? e.value : e.summeGanz}</strong></td>
          </tr>
        {:else}
          <tr><td colspan="5" class="muted">{t('Keine Ergebnisse.')}</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/snippet}

{#if error}
  <p class="error">{tx(error)}</p>
{/if}
<!-- The browser suggests the title as file name when saving as PDF. -->
<svelte:head><title>{c ? `Ergebnisliste_${c.name}_${c.date}`.replaceAll(' ', '_') : 'Auswertung'}</title></svelte:head>

{#if res && c}
  <div class="actions head">
    <div>
      <h1>{t('Ergebnisliste {name}', { name: c.name })}</h1>
      <p class="muted sub">{c.date} · {tx(c.modus)} {#if !c.active}<span class="badge">{t('beendet')}</span>{/if}</p>
    </div>
    <span class="spacer"></span>
    <a class="button" href={href(c.active ? `/competitions/${id}` : '/archive')}>{t('Zurück')}</a>
    {#if res.entries.length}
      <button onclick={() => window.print()} title={t('Im Druckdialog Papierformat wählen oder „Als PDF speichern“.')}
        >{t('Drucken / PDF')}</button
      >
      {#if res.league}
        <button onclick={downloadLeague} title={t('Füllt das RSB-Formular für Liga-Ergebnisse aus')}>{t('Liga-Formular (Excel)')}</button>
      {/if}
    {/if}
    {#if c.active}
      <button class="primary" onclick={() => (confirmFinish = true)}>{t('Wettkampf beenden')}</button>
    {:else}
      <button class="primary" onclick={reactivate}>{t('Wieder aktivieren')}</button>
    {/if}
  </div>

  {#if res.league && res.score}
    <!-- Laid out like the RSB league result form. -->
    {@const l = res.league}
    {@const s = res.score}
    {@const open = (e: Entry) => navigate(`/matches/${e.matchId}`, { competition: id })}
    <section class="card sheet">
      <div class="sheet-head">
        <span><span class="label">{t('Disziplin')}:</span> {l.discipline}</span>
        <span><span class="label">{t('Datum')}:</span> {c.date}</span>
        <span><span class="label">{tx(l.league)}</span> {t('Gruppe')} {l.group}</span>
      </div>

      <div class="teams">
        {#each [{ role: t('Gastgeber / Ausrichter'), club: l.homeClubName, team: l.homeTeamName, entries: res.home, places: s.homePlaces }, { role: t('Gast'), club: l.guestClubName, team: l.guestTeamName, entries: res.guest, places: s.guestPlaces }] as side}
          <div class="team">
            <div class="team-name">{side.club} – {side.team}</div>
            <div class="team-role">{side.role}</div>
            <table class="grid">
              <thead>
                <tr>
                  <th class="num nr">{t('Lfd. Nr.')}</th>
                  <th>{t('Name')}</th>
                  <th class="num">{t('Ergebnis')}</th>
                  <th class="num">{t('Platz')}</th>
                </tr>
              </thead>
              <tbody>
                {#each Array.from({ length: Math.max(5, side.entries.length) }, (_, i) => side.entries[i]) as e, i}
                  {#if e}
                    <tr class="clickable" onclick={() => open(e)}>
                      <td class="num nr">{i + 1}</td>
                      <td>{e.shooter} {#if e.active}<span class="badge active">{e.shots}/{c.count}</span>{/if}</td>
                      <td class="num"><strong>{e.summeGanz}</strong></td>
                      <td class="num">{side.places[i]}</td>
                    </tr>
                  {:else}
                    <tr class="empty"><td class="num nr">{i + 1}</td><td></td><td></td><td></td></tr>
                  {/if}
                {/each}
              </tbody>
            </table>
          </div>
        {/each}
      </div>

      <table class="grid pairings">
        <thead>
          <tr>
            <th class="num">{t('Platz')}</th>
            <th>{t('Name')}</th>
            <th class="num">{t('Ergebnis')}</th>
            <th class="num">{t('Punkte')}</th>
            <th class="colon"></th>
            <th class="num">{t('Punkte')}</th>
            <th class="num">{t('Ergebnis')}</th>
            <th>{t('Name')}</th>
            <th class="num">{t('Platz')}</th>
          </tr>
        </thead>
        <tbody>
          {#each s.pairings as p}
            {@const h = res.home[p.home]}
            {@const g = res.guest[p.guest]}
            {@const scored = !!h && !!g}
            <tr>
              <td class="num place">{p.place}</td>
              <td class:clickable={h} onclick={() => h && open(h)}>{h?.shooter ?? ''}</td>
              <td class="num">{h?.summeGanz ?? ''}</td>
              <td class="num points" class:win={scored && p.homePoints === 2}>{scored ? p.homePoints : ''}</td>
              <td class="colon">:</td>
              <td class="num points" class:win={scored && p.guestPoints === 2}>{scored ? p.guestPoints : ''}</td>
              <td class="num">{g?.summeGanz ?? ''}</td>
              <td class:clickable={g} onclick={() => g && open(g)}>{g?.shooter ?? ''}</td>
              <td class="num place">{p.place}</td>
            </tr>
          {/each}
        </tbody>
        <tfoot>
          <tr>
            <th colspan="3" class="num">{t('Einzelpunkte')}</th>
            <td class="num points total">{s.homePoints}</td>
            <td class="colon">:</td>
            <td class="num points total">{s.guestPoints}</td>
            <td colspan="3"></td>
          </tr>
          <tr>
            <th colspan="3" class="num">{t('Mannschaftspunkte')}</th>
            <td class="num points total" class:win={s.complete && s.homeTeamPoints === 2}>{s.complete ? s.homeTeamPoints : '–'}</td>
            <td class="colon">:</td>
            <td class="num points total" class:win={s.complete && s.guestTeamPoints === 2}>{s.complete ? s.guestTeamPoints : '–'}</td>
            <td colspan="3"></td>
          </tr>
        </tfoot>
      </table>
      {#if !s.complete}
        <p class="muted small">
          {t('Vorläufig: Mannschaftspunkte werden vergeben, sobald beide Mannschaften mindestens 3 vollständige Ergebnisse haben.')}
        </p>
      {/if}
    </section>
  {:else}
    <section class="card">{@render table(res.ranked, true)}</section>
  {/if}

  <Confirm
    bind:open={confirmFinish}
    title={t('Wettkampf beenden?')}
    message={t(
      res.entries.length
        ? 'Der Wettkampf wird ins Archiv verschoben und kann dort wieder aktiviert werden.'
        : 'Der Wettkampf hat keine Beiträge und wird gelöscht.',
    )}
    confirmLabel={t('Beenden')}
    onconfirm={finish}
  />
{:else if !error}
  <p class="muted">{t('Lade…')}</p>
{/if}

<style>
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
  .sheet-head {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 2.5rem;
    margin-bottom: 1.25rem;
  }
  .label {
    font-weight: 600;
  }
  .teams {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
    gap: 1rem 1.5rem;
    margin-bottom: 1.25rem;
  }
  .team-name {
    font-weight: 600;
    text-align: center;
    padding-bottom: 0.2rem;
    border-bottom: 1px solid var(--muted);
  }
  .team-role {
    text-align: center;
    font-size: 0.85rem;
    color: var(--muted);
    margin-bottom: 0.4rem;
  }
  /* Ruled like the paper form. */
  .grid {
    width: 100%;
    border-collapse: collapse;
    font-variant-numeric: tabular-nums;
  }
  .grid th,
  .grid td {
    border: 1px solid var(--border);
    padding: 0.3rem 0.5rem;
    height: 2.1rem;
  }
  .grid thead th {
    font-size: 0.8rem;
    font-weight: 500;
    color: var(--muted);
    text-transform: none;
    letter-spacing: 0;
    background: var(--surface-2);
  }
  .grid .nr {
    width: 3.2rem;
    color: var(--muted);
  }
  .grid tr.empty td {
    background: transparent;
  }
  .pairings {
    max-width: 60rem;
  }
  .pairings .place {
    width: 3.5rem;
    font-size: 1.1rem;
  }
  .pairings .points {
    width: 4rem;
  }
  .pairings .colon {
    width: 1rem;
    padding: 0;
    text-align: center;
    font-weight: 700;
    border-left: none;
    border-right: none;
  }
  .pairings tfoot th {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text);
    text-transform: none;
    letter-spacing: 0;
    border: none;
  }
  .pairings tfoot td[colspan] {
    border: none;
  }
  .pairings .total {
    font-weight: 700;
    font-size: 1.1rem;
  }
  .win {
    color: var(--ok);
    font-weight: 700;
  }
  .small {
    font-size: 0.85rem;
    margin: 0.75rem 0 0;
  }
</style>
