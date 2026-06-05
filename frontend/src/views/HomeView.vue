<script setup lang="ts">
/**
 * Landing page: join a bingo game by card ID, and (if any raffles exist) a
 * card linking to the public raffles list.
 */
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiGet } from '@/api/client'
import type { RafflesListResponse } from '@/types/models'

const router = useRouter()

const joinId = ref('')
const raffleCount = ref(0)
const openRaffleCount = ref(0)

const canJoin = computed(() => joinId.value.trim().length > 0)

function join() {
  const id = joinId.value.trim().toUpperCase()
  if (id) void router.push({ name: 'play', params: { cardId: id } })
}

onMounted(async () => {
  // Public list returns currently-open raffles plus closed (past) ones.
  try {
    const data = await apiGet<RafflesListResponse>('raffles')
    const raffles = data.raffles ?? []
    raffleCount.value = raffles.length
    openRaffleCount.value = raffles.filter((r) => r.status === 'open').length
  } catch {
    // Raffles are optional on the home screen — ignore load failures.
  }
})
</script>

<template>
  <div class="home">
    <section class="card">
      <h2>Play Bingo</h2>
      <p class="muted">Enter the card ID you were given.</p>
      <form class="join" @submit.prevent="join">
        <input
          v-model="joinId"
          autocomplete="off"
          autocapitalize="characters"
          placeholder="e.g. A2B3C4"
          maxlength="6"
          aria-label="Card ID"
        />
        <button type="submit" :disabled="!canJoin">Join</button>
      </form>
    </section>

    <section v-if="raffleCount > 0" class="card">
      <h2>Raffles</h2>
      <p class="muted">
        <template v-if="openRaffleCount > 0">
          {{ openRaffleCount }} raffle{{ openRaffleCount === 1 ? '' : 's' }} open for entries.
        </template>
        <template v-else>Browse past raffle results.</template>
      </p>
      <RouterLink class="btn-link" :to="{ name: 'raffles' }">View raffles</RouterLink>
    </section>
  </div>
</template>

<style scoped>
.home {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.card {
  border: 1px solid #26263a;
  border-radius: 14px;
  padding: 22px;
  background: #1c1c28;
}
.card h2 {
  margin: 0 0 6px;
}
.muted {
  color: #9a9ab0;
  margin: 0 0 14px;
}
.join {
  display: flex;
  gap: 8px;
}
.join input {
  flex: 1;
  text-transform: uppercase;
}
.btn-link {
  display: inline-block;
  padding: 8px 16px;
  border-radius: 8px;
  background: var(--accent, #e53170);
  color: #fff;
  text-decoration: none;
  font-weight: 600;
}
</style>
