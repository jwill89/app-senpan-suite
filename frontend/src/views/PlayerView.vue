<script setup lang="ts">
/**
 * Player view — the migration proof-of-concept vertical slice.
 *
 * Mirrors the legacy app.js player experience: fetch board + game state by
 * card ID, render the board, persist manual stamps in localStorage (keyed by
 * card + game), and apply live draws / game changes pushed over the WebSocket.
 * Everything here was previously interleaved into the single 2,100-line
 * component; as an isolated SFC it is independently readable and testable.
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import BingoBoard from '@/components/BingoBoard.vue'
import { apiGet } from '@/api/client'
import { useGameSocket } from '@/composables/useGameSocket'
import type { BingoGameState, BoardResponse, Card, WSMessage } from '@/types/models'

const props = defineProps<{ cardId: string }>()
const router = useRouter()

function leave() {
  void router.push('/')
}

const card = ref<Card | null>(null)
const game = ref<BingoGameState | null>(null)
const gameDetails = ref('')
const stamps = ref<Record<string, boolean>>({})
const error = ref('')
const loading = ref(true)

const cardIdRef = computed(() => props.cardId)

const calledNumbers = computed(() => game.value?.called_numbers ?? [])
const lastCalled = computed(() =>
  calledNumbers.value.length ? calledNumbers.value[calledNumbers.value.length - 1] : null,
)

function stampsKey(): string | null {
  if (!card.value || !game.value) return null
  return `stamps_${card.value.id}_${game.value.id}`
}

function loadStamps() {
  const k = stampsKey()
  if (!k) {
    stamps.value = {}
    return
  }
  const raw = localStorage.getItem(k)
  stamps.value = raw ? (JSON.parse(raw) as Record<string, boolean>) : {}
}

function saveStamps() {
  const k = stampsKey()
  if (k) localStorage.setItem(k, JSON.stringify(stamps.value))
}

function toggleStamp(r: number, c: number) {
  const key = `${r}-${c}`
  const next = { ...stamps.value }
  if (next[key]) delete next[key]
  else next[key] = true
  stamps.value = next
  saveStamps()
}

function clearStamps() {
  stamps.value = {}
  saveStamps()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<BoardResponse>(`board?id=${encodeURIComponent(props.cardId)}`)
    card.value = data.card
    game.value = data.game
    gameDetails.value = data.game_details ?? ''
    loadStamps()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load board'
  } finally {
    loading.value = false
  }
}

function onMessage(msg: WSMessage) {
  switch (msg.type) {
    case 'game_update': {
      const prevId = game.value?.id
      game.value = msg.game
      if (msg.game_details !== undefined) gameDetails.value = msg.game_details
      if (msg.game && msg.game.id !== prevId) loadStamps()
      if (!msg.game) gameDetails.value = ''
      break
    }
    case 'game_draw': {
      if (!game.value) break
      const nums = game.value.called_numbers ?? []
      if (!nums.includes(msg.drawn.number)) {
        game.value = {
          ...game.value,
          called_numbers: [...nums, msg.drawn.number],
          total_called: nums.length + 1,
        }
      }
      break
    }
    case 'details_update':
      gameDetails.value = msg.game_details
      break
    case 'card_deleted':
      socket.close()
      void router.push('/')
      break
    // style_update / halftime are out of scope for this POC slice.
  }
}

const socket = useGameSocket(cardIdRef, { onMessage, onReconnect: load })

watch(
  () => props.cardId,
  () => {
    void load()
  },
  { immediate: true },
)

socket.connect()

onBeforeUnmount(() => socket.close())
</script>

<template>
  <section class="player">
    <header class="player-bar">
      <div>
        <strong>Card {{ cardId }}</strong>
        <span v-if="card?.player_name" class="muted"> · {{ card.player_name }}</span>
      </div>
      <div class="player-bar-right">
        <span class="conn" :class="{ on: socket.connected.value }">
          {{ socket.connected.value ? 'live' : 'offline' }}
        </span>
        <button type="button" @click="leave">Leave</button>
      </div>
    </header>

    <p v-if="loading" class="muted">Loading…</p>
    <p v-else-if="error" class="error">{{ error }}</p>

    <template v-else-if="card">
      <p v-if="!game" class="muted">Waiting for the next game to start…</p>
      <p v-else class="called-line">
        <span class="muted">Last called:</span>
        <strong>{{ lastCalled ?? '—' }}</strong>
        <span class="muted">· {{ game.total_called }} drawn</span>
      </p>

      <BingoBoard
        :board="card.board_data"
        :called-numbers="calledNumbers"
        :stamps="stamps"
        @toggle="toggleStamp"
      />

      <div class="actions">
        <button type="button" @click="clearStamps">Clear stamps</button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.player {
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: flex-start;
}
.player-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  gap: 12px;
}
.player-bar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.conn {
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #888;
}
.conn.on {
  color: #2cb67d;
}
.called-line {
  display: flex;
  gap: 8px;
  align-items: baseline;
}
.muted {
  color: #9a9ab0;
}
.error {
  color: #ff6b6b;
}
.actions {
  display: flex;
  gap: 8px;
}
</style>
