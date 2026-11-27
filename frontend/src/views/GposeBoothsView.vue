<script setup lang="ts">
/**
 * Public list of the open gpose booths. Each opens its own sign-up page.
 */
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/ui/EmptyState.vue'
import { gposePriceLabel, useGposeStore } from '@/stores/gpose'
import type { PublicGposeBooth } from '@/api/client'

const gpose = useGposeStore()
const router = useRouter()

onMounted(() => gpose.loadPublicBooths())

function openBooth(b: PublicGposeBooth): void {
  void router.push({ name: 'gpose-booth', params: { id: String(b.id) } })
}

/** The cheapest price a booth asks, for its card. */
function fromPrice(b: PublicGposeBooth): string {
  const prices = b.types.map((t) => t.price).filter((p) => p > 0)
  return prices.length ? `From ${gposePriceLabel(Math.min(...prices))}` : ''
}
</script>

<template>
  <div>
    <div class="topbar">
      <button type="button" class="btn-neutral btn-sm" @click="router.push({ name: 'home' })">
        <font-awesome-icon :icon="['fas', 'arrow-left']" /> Home
      </button>
      <h2><font-awesome-icon :icon="['fad', 'camera-retro']" /> Gpose Booths</h2>
      <span></span>
    </div>
    <div class="tab-body content-container">
      <LoadingSpinner v-if="gpose.publicLoading" block label="Loading booths..." />
      <div v-else-if="gpose.publicBooths.length" class="card-grid card-grid--center">
        <div
          v-for="b in gpose.publicBooths"
          :key="b.id"
          class="media-card"
          role="button"
          tabindex="0"
          @click="openBooth(b)"
          @keydown.enter="openBooth(b)"
          @keydown.space.prevent="openBooth(b)"
        >
          <div class="media-card-body">
            <h3>{{ b.title }}</h3>
            <p class="text-sm text-muted">
              {{ b.types.map((t) => t.name).join(', ') }}
            </p>
            <p v-if="fromPrice(b)" class="text-sm">{{ fromPrice(b) }}</p>
            <p v-if="b.festivalName" class="text-sm text-muted">{{ b.festivalName }}</p>
          </div>
        </div>
      </div>
      <EmptyState
        v-else
        :icon="['fad', 'camera-retro']"
        text="No gpose booths are open right now."
        hint="Check back during the next festival."
      />
    </div>
  </div>
</template>
