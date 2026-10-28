<script lang="ts">
  // Creates a competition, or collects the settings of a quick analysis
  // (which is not stored as a competition).
  import { api, type Named } from '../lib/api'
  import { meta, today } from '../lib/meta.svelte'
  import { navigate } from '../lib/router.svelte'
  import NamedSelect from '../lib/NamedSelect.svelte'
  import { t, tx } from '../lib/i18n.svelte'

  let { quick = false }: { quick?: boolean } = $props()

  // Default names stay German: they are saved and printed in the (German) PDFs.
  // svelte-ignore state_referenced_locally
  let name = $state(quick ? 'Schnellauswertung' : 'Training')
  let date = $state(today())
  let count = $state(40)
  let shotsPerTarget = $state(1)
  let typeOfTarget = $state('LG')
  let modus = $state(meta.modi[0])

  let discipline = $state('')
  let league = $state(meta.leagues[0])
  let group = $state('')
  let homeClub = $state('')
  let homeTeam = $state('')
  let guestClub = $state('')
  let guestTeam = $state('')

  let clubs = $state<Named[]>([])
  let teams = $state<Named[]>([])
  let error = $state('')
  let saving = $state(false)

  const isLeague = $derived(modus === meta.leagueModus)

  $effect(() => {
    if (isLeague && clubs.length === 0) {
      api.clubs().then((c) => (clubs = c))
      api.teams().then((t) => (teams = t))
    }
  })

  async function createClub(n: string) {
    const c = await api.saveClub({ name: n })
    clubs = await api.clubs()
    return c
  }
  async function createTeam(n: string) {
    const t = await api.saveTeam({ name: n })
    teams = await api.teams()
    return t
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    if (quick) {
      navigate('/entry', { name, count, spt: shotsPerTarget, target: typeOfTarget, decimal: modus === 'Bestes Ergebnis Zehntel' })
      return
    }
    saving = true
    try {
      const c = await api.createCompetition({
        name,
        date,
        count,
        shots_per_target: shotsPerTarget,
        type_of_target: typeOfTarget,
        modus,
        leagueData: isLeague
          ? {
              discipline,
              league,
              group,
              home_club: homeClub,
              home_team: homeTeam,
              guest_club: guestClub,
              guest_team: guestTeam,
            }
          : undefined,
      })
      navigate(`/competitions/${c.id}`)
    } catch (err) {
      error = (err as Error).message
    } finally {
      saving = false
    }
  }
</script>

<h1>{t(quick ? 'Schnellauswertung' : 'Neuer Wettkampf')}</h1>
{#if quick}
  <p class="muted">{t('Das Ergebnis wird gespeichert, aber keinem Wettkampf hinzugefügt.')}</p>
{/if}

<form class="card" onsubmit={submit}>
  <div class="form-grid">
    <label class="field">
      <span>{t(quick ? 'Titel' : 'Name')}</span>
      <input bind:value={name} required title={t('z.B. Training, Ligawettkampf, Preisschießen')} />
    </label>
    {#if !quick}
      <label class="field">
        <span>{t('Datum')}</span>
        <input bind:value={date} required pattern={'\\d{2}\\.\\d{2}\\.\\d{4}'} placeholder={t('TT.MM.JJJJ')} />
      </label>
    {/if}
    <label class="field">
      <span>{t('Schusszahl')}</span>
      <input type="number" bind:value={count} min="1" max="999" required />
    </label>
    <label class="field">
      <span>{t('Schuss pro Scheibe')}</span>
      <input type="number" bind:value={shotsPerTarget} min="1" max="5" required title={t('Nur bei Maschinenauswertung')} />
    </label>
    <label class="field">
      <span>{t('Scheibentyp')}</span>
      <select bind:value={typeOfTarget}>
        {#each meta.targetTypes as type}<option value={type}>{type}</option>{/each}
      </select>
    </label>
    <label class="field">
      <span>{t('Wertung')}</span>
      <select bind:value={modus}>
        {#each quick ? meta.modi.slice(0, 2) : meta.modi as m}<option value={m}>{tx(m)}</option>{/each}
      </select>
    </label>
  </div>

  {#if isLeague}
    <h2 class="league-title">{t('Liga')}</h2>
    <div class="form-grid">
      <label class="field">
        <span>{t('Disziplin')}</span>
        <input bind:value={discipline} />
      </label>
      <label class="field">
        <span>{t('Liga')}</span>
        <select bind:value={league}>
          {#each meta.leagues as l}<option value={l}>{tx(l)}</option>{/each}
        </select>
      </label>
      <label class="field">
        <span>{t('Gruppe')}</span>
        <input bind:value={group} />
      </label>
    </div>
    <div class="form-grid sides">
      <NamedSelect label={t('Heimverein')} items={clubs} bind:value={homeClub} create={createClub} />
      <NamedSelect label={t('Heimmannschaft')} items={teams} bind:value={homeTeam} create={createTeam} />
      <NamedSelect label={t('Gastverein')} items={clubs} bind:value={guestClub} create={createClub} />
      <NamedSelect label={t('Gastmannschaft')} items={teams} bind:value={guestTeam} create={createTeam} />
    </div>
  {/if}

  {#if error}<p class="error">{tx(error)}</p>{/if}

  <div class="actions submit">
    <a class="button" href="#/">{t('Abbrechen')}</a>
    <button class="primary" type="submit" disabled={saving}>{t(quick ? 'Weiter' : 'Wettkampf anlegen')}</button>
  </div>
</form>

<style>
  .league-title {
    margin-top: 1.5rem;
  }
  .sides {
    margin-top: 0.9rem;
  }
  .submit {
    justify-content: flex-end;
    margin-top: 1.25rem;
  }
</style>
