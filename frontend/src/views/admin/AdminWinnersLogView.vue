<script setup lang="ts">
/** Paginated, sortable winners log (bingo tab). GET /api/winners-log. */
import { computed, onMounted, ref } from 'vue'
import { apiGet } from '@/api/client'
import { useToast } from '@/composables/useToast'
import type { WinnersLogEntry, WinnersLogResponse } from '@/types/models'

const { show } = useToast()

type SortField = 'logged_at' | 'card_id' | 'player_name' | 'game_details'

const entries = ref<WinnersLogEntry[]>([])
const total = ref(0)
const page = ref(1)
const perPage = ref(25)
const sort = ref<SortField>('logged_at')
const dir = ref<'asc' | 'desc'>('desc')
const loading = ref(true)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / perPage.value)))

const columns: { field: SortField; label: string }[] = [
  { field: 'logged_at', label: 'When' },
  { field: 'card_id', label: 'Card' },
  { field: 'player_name', label: 'Player' },
  { field: 'game_details', label: 'Game' },
]

async function load() {
  loading.value = true
  try {
    const q = `winners-log?page=${page.value}&per_page=${perPage.value}&sort=${sort.value}&dir=${dir.value}`
    const data = await apiGet<WinnersLogResponse>(q)
    entries.value = data.entries ?? []
    total.value = data.total ?? 0
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to load winners log', 'error')
  } finally {
    loading.value = false
  }
}

function setSort(field: SortField) {
  if (sort.value === field) {
    dir.value = dir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sort.value = field
    dir.value = 'desc'
  }
  page.value = 1
  void load()
}

function goPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  page.value = p
  void load()
}

function patternsOf(entry: WinnersLogEntry): string {
  try {
    const arr = JSON.parse(entry.winning_patterns) as string[]
    return Array.isArray(arr) ? arr.join(', ') : ''
  } catch {
    return ''
  }
}

onMounted(load)
</script>

<template>
  <section>
    <div class="head">
      <h2>Winners Log</h2>
      <label class="per-page">
        Per page
        <select v-model.number="perPage" @change="goPage(1)">
          <option :value="10">10</option>
          <option :value="25">25</option>
          <option :value="50">50</option>
          <option :value="100">100</option>
        </select>
      </label>
    </div>

    <p v-if="loading" class="muted">Loading…</p>
    <p v-else-if="entries.length === 0" class="muted">No winners logged yet.</p>

    <template v-else>
      <table>
        <thead>
          <tr>
            <th v-for="c in columns" :key="c.field" class="sortable" @click="setSort(c.field)">
              {{ c.label }}
              <span v-if="sort === c.field">{{ dir === 'asc' ? '▲' : '▼' }}</span>
            </th>
            <th>Patterns</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in entries" :key="e.id">
            <td>{{ e.logged_at }}</td>
            <td class="mono">{{ e.card_id }}</td>
            <td>{{ e.player_name || '—' }}</td>
            <td>{{ e.game_details || '—' }}</td>
            <td>{{ patternsOf(e) || '—' }}</td>
          </tr>
        </tbody>
      </table>

      <div class="pager">
        <button type="button" :disabled="page <= 1" @click="goPage(page - 1)">Prev</button>
        <span class="muted">Page {{ page }} of {{ totalPages }} · {{ total }} total</span>
        <button type="button" :disabled="page >= totalPages" @click="goPage(page + 1)">Next</button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.per-page {
  display: flex;
  gap: 6px;
  align-items: center;
  font-size: 0.85rem;
  color: #9a9ab0;
}
table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 8px;
}
th,
td {
  text-align: left;
  padding: 8px 10px;
  border-bottom: 1px solid #26263a;
}
th.sortable {
  cursor: pointer;
  user-select: none;
  color: #c4c4d4;
}
th.sortable:hover {
  color: #fff;
}
.mono {
  font-family: ui-monospace, monospace;
}
.pager {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 16px;
}
.muted {
  color: #9a9ab0;
}
</style>
