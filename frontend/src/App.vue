<script setup lang="ts">
/**
 * App shell for the POC. In the full migration this is where Vue Router lives
 * and where the home / player / admin / raffles routes are mounted. For now it
 * hosts a minimal join flow into the migrated PlayerView.
 */
import { ref } from 'vue'
import PlayerView from '@/views/PlayerView.vue'

const joinId = ref('')
const activeCardId = ref<string | null>(null)

function join() {
  const id = joinId.value.trim().toUpperCase()
  if (id) activeCardId.value = id
}

function leave() {
  activeCardId.value = null
  joinId.value = ''
}
</script>

<template>
  <main class="shell">
    <h1>Bingo</h1>
    <p class="poc-note">Vite + TypeScript migration · player view proof-of-concept</p>

    <form v-if="!activeCardId" class="join" @submit.prevent="join">
      <label for="join-id">Enter your card ID</label>
      <div class="join-row">
        <input
          id="join-id"
          v-model="joinId"
          autocomplete="off"
          autocapitalize="characters"
          placeholder="e.g. A2B3C4"
          maxlength="6"
        />
        <button type="submit" :disabled="!joinId.trim()">Join</button>
      </div>
    </form>

    <PlayerView
      v-else
      :key="activeCardId"
      :card-id="activeCardId"
      @leave="leave"
      @deleted="leave"
    />
  </main>
</template>

<style scoped>
.shell {
  max-width: 720px;
  margin: 0 auto;
  padding: 32px 20px;
}
h1 {
  margin: 0;
  color: var(--accent, #e53170);
}
.poc-note {
  margin: 4px 0 28px;
  color: #9a9ab0;
  font-size: 0.9rem;
}
.join {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.join-row {
  display: flex;
  gap: 8px;
}
.join input {
  flex: 1;
  text-transform: uppercase;
}
</style>
