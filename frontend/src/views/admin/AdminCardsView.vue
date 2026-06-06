<script setup lang="ts">
/**
 * Card management (bingo tab): generate, search, preview (reusing BingoBoard),
 * edit player name/details, and delete. POST /api/cards actions.
 */
import { computed, onMounted, ref } from 'vue'
import BingoBoard from '@/components/BingoBoard.vue'
import { apiGet, apiPost } from '@/api/client'
import { useToast } from '@/composables/useToast'
import type {
  AdminCardSummary,
  AdminCardsResponse,
  BoardResponse,
  Card,
  GenerateCardsResponse,
} from '@/types/models'

const { show } = useToast()

const cards = ref<AdminCardSummary[]>([])
const loading = ref(true)
const search = ref('')
const generateCount = ref(10)
const generating = ref(false)

const preview = ref<Card | null>(null)
const editName = ref('')
const editDetails = ref('')
const previewLoading = ref(false)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return cards.value
  return cards.value.filter(
    (c) => c.id.toLowerCase().includes(q) || (c.player_name ?? '').toLowerCase().includes(q),
  )
})

async function load() {
  loading.value = true
  try {
    const data = await apiGet<AdminCardsResponse>('cards')
    cards.value = data.cards ?? []
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to load cards', 'error')
  } finally {
    loading.value = false
  }
}

async function generate() {
  const count = Math.max(1, Math.min(500, Math.trunc(generateCount.value) || 1))
  generating.value = true
  try {
    const data = await apiPost<GenerateCardsResponse>('cards', { action: 'generate', count })
    show(`Generated ${data.count} card(s)`, 'success')
    await load()
  } catch (e) {
    show(e instanceof Error ? e.message : 'Generate failed', 'error')
  } finally {
    generating.value = false
  }
}

async function openPreview(id: string) {
  if (previewLoading.value) return
  previewLoading.value = true
  try {
    const data = await apiGet<BoardResponse>(`board?id=${encodeURIComponent(id)}&preview=1`)
    preview.value = data.card
    editName.value = data.card.player_name ?? ''
    editDetails.value = data.card.details ?? ''
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to load card', 'error')
  } finally {
    previewLoading.value = false
  }
}

function closePreview() {
  preview.value = null
}

async function savePreview() {
  if (!preview.value) return
  const id = preview.value.id
  try {
    await apiPost('cards', {
      action: 'update_player',
      id,
      player_name: editName.value,
      details: editDetails.value,
    })
    const c = cards.value.find((x) => x.id === id)
    if (c) {
      c.player_name = editName.value
      c.details = editDetails.value
    }
    show('Card updated', 'success')
    closePreview()
  } catch (e) {
    show(e instanceof Error ? e.message : 'Update failed', 'error')
  }
}

async function deleteCard(id: string) {
  if (!confirm(`Delete card ${id}?`)) return
  try {
    await apiPost('cards', { action: 'delete', id })
    cards.value = cards.value.filter((c) => c.id !== id)
    if (preview.value?.id === id) closePreview()
    show('Card deleted', 'info')
  } catch (e) {
    show(e instanceof Error ? e.message : 'Delete failed', 'error')
  }
}

async function deleteAll() {
  if (!confirm('Delete ALL cards? This cannot be undone.')) return
  try {
    await apiPost('cards', { action: 'delete_all' })
    cards.value = []
    show('All cards deleted', 'info')
  } catch (e) {
    show(e instanceof Error ? e.message : 'Delete failed', 'error')
  }
}

onMounted(load)
</script>

<template>
  <section>
    <h2>Cards</h2>

    <div class="toolbar">
      <form class="gen" @submit.prevent="generate">
        <input v-model.number="generateCount" type="number" min="1" max="500" aria-label="Count" />
        <button type="submit" :disabled="generating">
          {{ generating ? 'Generating…' : 'Generate' }}
        </button>
      </form>
      <input v-model="search" class="search" placeholder="Search id or name…" aria-label="Search" />
      <button type="button" class="danger" :disabled="cards.length === 0" @click="deleteAll">
        Delete all
      </button>
    </div>

    <p v-if="loading" class="muted">Loading…</p>
    <p v-else-if="cards.length === 0" class="muted">No cards yet. Generate some above.</p>
    <p v-else class="muted small">{{ filtered.length }} of {{ cards.length }} cards</p>

    <ul class="chips">
      <li
        v-for="c in filtered"
        :key="c.id"
        class="chip"
        :class="{ named: !!c.player_name }"
        :title="c.player_name || 'No player assigned'"
        @click="openPreview(c.id)"
      >
        <span class="chip-id">{{ c.id }}</span>
        <span v-if="c.player_name" class="chip-name">{{ c.player_name }}</span>
      </li>
    </ul>

    <!-- Preview / edit modal -->
    <div v-if="preview" class="modal-backdrop" @click.self="closePreview">
      <div class="modal">
        <header class="modal-head">
          <strong class="mono">{{ preview.id }}</strong>
          <button type="button" class="x" @click="closePreview">✕</button>
        </header>

        <BingoBoard
          :board="preview.board_data"
          :called-numbers="[]"
          :stamps="{}"
          :interactive="false"
        />

        <div class="fields">
          <label>
            Player name
            <input v-model="editName" />
          </label>
          <label>
            Details
            <input v-model="editDetails" />
          </label>
        </div>

        <footer class="modal-foot">
          <button type="button" class="danger" @click="deleteCard(preview.id)">Delete</button>
          <button type="button" @click="savePreview">Save</button>
        </footer>
      </div>
    </div>
  </section>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-bottom: 12px;
}
.gen {
  display: flex;
  gap: 6px;
}
.gen input {
  width: 80px;
}
.search {
  flex: 1;
  min-width: 160px;
}
.danger {
  border-color: #5a2a36;
  color: #ff9a9a;
}
.chips {
  list-style: none;
  padding: 0;
  margin: 12px 0 0;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.chip {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 12px;
  border: 1px solid #2a2a3a;
  border-radius: 10px;
  background: #1c1c28;
  cursor: pointer;
  min-width: 84px;
}
.chip:hover {
  border-color: var(--accent, #e53170);
}
.chip.named {
  border-left: 3px solid #2cb67d;
}
.chip-id {
  font-family: ui-monospace, monospace;
  font-weight: 600;
}
.chip-name {
  font-size: 0.78rem;
  color: #9a9ab0;
}
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: grid;
  place-items: center;
  z-index: 900;
  padding: 16px;
}
.modal {
  background: #16161f;
  border: 1px solid #2a2a3a;
  border-radius: 16px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: center;
  max-height: 90vh;
  overflow: auto;
}
.modal-head,
.modal-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 12px;
}
.x {
  border: none;
  background: transparent;
  color: #9a9ab0;
  font-size: 1.1rem;
}
.fields {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}
.fields label {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.mono {
  font-family: ui-monospace, monospace;
}
.muted {
  color: #9a9ab0;
}
.small {
  font-size: 0.85rem;
}
</style>
