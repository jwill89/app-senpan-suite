<script setup lang="ts">
/** Small read-only 5×5 grid visualizing a win pattern (true = required cell). */
defineProps<{
  pattern: boolean[][]
  /** Cell size in px. */
  size?: number
}>()
</script>

<template>
  <div class="pp" :style="{ '--cell': `${size ?? 14}px` }">
    <div v-for="(row, r) in pattern" :key="r" class="pp-row">
      <span v-for="(on, c) in row" :key="c" class="pp-cell" :class="{ on }" />
    </div>
  </div>
</template>

<style scoped>
.pp {
  display: inline-flex;
  flex-direction: column;
  gap: 2px;
}
.pp-row {
  display: flex;
  gap: 2px;
}
.pp-cell {
  width: var(--cell);
  height: var(--cell);
  border-radius: 3px;
  background: #2a2a3a;
}
.pp-cell.on {
  background: var(--accent, #e53170);
}
</style>
