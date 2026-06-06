<script setup lang="ts">
/**
 * Presentational 5×5 bingo board. Knows nothing about the API or sockets —
 * it renders the grid and emits stamp toggles. Reusable by the player view,
 * the admin winner-verification modal, and card previews.
 */
import { computed } from 'vue'

const props = defineProps<{
  board: number[][]
  calledNumbers: number[]
  /** Stamp state keyed by "row-col". */
  stamps: Record<string, boolean>
  /** When false, cells are display-only (e.g. admin preview). */
  interactive?: boolean
  /** Cells (keyed "row-col") to highlight, e.g. a winning pattern. */
  highlight?: Set<string>
}>()

const emit = defineEmits<{ toggle: [row: number, col: number] }>()

const headers = ['B', 'I', 'N', 'G', 'O']
const calledSet = computed(() => new Set(props.calledNumbers))

function isStamped(r: number, c: number): boolean {
  return !!props.stamps[`${r}-${c}`]
}

function onCell(r: number, c: number) {
  if (props.interactive !== false) emit('toggle', r, c)
}
</script>

<template>
  <div class="board">
    <div class="board-row board-header">
      <div v-for="h in headers" :key="h" class="board-header-cell">{{ h }}</div>
    </div>
    <div v-for="(row, r) in board" :key="r" class="board-row">
      <button
        v-for="(cell, c) in row"
        :key="c"
        type="button"
        class="board-cell"
        :class="{
          free: cell === 0,
          called: cell !== 0 && calledSet.has(cell),
          stamped: isStamped(r, c),
          matched: highlight?.has(`${r}-${c}`),
        }"
        :disabled="interactive === false"
        @click="onCell(r, c)"
      >
        <span class="board-cell-num">{{ cell === 0 ? 'FREE' : cell }}</span>
        <span v-if="isStamped(r, c)" class="board-cell-stamp" aria-hidden="true"></span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.board {
  display: inline-grid;
  gap: 6px;
  user-select: none;
}
.board-row {
  display: grid;
  grid-template-columns: repeat(5, 64px);
  gap: 6px;
}
.board-header-cell {
  text-align: center;
  font-weight: 700;
  font-size: 1.4rem;
  color: var(--accent, #e53170);
}
.board-cell {
  position: relative;
  aspect-ratio: 1;
  border: 2px solid #2a2a3a;
  border-radius: 10px;
  background: #1c1c28;
  color: #f4f4f8;
  font-size: 1.25rem;
  font-weight: 600;
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: background 0.12s, border-color 0.12s;
}
.board-cell:disabled {
  cursor: default;
}
.board-cell.called {
  border-color: var(--accent, #e53170);
  background: #2a2030;
}
.board-cell.free {
  color: var(--accent, #e53170);
  font-size: 0.85rem;
}
.board-cell.matched {
  border-color: #2cb67d;
  background: #1f3a30;
  box-shadow: inset 0 0 0 2px #2cb67d;
}
.board-cell-stamp {
  position: absolute;
  inset: 8px;
  border-radius: 50%;
  background: var(--stamp-color, rgba(229, 49, 112, 0.55));
  pointer-events: none;
}
</style>
