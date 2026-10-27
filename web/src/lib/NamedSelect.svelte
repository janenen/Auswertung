<script lang="ts">
  // Select for clubs or teams with inline creation of a new entry.
  import type { Named } from './api'

  let {
    items,
    value = $bindable(''),
    label,
    create,
    disabled = false,
    allowEmpty = true,
  }: {
    items: Named[]
    value?: string
    label: string
    create: (name: string) => Promise<Named>
    disabled?: boolean
    allowEmpty?: boolean
  } = $props()

  let adding = $state(false)
  let name = $state('')
  let error = $state('')

  async function add() {
    error = ''
    try {
      const item = await create(name)
      value = item.id
      adding = false
      name = ''
    } catch (e) {
      error = (e as Error).message
    }
  }
</script>

<label class="field">
  <span>{label}</span>
  {#if adding}
    <span class="row">
      <!-- svelte-ignore a11y_autofocus -->
      <input
        bind:value={name}
        placeholder="Neuer Name"
        autofocus
        onkeydown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            add()
          }
          if (e.key === 'Escape') adding = false
        }}
      />
      <button type="button" class="primary" onclick={add}>Anlegen</button>
      <button type="button" onclick={() => (adding = false)}>×</button>
    </span>
    {#if error}<small class="error">{error}</small>{/if}
  {:else}
    <span class="row">
      <select bind:value {disabled}>
        {#if allowEmpty}<option value="">–</option>{/if}
        {#each items as item (item.id)}
          <option value={item.id}>{item.name}</option>
        {/each}
      </select>
      <button type="button" {disabled} onclick={() => (adding = true)} title="{label} anlegen">+ Neu</button>
    </span>
  {/if}
</label>

<style>
  .row {
    display: flex;
    gap: 0.4rem;
  }
  .row select,
  .row input {
    flex: 1;
    min-width: 0;
  }
</style>
