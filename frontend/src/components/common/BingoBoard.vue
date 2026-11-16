<script setup lang="ts">
/**
 * Bingo board (5x5) with the B-I-N-G-O header. Used in three places:
 *  - Player view: interactive, shows stamp marks (mode="player").
 *  - Winner verification: static, highlights pattern-hit cells (mode="preview").
 *  - Card preview: static board (mode="preview").
 *
 * Renders the `.bingo-board` / `.bingo-board__header` / `.bingo-board__grid` /
 * `.bingo-board__cell` markup that the board styles target.
 */
import type { StyleValue } from 'vue'
import CornerFlourish from '@/components/common/CornerFlourish.vue'

const props = defineProps<{
  board: number[][]
  /** 'player' = interactive + stamps; 'preview' = static. */
  mode?: 'player' | 'preview'
  /** Adds the `.bingo-board--preview` modifier class (used in modals). */
  preview?: boolean
  /** Player mode: is the cell stamped? */
  isStamped?: (ri: number, ci: number) => boolean
  /** Player mode: per-cell class builder. */
  cellClass?: (ri: number, ci: number, cell: number) => (string | false)[]
  /** Preview mode: should this cell be highlighted as a pattern hit? */
  isCellMatch?: (ri: number, ci: number) => boolean
  /** Player mode: inline style for the primary stamp mark. */
  stampMarkStyle?: StyleValue
  /** Player mode: emoji to show for stamped cells. */
  stampEmoji?: string
  /** Player mode: custom stamp shape id ('custom' shows the image). */
  stampShape?: string
  /** Player mode: custom stamp image data URL. */
  customStampImage?: string | null
  /**
   * Player mode: optional secondary-stamp style (a plain coloured circle). When
   * provided, stamped cells that are NOT part of a win pattern render this
   * instead of the primary stamp (no emoji/image). Omit to use the primary stamp
   * everywhere.
   */
  secondaryStampStyle?: StyleValue
  /** Player mode: is cell [ri,ci] part of an active win pattern? */
  isWinningPatternCell?: (ri: number, ci: number) => boolean
}>()

const emit = defineEmits<{ cellClick: [ri: number, ci: number, cell: number] }>()

/**
 * Whether the secondary stamp (plain coloured circle) applies to a stamped cell:
 * only when a secondary style is supplied and the cell falls outside every active
 * win pattern.
 */
function usesSecondaryStamp(ri: number, ci: number): boolean {
  return (
    !!props.secondaryStampStyle &&
    !!props.isStamped &&
    props.isStamped(ri, ci) &&
    !!props.isWinningPatternCell &&
    !props.isWinningPatternCell(ri, ci)
  )
}
</script>

<template>
  <div class="bingo-board" :class="{ 'bingo-board--preview': preview }">
    <!-- Decorative flourishes in each corner (player view only; captured into the
         exported card image). Mirrored per-corner via CSS transforms. -->
    <template v-if="mode === 'player'">
      <CornerFlourish class="corner-flourish corner-flourish--tl" />
      <CornerFlourish class="corner-flourish corner-flourish--tr" />
      <CornerFlourish class="corner-flourish corner-flourish--bl" />
      <CornerFlourish class="corner-flourish corner-flourish--br" />
    </template>
    <div class="bingo-board__header">
      <span>B</span><span>I</span><span>N</span><span>G</span><span>O</span>
    </div>
    <div class="bingo-board__grid">
      <template v-for="(row, ri) in board" :key="ri">
        <!-- Player mode: interactive cells with stamps -->
        <template v-if="mode === 'player'">
          <div
            v-for="(cell, ci) in row"
            :key="ri + '-' + ci"
            :class="
              cellClass
                ? cellClass(ri, ci, cell)
                : ['bingo-board__cell', cell === 0 ? 'is-free' : '']
            "
            role="button"
            tabindex="0"
            :aria-pressed="isStamped ? isStamped(ri, ci) : false"
            @click="emit('cellClick', ri, ci, cell)"
            @keydown.enter="emit('cellClick', ri, ci, cell)"
            @keydown.space.prevent="emit('cellClick', ri, ci, cell)"
          >
            <span class="bingo-board__cell-num">{{ cell === 0 ? 'FREE' : cell }}</span>
            <div
              class="bingo-board__stamp-mark"
              :style="
                isStamped && isStamped(ri, ci)
                  ? usesSecondaryStamp(ri, ci)
                    ? secondaryStampStyle
                    : stampMarkStyle
                  : {}
              "
            >
              <!-- Secondary stamp is a plain coloured circle (no icon). -->
              <template v-if="isStamped && isStamped(ri, ci) && !usesSecondaryStamp(ri, ci)">
                <img
                  v-if="stampShape === 'custom' && customStampImage"
                  :src="customStampImage"
                  class="bingo-board__stamp-custom-img"
                  alt="stamp"
                />
                <template v-else>{{ stampEmoji }}</template>
              </template>
            </div>
          </div>
        </template>

        <!-- Preview mode: static cells, optional pattern-hit highlight -->
        <template v-else>
          <div
            v-for="(cell, ci) in row"
            :key="ri + '-' + ci"
            :class="[
              'bingo-board__cell',
              cell === 0 ? 'is-free' : '',
              isCellMatch && isCellMatch(ri, ci) ? 'bingo-board__cell--pattern-hit' : '',
              'bingo-board__cell--static',
            ]"
          >
            <span class="bingo-board__cell-num">{{ cell === 0 ? 'FREE' : cell }}</span>
            <div v-if="cell === 0" class="bingo-board__stamp-mark"></div>
          </div>
        </template>
      </template>
    </div>
  </div>
</template>
