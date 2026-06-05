<script setup lang="ts">
/**
 * Public raffle listing. The API already filters by role: anonymous callers get
 * currently-open raffles (within their availability window) plus closed ones.
 */
import { computed, onMounted, ref } from 'vue'
import { apiGet } from '@/api/client'
import { raffleImageSrc } from '@/lib/assets'
import type { Raffle, RafflesListResponse } from '@/types/models'

const raffles = ref<Raffle[]>([])
const loading = ref(true)
const error = ref('')

const openRaffles = computed(() => raffles.value.filter((r) => r.status === 'open'))
const closedRaffles = computed(() => raffles.value.filter((r) => r.status === 'closed'))

onMounted(async () => {
  try {
    const data = await apiGet<RafflesListResponse>('raffles')
    raffles.value = data.raffles ?? []
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load raffles'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="raffles">
    <h1>Raffles</h1>

    <p v-if="loading" class="muted">Loading…</p>
    <p v-else-if="error" class="error">{{ error }}</p>

    <template v-else>
      <p v-if="raffles.length === 0" class="muted">No raffles right now. Check back later!</p>

      <section v-if="openRaffles.length" class="group">
        <h2>Open</h2>
        <ul class="grid">
          <li v-for="r in openRaffles" :key="r.id">
            <RouterLink class="tile" :to="{ name: 'raffle', params: { id: String(r.id) } }">
              <img
                v-if="r.prize_image"
                :src="raffleImageSrc(r.prize_image)"
                alt=""
                class="thumb"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
              <div class="tile-body">
                <strong>{{ r.title }}</strong>
                <span class="muted small">
                  {{ r.cost_per_entry > 0 ? `${r.cost_per_entry} per entry` : 'Free entry' }}
                </span>
              </div>
            </RouterLink>
          </li>
        </ul>
      </section>

      <section v-if="closedRaffles.length" class="group">
        <h2>Past raffles</h2>
        <ul class="grid">
          <li v-for="r in closedRaffles" :key="r.id">
            <RouterLink class="tile closed" :to="{ name: 'raffle', params: { id: String(r.id) } }">
              <img
                v-if="r.prize_image"
                :src="raffleImageSrc(r.prize_image)"
                alt=""
                class="thumb"
                @error="($event.target as HTMLImageElement).style.display = 'none'"
              />
              <div class="tile-body">
                <strong>{{ r.title }}</strong>
                <span class="muted small">Closed</span>
              </div>
            </RouterLink>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>

<style scoped>
h1 {
  margin-top: 0;
}
.group {
  margin-top: 24px;
}
.group h2 {
  font-size: 1.05rem;
  color: #c4c4d4;
}
.grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}
.tile {
  display: flex;
  flex-direction: column;
  border: 1px solid #26263a;
  border-radius: 12px;
  overflow: hidden;
  background: #1c1c28;
  text-decoration: none;
  color: inherit;
  transition: border-color 0.12s;
}
.tile:hover {
  border-color: var(--accent, #e53170);
}
.tile.closed {
  opacity: 0.8;
}
.thumb {
  width: 100%;
  height: 130px;
  object-fit: cover;
  background: #14141d;
}
.tile-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
}
.muted {
  color: #9a9ab0;
}
.small {
  font-size: 0.85rem;
}
.error {
  color: #ff6b6b;
}
</style>
