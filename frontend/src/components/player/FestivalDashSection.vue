<script setup lang="ts">
/**
 * One section of the festival dashboard: a full-width panel whose heading is the
 * button that folds it away. Folded, the heading still shows the section's tally
 * (stamps collected, draws left), so a visitor can put away what they have done
 * and keep the rest in view.
 *
 * The body is hidden rather than unmounted, so the plan keeps its zoom and the
 * drum its spin when a section is folded and opened again.
 */
defineProps<{
  /** The section's key: its element is `festival-<id>`, its body `festival-<id>-body`. */
  id: string
  title: string
  /** A duotone icon name. */
  icon: string
  /** The tally beside the title, kept short ("2 of 4"). */
  summary?: string
  collapsed: boolean
  /** The section a phone is showing (see FestivalDashboardView). */
  current: boolean
}>()

const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <section
    :id="`festival-${id}`"
    class="festival-dash-section"
    :class="{ 'is-current': current, 'is-collapsed': collapsed }"
    :aria-labelledby="`festival-${id}-title`"
  >
    <h3 :id="`festival-${id}-title`" class="festival-dash-section-title">
      <button
        type="button"
        class="festival-dash-section-toggle"
        :aria-expanded="!collapsed"
        :aria-controls="`festival-${id}-body`"
        @click="emit('toggle')"
      >
        <span class="festival-dash-section-icon">
          <font-awesome-icon :icon="['fad', icon]" />
        </span>
        <span class="festival-dash-section-name">{{ title }}</span>
        <!-- The comma keeps the two apart when read aloud: the spans touch in the
             markup, so the name would otherwise run on as "Festival Map17 stalls". -->
        <span v-if="summary" class="festival-dash-section-summary"
          ><span class="sr-only">, </span>{{ summary }}</span
        >
        <font-awesome-icon :icon="['fas', 'chevron-down']" class="festival-dash-section-chevron" />
      </button>
    </h3>
    <div v-show="!collapsed" :id="`festival-${id}-body`" class="festival-dash-section-body">
      <slot />
    </div>
  </section>
</template>
