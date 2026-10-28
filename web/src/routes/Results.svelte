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
  const teamTotal = (entries: Entry[]) => entries.reduce((sum, e) => sum + e.summeGanz, 0)

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
{#if res && c}
  <div class="actions head">
    <div>
      <h1>{t('Ergebnisliste {name}', { name: c.name })}</h1>
      <p class="muted sub">{c.date} · {tx(c.modus)} {#if !c.active}<span class="badge">{t('beendet')}</span>{/if}</p>
    </div>
    <span class="spacer"></span>
    <a class="button" href={href(c.active ? `/competitions/${id}` : '/archive')}>{t('Zurück')}</a>
    {#if res.entries.length}
      <a class="button" href="/api/competitions/{id}/report.pdf" target="_blank">{t('PDF erzeugen')}</a>
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

  {#if res.league}
    <div class="league">
      <section class="card">
        <div class="actions">
          <h2>{t('Heim: {club} – {team}', { club: res.league.homeClubName, team: res.league.homeTeamName })}</h2>
          <span class="spacer"></span><strong class="total">{teamTotal(res.home)}</strong>
        </div>
        {@render table(res.home, false)}
      </section>
      <section class="card">
        <div class="actions">
          <h2>{t('Gast: {club} – {team}', { club: res.league.guestClubName, team: res.league.guestTeamName })}</h2>
          <span class="spacer"></span><strong class="total">{teamTotal(res.guest)}</strong>
        </div>
        {@render table(res.guest, false)}
      </section>
    </div>
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
  .league {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(22rem, 1fr));
    gap: 1rem;
  }
  .league h2 {
    margin: 0;
  }
  .total {
    font-size: 1.4rem;
  }
</style>
