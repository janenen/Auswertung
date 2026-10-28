<script lang="ts">
  import { api, type CompetitionSummary } from '../lib/api'
  import { href, navigate } from '../lib/router.svelte'
  import { t, tx } from '../lib/i18n.svelte'

  let competitions = $state<CompetitionSummary[] | null>(null)
  let error = $state('')
  let search = $state('')

  api
    .competitions(false)
    .then((c) => (competitions = c))
    .catch((e) => (error = e.message))

  const filtered = $derived(
    competitions?.filter((c) => `${c.name} ${c.date}`.toLowerCase().includes(search.toLowerCase())) ?? [],
  )

  async function activate(id: string) {
    try {
      await api.setActive(id, true)
      navigate(`/competitions/${id}`)
    } catch (e) {
      error = (e as Error).message
    }
  }
</script>

<div class="actions head">
  <h1>{t('Archiv')}</h1>
  <span class="spacer"></span>
  <input type="search" placeholder={t('Suchen…')} bind:value={search} />
</div>

{#if error}<p class="error">{tx(error)}</p>{/if}
{#if competitions === null}
  <p class="muted">{t('Lade…')}</p>
{:else}
  <section class="card">
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t('Wettkampf')}</th>
            <th>{t('Datum')}</th>
            <th>{t('Scheibe')}</th>
            <th>{t('Wertung')}</th>
            <th class="num">{t('Beiträge')}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each filtered as c (c.id)}
            <tr>
              <td><a href={href(`/competitions/${c.id}/results`)}>{c.name}</a></td>
              <td>{c.date}</td>
              <td>{c.type_of_target}</td>
              <td class="muted">{tx(c.modus)}</td>
              <td class="num">{c.entryCount}</td>
              <td class="num"><button onclick={() => activate(c.id)}>{t('Laden')}</button></td>
            </tr>
          {:else}
            <tr><td colspan="6" class="muted">{t('Keine beendeten Wettkämpfe.')}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  </section>
{/if}

<style>
  .head {
    margin-bottom: 1rem;
  }
  .head h1 {
    margin: 0;
  }
  td button {
    padding: 0.2rem 0.7rem;
  }
</style>
