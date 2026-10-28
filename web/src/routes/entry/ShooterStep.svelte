<script lang="ts">
  import { api, type League, type Named, type User } from '../../lib/api'
  import NamedSelect from '../../lib/NamedSelect.svelte'
  import { t, tx } from '../../lib/i18n.svelte'

  let {
    league,
    user = $bindable(),
    clubId = $bindable(),
    teamId = $bindable(),
    clubs = $bindable(),
    teams = $bindable(),
    onnext,
  }: {
    league: League | null
    user: User | null
    clubId: string
    teamId: string
    clubs: Named[]
    teams: Named[]
    onnext: () => void
  } = $props()

  let users = $state<User[]>([])
  let search = $state('')
  let side = $state<'' | 'home' | 'guest'>('')
  let editing = $state<{ id?: string; name: string; birthday: string } | null>(null)
  let error = $state('')

  api.users().then((u) => (users = u))
  if (clubs.length === 0) api.clubs().then((c) => (clubs = c))
  if (teams.length === 0) api.teams().then((t) => (teams = t))

  const filtered = $derived(
    search ? users.filter((u) => u.name.toLowerCase().includes(search.toLowerCase())) : users,
  )

  $effect(() => {
    if (!league) return
    if (side === 'home') {
      clubId = league.home_club
      teamId = league.home_team
    } else if (side === 'guest') {
      clubId = league.guest_club
      teamId = league.guest_team
    }
  })

  async function saveUser(e: SubmitEvent) {
    e.preventDefault()
    if (!editing) return
    error = ''
    try {
      const saved = await api.saveUser(editing)
      users = await api.users()
      user = users.find((u) => u.id === saved.id) ?? saved
      editing = null
    } catch (err) {
      error = (err as Error).message
    }
  }

  const canContinue = $derived(!!user && (!league || side !== ''))
</script>

<div class="layout">
  <section class="card users">
    <div class="actions">
      <h2>{t('Schütze')}</h2>
      <span class="spacer"></span>
      <button onclick={() => (editing = { name: search, birthday: '' })}>{t('+ Neuer Schütze')}</button>
    </div>
    <input class="search" type="search" placeholder={t('Suchen…')} bind:value={search} />
    <ul class="list" role="listbox" aria-label={t('Schützen')}>
      {#each filtered as u (u.id)}
        <li>
          <button
            role="option"
            aria-selected={user?.id === u.id}
            class:selected={user?.id === u.id}
            onclick={() => (user = u)}
            ondblclick={() => {
              user = u
              if (canContinue) onnext()
            }}>{u.name}</button
          >
        </li>
      {:else}
        <li class="muted empty">{t('Kein Schütze gefunden.')}</li>
      {/each}
    </ul>
  </section>

  <section class="card">
    {#if editing}
      <form onsubmit={saveUser}>
        <h2>{t(editing.id ? 'Schütze bearbeiten' : 'Neuer Schütze')}</h2>
        <div class="fields">
          <label class="field">
            <span>{t('Name')}</span>
            <!-- svelte-ignore a11y_autofocus -->
            <input bind:value={editing.name} required autofocus />
          </label>
          <label class="field">
            <span>{t('Geburtsdatum')}</span>
            <input bind:value={editing.birthday} placeholder={t('TT.MM.JJJJ')} />
          </label>
        </div>
        {#if error}<p class="error">{tx(error)}</p>{/if}
        <div class="actions end">
          <button type="button" onclick={() => (editing = null)}>{t('Abbrechen')}</button>
          <button type="submit" class="primary">{t('Speichern')}</button>
        </div>
      </form>
    {:else}
      <div class="actions">
        <h2>{user?.name ?? t('Schütze auswählen')}</h2>
        <span class="spacer"></span>
        {#if user}
          <button onclick={() => (editing = { id: user!.id, name: user!.name, birthday: user!.birthday })}
            >{t('Bearbeiten')}</button
          >
        {/if}
      </div>
      <div class="fields">
        {#if league}
          <fieldset>
            <legend>{t('Mannschaft')}</legend>
            <label
              ><input type="radio" bind:group={side} value="home" />
              {t('Heim: {club} – {team}', { club: league.homeClubName, team: league.homeTeamName })}</label
            >
            <label
              ><input type="radio" bind:group={side} value="guest" />
              {t('Gast: {club} – {team}', { club: league.guestClubName, team: league.guestTeamName })}</label
            >
          </fieldset>
        {:else}
          <NamedSelect
            label={t('Verein')}
            items={clubs}
            bind:value={clubId}
            create={async (name) => {
              const c = await api.saveClub({ name })
              clubs = await api.clubs()
              return c
            }}
          />
          <NamedSelect
            label={t('Mannschaft')}
            items={teams}
            bind:value={teamId}
            create={async (name) => {
              const t = await api.saveTeam({ name })
              teams = await api.teams()
              return t
            }}
          />
        {/if}
      </div>
      <div class="actions end">
        <button class="primary" disabled={!canContinue} onclick={onnext}>{t('Weiter')}</button>
      </div>
    {/if}
  </section>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: minmax(16rem, 1fr) minmax(18rem, 1.3fr);
    gap: 1rem;
    align-items: start;
  }
  @media (max-width: 720px) {
    .layout {
      grid-template-columns: 1fr;
    }
  }
  h2 {
    margin: 0;
  }
  .search {
    width: 100%;
    margin: 0.75rem 0 0.5rem;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 26rem;
    overflow-y: auto;
  }
  .list button {
    width: 100%;
    text-align: left;
    border: none;
    border-radius: 6px;
    background: none;
    padding: 0.4rem 0.6rem;
  }
  .list button.selected {
    background: var(--accent);
    color: var(--accent-text);
  }
  .empty {
    padding: 0.4rem 0.6rem;
  }
  .fields {
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    margin: 1rem 0;
  }
  fieldset {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .end {
    justify-content: flex-end;
  }
</style>
