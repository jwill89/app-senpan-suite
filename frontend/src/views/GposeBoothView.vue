<script setup lang="ts">
/**
 * One gpose booth's public page (/gpose-booths/:id): what it offers, the sign-up
 * form, and where the visitor stands.
 *
 * The visitor is whoever this device signed up as (lib/signups). While one of their
 * sign-ups waits, the page reads them every half minute, so a staff call reaches
 * them, and alerts them (useGposeAlerts) until they dismiss it. A booth filed under
 * a published festival links to that festival's dashboard, which also alerts a
 * timeslot booking as its time comes.
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/ui/EmptyState.vue'
import GposeBoothPanel from '@/components/player/GposeBoothPanel.vue'
import { useGposeAlerts } from '@/composables/useGposeAlerts'
import { api, unwrap, type PublicGposeSignup } from '@/api/client'
import { savedGposeSignup } from '@/lib/signups'
import { useGposeStore } from '@/stores/gpose'

const props = defineProps<{ id: string }>()

const POLL_MS = 30_000

const router = useRouter()
const gpose = useGposeStore()
const turnstileSiteKey = ref('')
const name = ref('')
const world = ref('')
const mine = ref<PublicGposeSignup[]>([])

const boothId = computed(() => Number(props.id))
const booth = computed(() => gpose.publicBooth)

async function readMine(): Promise<void> {
  if (!name.value) {
    mine.value = []
    return
  }
  const found = await gpose.lookUp(boothId.value, name.value, world.value)
  if (found) {
    mine.value = found
  }
}

async function load(): Promise<void> {
  const saved = savedGposeSignup(boothId.value)
  name.value = saved?.name ?? ''
  world.value = saved?.world ?? ''
  await gpose.loadPublicBooth(boothId.value)
  await readMine()
}

let poll: ReturnType<typeof setInterval> | null = null
onMounted(async () => {
  void load()
  // Only while something waits: nothing else here changes on its own.
  poll = setInterval(() => {
    if (mine.value.some((s) => s.status === 'waiting')) {
      void readMine()
    }
  }, POLL_MS)
  try {
    turnstileSiteKey.value = (await unwrap(api.GET('/config'))).turnstileSiteKey
  } catch {
    turnstileSiteKey.value = ''
  }
})
onBeforeUnmount(() => {
  if (poll !== null) {
    clearInterval(poll)
  }
})
watch(boothId, () => void load())

useGposeAlerts(
  () => (booth.value ? [{ boothTitle: booth.value.title, signups: mine.value }] : []),
  { slotAlerts: false },
)

async function onSignedUp(_s: PublicGposeSignup, n: string, w: string): Promise<void> {
  name.value = n
  world.value = w
  await Promise.all([gpose.loadPublicBooth(boothId.value, true), readMine()])
}
</script>

<template>
  <div>
    <div class="topbar">
      <button
        type="button"
        class="btn-neutral btn-sm"
        @click="router.push({ name: 'gpose-booths' })"
      >
        <font-awesome-icon :icon="['fas', 'arrow-left']" /> Gpose Booths
      </button>
      <h2>
        <font-awesome-icon :icon="['fad', 'camera-retro']" />
        {{ booth?.title ?? 'Gpose Booth' }}
      </h2>
      <span></span>
    </div>
    <div class="tab-body content-container">
      <LoadingSpinner v-if="gpose.publicLoading && !booth" block label="Loading the booth..." />
      <EmptyState
        v-else-if="gpose.publicNotFound || !booth"
        :icon="['fad', 'camera-retro']"
        text="That gpose booth isn't open."
        hint="It may have closed for now."
      />
      <template v-else>
        <p v-if="booth.festivalPath" class="text-sm">
          Part of {{ booth.festivalName }} -
          <RouterLink :to="{ name: 'festival-dashboard', params: { id: booth.festivalPath } }">
            open the festival dashboard
          </RouterLink>
          to follow your booking with the rest of the festival.
        </p>
        <GposeBoothPanel
          :booth="booth"
          :signups="mine"
          :visitor-name="name"
          :visitor-world="world"
          :turnstile-site-key="turnstileSiteKey"
          @signed-up="onSignedUp"
        />
      </template>
    </div>
  </div>
</template>
