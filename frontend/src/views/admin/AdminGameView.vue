<script setup lang="ts">
/**
 * Admin game control (bingo tab) — the core gameplay loop.
 *
 * No active game → pattern picker (category filter + search + multi-select) → start.
 * Active game → draw numbers (with optional player-reveal delay + countdown),
 * called-numbers grid, winners (click to verify against the active patterns),
 * end-game with winner selection, game-details markdown, and the halftime prompt.
 *
 * Connects the WebSocket on the admin channel (no card id) so draws/state from
 * other admin tabs stay in sync; the drawing tab updates from the HTTP response
 * and de-dupes the echoed socket message.
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import BingoBoard from '@/components/BingoBoard.vue'
import PatternPreview from '@/components/PatternPreview.vue'
import { apiGet, apiPost } from '@/api/client'
import { useGameSocket } from '@/composables/useGameSocket'
import { useToast } from '@/composables/useToast'
import { renderMarkdown } from '@/lib/markdown'
import type {
  BingoDrawnNumber,
  BingoGameState,
  BoardResponse,
  Card,
  DrawResultResponse,
  FrequentWinner,
  FrequentWinnersResponse,
  GameStateResponse,
  Pattern,
  PatternCategory,
  PatternsListResponse,
  WSMessage,
} from '@/types/models'

const { show } = useToast()

const currentGame = ref<BingoGameState | null>(null)
const winners = ref<string[]>([])
const gameDetails = ref('')
const lastDrawn = ref<BingoDrawnNumber | null>(null)
const frequentWinners = ref<FrequentWinner[]>([])

// Pattern picker (start a game)
const patterns = ref<Pattern[]>([])
const categories = ref<PatternCategory[]>([])
const selectedPatternIds = ref<number[]>([])
const categoryFilter = ref<number | null>(null)
const patternSearch = ref('')

// Draw controls
const drawDelay = ref(0)
const drawCountdown = ref<number | null>(null)
const drawSent = ref(false)
let countdownTimer: number | null = null

// Modals
const winnerPreview = ref<{ card: Card; matched: Set<string> } | null>(null)
const winnerLoading = ref(false)
const showEndGame = ref(false)
const endGameSelected = ref<string[]>([])
const showHalftime = ref(false)

// Busy flags
const starting = ref(false)
const drawing = ref(false)

const hasGame = computed(() => currentGame.value !== null)
const calledSet = computed(() => new Set(currentGame.value?.called_numbers ?? []))
const totalCalled = computed(() => currentGame.value?.called_numbers?.length ?? 0)
const renderedDetails = computed(() => renderMarkdown(gameDetails.value))

const filteredPatterns = computed(() => {
  let list = patterns.value
  if (categoryFilter.value !== null) {
    list = list.filter((p) => p.category_id === categoryFilter.value)
  }
  const q = patternSearch.value.trim().toLowerCase()
  if (q) list = list.filter((p) => p.name.toLowerCase().includes(q))
  return list
})

function colNums(col: number): number[] {
  const start = col * 15 + 1
  return Array.from({ length: 15 }, (_, i) => start + i)
}

// ── Data loading ────────────────────────────────────────────────────────────

async function loadState() {
  const data = await apiGet<GameStateResponse>('game')
  currentGame.value = data.game
  winners.value = data.winners ?? []
  gameDetails.value = data.game_details ?? ''
}

async function loadPatterns() {
  const data = await apiGet<PatternsListResponse>('patterns')
  patterns.value = data.patterns ?? []
  categories.value = data.categories ?? []
}

async function loadFrequent() {
  try {
    const data = await apiGet<FrequentWinnersResponse>('winners-log/frequent')
    frequentWinners.value = data.winners ?? []
  } catch {
    frequentWinners.value = []
  }
}

async function reloadAll() {
  try {
    await Promise.all([loadState(), loadPatterns(), loadFrequent()])
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to load game', 'error')
  }
}

// ── WebSocket ───────────────────────────────────────────────────────────────

const adminChannel = ref<string | null>(null) // no card id → admin channel
const socket = useGameSocket(adminChannel, { onMessage, onReconnect: reloadAll })

function onMessage(msg: WSMessage) {
  if (msg.type === 'game_update') {
    currentGame.value = msg.game
    winners.value = msg.winners ?? []
    if (!msg.game) {
      lastDrawn.value = null
      winnerPreview.value = null
    }
  } else if (msg.type === 'game_draw') {
    if (!currentGame.value) return
    const nums = currentGame.value.called_numbers ?? []
    if (!nums.includes(msg.drawn.number)) {
      currentGame.value = {
        ...currentGame.value,
        called_numbers: [...nums, msg.drawn.number],
        total_called: nums.length + 1,
      }
      lastDrawn.value = msg.drawn
    }
    const prev = winners.value.length
    if (msg.winners) winners.value = msg.winners
    if (winners.value.length > prev) show('We have winner(s)!', 'success')
  }
}

// ── Actions ─────────────────────────────────────────────────────────────────

function togglePattern(id: number) {
  const i = selectedPatternIds.value.indexOf(id)
  if (i >= 0) selectedPatternIds.value.splice(i, 1)
  else selectedPatternIds.value.push(id)
}

async function startGame() {
  if (selectedPatternIds.value.length === 0) {
    show('Select at least one win pattern', 'error')
    return
  }
  starting.value = true
  try {
    const data = await apiPost<GameStateResponse>('game', {
      action: 'start',
      pattern_ids: selectedPatternIds.value,
    })
    currentGame.value = data.game
    winners.value = []
    lastDrawn.value = null
    selectedPatternIds.value = []
    gameDetails.value = data.game_details ?? ''
    show('Game started!', 'success')
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to start game', 'error')
  } finally {
    starting.value = false
  }
}

function clearCountdown() {
  if (countdownTimer !== null) {
    window.clearInterval(countdownTimer)
    countdownTimer = null
  }
  drawCountdown.value = null
}

async function drawNumber() {
  if (drawing.value) return
  drawing.value = true
  try {
    const delay = Math.max(0, Math.min(60, Math.trunc(drawDelay.value) || 0))
    const prevWinners = winners.value.length
    const data = await apiPost<DrawResultResponse>('game', { action: 'draw', delay })
    lastDrawn.value = data.drawn
    currentGame.value = data.game
    winners.value = data.winners ?? []
    if (winners.value.length > prevWinners) show('We have winner(s)!', 'success')

    clearCountdown()
    if (delay > 0) {
      drawSent.value = false
      drawCountdown.value = delay
      countdownTimer = window.setInterval(() => {
        const v = drawCountdown.value
        if (v === null) return
        const next = v - 1
        drawCountdown.value = next
        if (next <= 0) {
          clearCountdown()
          drawSent.value = true
          window.setTimeout(() => (drawSent.value = false), 3000)
        }
      }, 1000)
    } else {
      drawSent.value = false
    }

    if (totalCalled.value === 35) showHalftime.value = true
  } catch (e) {
    show(e instanceof Error ? e.message : 'Draw failed', 'error')
  } finally {
    drawing.value = false
  }
}

function endGame() {
  if (winners.value.length > 0) {
    endGameSelected.value = [...winners.value]
    showEndGame.value = true
  } else {
    void confirmEndGame([])
  }
}

function toggleEndGameWinner(id: string) {
  const i = endGameSelected.value.indexOf(id)
  if (i >= 0) endGameSelected.value.splice(i, 1)
  else endGameSelected.value.push(id)
}

async function confirmEndGame(ids: string[]) {
  showEndGame.value = false
  try {
    await apiPost('game', { action: 'end', valid_winner_ids: ids })
    currentGame.value = null
    winners.value = []
    lastDrawn.value = null
    winnerPreview.value = null
    clearCountdown()
    drawSent.value = false
    show('Game ended', 'info')
    void loadFrequent()
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to end game', 'error')
  }
}

async function saveDetails() {
  try {
    await apiPost('game', { action: 'update_details', details: gameDetails.value })
    show('Game details saved', 'success')
  } catch (e) {
    show(e instanceof Error ? e.message : 'Save failed', 'error')
  }
}

async function viewWinner(cardId: string) {
  if (winnerLoading.value) return
  winnerLoading.value = true
  try {
    const data = await apiGet<BoardResponse>(`board?id=${encodeURIComponent(cardId)}&preview=1`)
    const card = data.card
    const called = calledSet.value
    const matched = new Set<string>()
    for (const pat of currentGame.value?.patterns ?? []) {
      const pd = pat.pattern_data
      let satisfied = true
      for (let r = 0; r < 5 && satisfied; r++) {
        for (let c = 0; c < 5; c++) {
          if (pd[r]?.[c]) {
            const v = card.board_data[r][c]
            if (v !== 0 && !called.has(v)) {
              satisfied = false
              break
            }
          }
        }
      }
      if (satisfied) {
        for (let r = 0; r < 5; r++) {
          for (let c = 0; c < 5; c++) {
            if (pd[r]?.[c]) matched.add(`${r}-${c}`)
          }
        }
      }
    }
    winnerPreview.value = { card, matched }
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to load card', 'error')
  } finally {
    winnerLoading.value = false
  }
}

async function triggerHalftime() {
  showHalftime.value = false
  try {
    await apiPost('game', { action: 'trigger_halftime' })
    show('Halftime alert sent to players', 'success')
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to send alert', 'error')
  }
}

onMounted(() => {
  void reloadAll()
  socket.connect()
})

onBeforeUnmount(() => {
  socket.close()
  clearCountdown()
})
</script>

<template>
  <section class="game">
    <div class="head">
      <h2>{{ hasGame ? 'Current Game' : 'New Game' }}</h2>
      <span class="conn" :class="{ on: socket.connected.value }">
        {{ socket.connected.value ? 'live' : 'offline' }}
      </span>
    </div>

    <div v-if="frequentWinners.length" class="freq">
      ⚠️ Frequent winners:
      <span v-for="w in frequentWinners" :key="w.player_name" class="freq-item">
        {{ w.player_name }} ({{ w.win_count }})
      </span>
    </div>

    <!-- ── No active game: pattern picker ── -->
    <div v-if="!hasGame" class="start">
      <p class="muted">Pick the win pattern(s), then start the game.</p>
      <div class="filters">
        <select v-model="categoryFilter">
          <option :value="null">All categories</option>
          <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <input v-model="patternSearch" placeholder="Search patterns…" />
      </div>

      <ul class="pattern-grid">
        <li
          v-for="p in filteredPatterns"
          :key="p.id"
          class="pattern"
          :class="{ selected: selectedPatternIds.includes(p.id) }"
          @click="togglePattern(p.id)"
        >
          <PatternPreview :pattern="p.pattern_data" />
          <span class="pattern-name">{{ p.name }}</span>
        </li>
      </ul>

      <button
        type="button"
        class="primary"
        :disabled="starting || selectedPatternIds.length === 0"
        @click="startGame"
      >
        {{ starting ? 'Starting…' : `Start game (${selectedPatternIds.length})` }}
      </button>
    </div>

    <!-- ── Active game ── -->
    <div v-else class="active">
      <div class="controls">
        <button type="button" class="primary big" :disabled="drawing" @click="drawNumber">
          Draw
        </button>
        <label class="delay">
          Player delay
          <input v-model.number="drawDelay" type="number" min="0" max="60" /> s
        </label>
        <button type="button" class="danger" @click="endGame">End game</button>
      </div>

      <div class="status">
        <div v-if="lastDrawn" class="last">
          <span class="last-letter">{{ lastDrawn.letter }}</span>
          <span class="last-num">{{ lastDrawn.number }}</span>
        </div>
        <span v-if="drawCountdown !== null" class="muted">
          Players see it in {{ drawCountdown }}s…
        </span>
        <span v-else-if="drawSent" class="sent">Sent to players ✓</span>
        <span class="muted">· {{ totalCalled }} / 75 drawn</span>
      </div>

      <!-- Called numbers grid -->
      <div class="called">
        <div v-for="col in 5" :key="col" class="called-col">
          <div class="called-head">{{ ['B', 'I', 'N', 'G', 'O'][col - 1] }}</div>
          <div
            v-for="n in colNums(col - 1)"
            :key="n"
            class="called-cell"
            :class="{ on: calledSet.has(n), last: lastDrawn?.number === n }"
          >
            {{ n }}
          </div>
        </div>
      </div>

      <!-- Winners -->
      <div class="winners">
        <h3>Winners ({{ winners.length }})</h3>
        <p v-if="winners.length === 0" class="muted">No winners yet.</p>
        <ul v-else class="winner-chips">
          <li v-for="id in winners" :key="id">
            <button type="button" class="winner-chip" @click="viewWinner(id)">{{ id }}</button>
          </li>
        </ul>
      </div>

      <!-- Game details -->
      <div class="details">
        <h3>Game details (markdown)</h3>
        <textarea v-model="gameDetails" rows="3" placeholder="Prizes, theme, notes…"></textarea>
        <div class="details-actions">
          <button type="button" @click="saveDetails">Save details</button>
        </div>
        <div v-if="renderedDetails" class="prose" v-html="renderedDetails" />
      </div>
    </div>

    <!-- Winner verification modal -->
    <div v-if="winnerPreview" class="modal-backdrop" @click.self="winnerPreview = null">
      <div class="modal">
        <header class="modal-head">
          <strong class="mono">{{ winnerPreview.card.id }}</strong>
          <button type="button" class="x" @click="winnerPreview = null">✕</button>
        </header>
        <p v-if="winnerPreview.card.player_name" class="muted">
          {{ winnerPreview.card.player_name }}
        </p>
        <BingoBoard
          :board="winnerPreview.card.board_data"
          :called-numbers="currentGame?.called_numbers ?? []"
          :stamps="{}"
          :highlight="winnerPreview.matched"
          :interactive="false"
        />
        <p class="muted small">Green cells complete a winning pattern.</p>
      </div>
    </div>

    <!-- End-game winner confirmation -->
    <div v-if="showEndGame" class="modal-backdrop" @click.self="showEndGame = false">
      <div class="modal narrow">
        <h3>Confirm winners to log</h3>
        <p class="muted small">Uncheck any that aren't valid before ending.</p>
        <ul class="confirm-list">
          <li v-for="id in winners" :key="id">
            <label>
              <input
                type="checkbox"
                :checked="endGameSelected.includes(id)"
                @change="toggleEndGameWinner(id)"
              />
              <span class="mono">{{ id }}</span>
            </label>
          </li>
        </ul>
        <footer class="modal-foot">
          <button type="button" @click="showEndGame = false">Cancel</button>
          <button type="button" class="danger" @click="confirmEndGame(endGameSelected)">
            End &amp; log {{ endGameSelected.length }}
          </button>
        </footer>
      </div>
    </div>

    <!-- Halftime prompt -->
    <div v-if="showHalftime" class="modal-backdrop" @click.self="showHalftime = false">
      <div class="modal narrow">
        <h3>Halftime!</h3>
        <p>35 numbers drawn. Send a halftime mini-game alert to all players?</p>
        <footer class="modal-foot">
          <button type="button" @click="showHalftime = false">Not now</button>
          <button type="button" class="primary" @click="triggerHalftime">Send alert</button>
        </footer>
      </div>
    </div>
  </section>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.conn {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #888;
}
.conn.on {
  color: #2cb67d;
}
.freq {
  margin: 8px 0 4px;
  padding: 8px 12px;
  border: 1px solid #5a4a2a;
  background: #2a2418;
  border-radius: 8px;
  font-size: 0.9rem;
}
.freq-item {
  margin-left: 8px;
}
.muted {
  color: #9a9ab0;
}
.small {
  font-size: 0.85rem;
}

/* Start panel */
.filters {
  display: flex;
  gap: 8px;
  margin: 10px 0;
}
.filters input {
  flex: 1;
}
.pattern-grid {
  list-style: none;
  padding: 0;
  margin: 0 0 16px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 10px;
}
.pattern {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 1px solid #2a2a3a;
  border-radius: 10px;
  cursor: pointer;
  background: #1c1c28;
}
.pattern.selected {
  border-color: var(--accent, #e53170);
  box-shadow: inset 0 0 0 1px var(--accent, #e53170);
}
.pattern-name {
  font-size: 0.82rem;
  text-align: center;
}

/* Active game */
.controls {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  margin: 12px 0;
}
.primary {
  background: var(--accent, #e53170);
  color: #fff;
  border-color: transparent;
}
.primary.big {
  font-size: 1.1rem;
  padding: 12px 28px;
}
.danger {
  border-color: #5a2a36;
  color: #ff9a9a;
}
.delay {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #9a9ab0;
  font-size: 0.85rem;
}
.delay input {
  width: 64px;
}
.status {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.last {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 10px;
  background: var(--accent, #e53170);
  color: #fff;
}
.last-letter {
  font-weight: 700;
}
.last-num {
  font-size: 1.4rem;
  font-weight: 700;
}
.sent {
  color: #2cb67d;
}
.called {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
  max-width: 360px;
}
.called-col {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.called-head {
  text-align: center;
  font-weight: 700;
  color: var(--accent, #e53170);
}
.called-cell {
  text-align: center;
  font-size: 0.8rem;
  padding: 3px 0;
  border-radius: 5px;
  background: #1c1c28;
  color: #6f6f86;
}
.called-cell.on {
  background: #2a2030;
  color: #f4f4f8;
  font-weight: 600;
}
.called-cell.last {
  background: var(--accent, #e53170);
  color: #fff;
}
.winners {
  margin-top: 20px;
}
.winner-chips {
  list-style: none;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0;
  margin: 0;
}
.winner-chip {
  font-family: ui-monospace, monospace;
  font-weight: 600;
  border-color: #2cb67d;
  color: #9ff0cc;
}
.details {
  margin-top: 20px;
}
.details textarea {
  width: 100%;
  font: inherit;
  padding: 8px 12px;
  border: 1px solid #3a3a4a;
  border-radius: 8px;
  background: #1c1c28;
  color: #f4f4f8;
  resize: vertical;
}
.details-actions {
  margin: 8px 0;
}
.prose {
  color: #d4d4e0;
  line-height: 1.5;
  border-top: 1px solid #26263a;
  padding-top: 10px;
}

/* Modals */
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
  gap: 14px;
  align-items: center;
  max-height: 90vh;
  overflow: auto;
}
.modal.narrow {
  align-items: stretch;
  min-width: 280px;
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
.confirm-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.confirm-list label {
  display: flex;
  gap: 8px;
  align-items: center;
}
.mono {
  font-family: ui-monospace, monospace;
}
</style>
