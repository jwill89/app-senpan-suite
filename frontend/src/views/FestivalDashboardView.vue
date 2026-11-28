<script setup lang="ts">
/**
 * The festival dashboard (/festival/:id): one page where a visitor follows and
 * takes part in everything filed under a festival - the plan, the stamp rally,
 * the garapon, the raffles, the plaque board and the gpose booths - without being
 * sent off to a page for each.
 *
 * A gpose sign-up alerts the visitor here until they dismiss it (useGposeAlerts):
 * when staff call them up, and as a timeslot booking's time comes. While one waits,
 * or a finished shoot's gpose may still come, the page reads the sign-ups every half
 * minute so a call or the gpose reaches them.
 *
 * A wide screen shows every section at once, one under another at the page's full
 * width - the stamp card and the drum need the room to be read - and each folds
 * away from its heading. A phone shows one at a time, picked from the tiles, which
 * double as the at-a-glance summary on both - stamps collected, draws left,
 * raffles entered.
 *
 * The state and every action live in the festival-dashboard store. The stamp card
 * and the drum are the same panels their own pages render.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import EmptyState from '@/components/common/ui/EmptyState.vue'
import FestivalMapWidget from '@/components/common/ui/FestivalMapWidget.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import MarkdownText from '@/components/common/MarkdownText.vue'
import TurnstileWidget from '@/components/common/TurnstileWidget.vue'
import WorldPicker from '@/components/common/ui/WorldPicker.vue'
import FestivalDashSection from '@/components/player/FestivalDashSection.vue'
import FestivalEmaSection from '@/components/player/FestivalEmaSection.vue'
import FestivalRaffleCard from '@/components/player/FestivalRaffleCard.vue'
import GposeBoothPanel from '@/components/player/GposeBoothPanel.vue'
import { useGposeAlerts } from '@/composables/useGposeAlerts'
import GaraponPanel from '@/components/player/GaraponPanel.vue'
import StampCardPanel from '@/components/player/StampCardPanel.vue'
import { useFestivalDashboardStore } from '@/stores/festivalDashboard'
import { useGaraponsStore } from '@/stores/garapons'
import { useStampRalliesStore } from '@/stores/stampRallies'
import { api, unwrap } from '@/api/client'
import { formatEventRange } from '@/lib/festivalmap'
import { participantLabel } from '@/lib/participant'

// The route param is the festival's shortcode OR its numeric id - the server
// resolves either, so it is passed through as typed.
const props = defineProps<{ id: string }>()

type Section = 'map' | 'stamps' | 'garapon' | 'raffles' | 'ema' | 'gpose'

const router = useRouter()
const store = useFestivalDashboardStore()
const stampRallies = useStampRalliesStore()
const garapons = useGaraponsStore()

const turnstileSiteKey = ref('')
/** The section a phone shows; on a wide screen, the one last jumped to. */
const current = ref<Section>('map')
/** The sections folded away on a wide screen. */
const collapsed = ref(new Set<Section>())

function toggle(section: Section): void {
  if (!collapsed.value.delete(section)) {
    collapsed.value.add(section)
  }
}

onMounted(async () => {
  void store.open(props.id)
  try {
    turnstileSiteKey.value = (await unwrap(api.GET('/config'))).turnstileSiteKey
  } catch {
    turnstileSiteKey.value = '' // config probe failed -> behave as if disabled
  }
})
// An in-app link from one festival to another reuses this component.
watch(
  () => props.id,
  (id) => {
    current.value = 'map'
    collapsed.value.clear()
    void store.open(id)
  },
)

/**
 * Coming back to the tab re-reads the visitor's standing, so a stall that opened
 * or a raffle that was drawn while they were away shows without a reload. Not
 * mid-spin: the drum commits its draw when the ball lands, and a refresh before
 * then would show the prize first.
 */
function onVisible(): void {
  if (document.visibilityState !== 'visible' || garapons.drawing) {
    return
  }
  if (Date.now() - store.loadedAt > 60_000) {
    void store.refresh()
  }
}
onMounted(() => document.addEventListener('visibilitychange', onVisible))
onBeforeUnmount(() => document.removeEventListener('visibilitychange', onVisible))

// -- Gpose booths: keeping the sign-ups fresh, and the alerts ------------------------

const GPOSE_POLL_MS = 30_000
let gposePoll: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  gposePoll = setInterval(() => {
    if (store.gposeWaiting || store.gposeAwaitingPhoto) {
      void store.refreshGpose()
    }
  }, GPOSE_POLL_MS)
})
onBeforeUnmount(() => {
  if (gposePoll !== null) {
    clearInterval(gposePoll)
  }
})
useGposeAlerts(
  () => store.gposeBooths.map((b) => ({ boothTitle: b.booth.title, signups: b.signups })),
  { slotAlerts: true },
)

function onGposeSignedUp(_signup: unknown, name: string, world: string): void {
  void store.afterGposeSignup(name, world)
}

const data = computed(() => store.dashboard)
const rally = computed(() => data.value?.rally ?? null)
const garapon = computed(() => data.value?.garapon ?? null)
const raffles = computed(() =>
  // Running ones first: they are the ones a visitor can still do something about.
  [...(data.value?.raffles ?? [])].sort((a, b) => Number(b.running) - Number(a.running)),
)

// -- The festival's days ----------------------------------------------------------

/** Each day as its name and its hours, set apart so the name can lead. */
const days = computed(() =>
  (store.map?.times ?? []).map((t, i) => ({
    label: t.label.trim() || `Day ${String(i + 1)}`,
    when: formatEventRange(t.start, t.end),
  })),
)

// -- The tiles ------------------------------------------------------------------

const stampTally = computed(() => {
  const p = stampRallies.cardProgress
  if (!store.card) {
    return rally.value?.signupOpen ? 'Get a card' : 'No card yet'
  }
  if (p.byType) {
    const need = p.food.required + p.game.required
    const have =
      Math.min(p.food.collected, p.food.required) + Math.min(p.game.collected, p.game.required)
    return store.card.completed ? 'Complete' : `${have} of ${need}`
  }
  return store.card.completed ? 'Complete' : `${p.collected} of ${p.total}`
})

const drawTally = computed(() => {
  const g = garapon.value
  if (!g) {
    return ''
  }
  if (g.garapon.status === 'closed') {
    return 'Closed'
  }
  if (g.player) {
    return `${garapons.drawsRemaining} left`
  }
  if (g.hasPairedLink) {
    return `${g.pairedDrawsLeft} waiting`
  }
  return 'With a card'
})

const raffleTally = computed(() => {
  const mine = store.enteredRaffles.length
  if (mine) {
    return `In ${mine}`
  }
  const open = store.runningRaffles.length
  return open ? `${open} open` : 'Drawn'
})

const emaTally = computed(() => {
  const e = data.value?.ema
  if (!e) {
    return ''
  }
  if (e.hasPlaqueThisWindow) {
    return 'Hung today'
  }
  return e.currentWindowId ? 'Open today' : 'Not today'
})

const gposeTally = computed(() => {
  const mine = store.gposeBooths.flatMap((b) => b.signups)
  const waiting = mine.filter((s) => s.status === 'waiting')
  const next = waiting.find((s) => !s.waitlisted)
  if (next) {
    return next.slotStart ? 'Booked' : `#${next.position} in line`
  }
  if (waiting.length) {
    return 'On the waitlist'
  }
  if (mine.some((s) => s.photoUrl)) {
    return 'Gpose ready'
  }
  const open = store.gposeBooths.filter((b) => b.open).length
  return open === 1 ? 'Open' : `${open} open`
})

const tiles = computed(() => {
  const list: { key: Section; label: string; icon: string; value: string }[] = [
    {
      key: 'map',
      label: 'Map',
      icon: 'map-location-dot',
      value: `${store.map?.stalls.length ?? 0} stalls`,
    },
  ]
  if (rally.value) {
    list.push({ key: 'stamps', label: 'Stamp Rally', icon: 'stamp', value: stampTally.value })
  }
  if (garapon.value) {
    list.push({ key: 'garapon', label: 'Garapon', icon: 'ferris-wheel', value: drawTally.value })
  }
  if (raffles.value.length) {
    list.push({ key: 'raffles', label: 'Raffles', icon: 'ticket', value: raffleTally.value })
  }
  if (data.value?.ema) {
    list.push({ key: 'ema', label: 'Ema Plaques', icon: 'torii-gate', value: emaTally.value })
  }
  if (store.gposeBooths.length) {
    list.push({ key: 'gpose', label: 'Gpose Booth', icon: 'camera-retro', value: gposeTally.value })
  }
  return list
})

/** Each section's tally, for its heading as well as its tile. */
const tally = computed(
  () => Object.fromEntries(tiles.value.map((t) => [t.key, t.value])) as Record<Section, string>,
)

/** Shows a section (a phone) or brings it into view, unfolded (a wide screen). */
async function goTo(section: Section, anchor?: string): Promise<void> {
  current.value = section
  collapsed.value.delete(section)
  await nextTick()
  document
    .getElementById(anchor ?? `festival-${section}`)
    ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// -- Who is visiting --------------------------------------------------------------

const editingVisitor = ref(false)
const visitorName = ref('')
const visitorWorld = ref('')

const visitorLabel = computed(() =>
  store.visitorName ? participantLabel(store.visitorName, store.visitorWorld) : '',
)

function editVisitor(): void {
  visitorName.value = store.visitorName
  visitorWorld.value = store.visitorWorld
  editingVisitor.value = true
}

async function saveVisitor(): Promise<void> {
  if (await store.setVisitor(visitorName.value, visitorWorld.value)) {
    editingVisitor.value = false
  }
}

// -- Stamp rally: sign up / find a card / stamp from the plan ----------------------

const signupName = ref('')
const signupWorld = ref('')
const signupTurnstile = ref('')
const signupWidget = useTemplateRef<InstanceType<typeof TurnstileWidget>>('signupWidget')

// The sign-up form starts from whoever the visitor said they are.
watch(
  () => [store.visitorName, store.visitorWorld] as const,
  ([name, world]) => {
    if (!signupName.value) {
      signupName.value = name
    }
    if (!signupWorld.value) {
      signupWorld.value = world
    }
  },
  { immediate: true },
)

async function signUp(): Promise<void> {
  const ok = await store.signUp(signupName.value, signupWorld.value, signupTurnstile.value)
  if (!ok) {
    // Single-use: a refused attempt (a taken name, most often) needs a fresh one.
    signupTurnstile.value = ''
    signupWidget.value?.reset()
  }
}

function findCard(): void {
  void store.findCard(signupName.value, signupWorld.value)
}

/** One code box for the stall panel: only one stall's panel is open at a time. */
const stallCode = ref('')
async function stampFromStall(): Promise<void> {
  if (await store.stamp(stallCode.value)) {
    stallCode.value = ''
  }
}

function forget(): void {
  editingVisitor.value = false
  void store.forgetDevice()
}

function goBack(): void {
  void router.push({ name: 'festival-maps' })
}
</script>

<template>
  <div>
    <div class="topbar">
      <button type="button" class="btn-neutral btn-sm" @click="goBack">
        <font-awesome-icon :icon="['fas', 'arrow-left']" /> Festivals
      </button>
      <h2>
        <font-awesome-icon :icon="['fad', 'map-location-dot']" />
        {{ store.map?.title ?? 'Festival' }}
      </h2>
      <span></span>
    </div>

    <div class="tab-body content-container festival-dash">
      <LoadingSpinner v-if="store.loading && !store.map" block label="Loading the festival..." />

      <EmptyState
        v-else-if="store.notFound || !store.map"
        :icon="['fad', 'map-location-dot']"
        text="That festival isn't available."
        hint="It may not be published yet, or the festival is over."
      />

      <template v-else>
        <!-- The festival, and who is visiting it -->
        <header class="festival-dash-header">
          <div class="festival-dash-when">
            <span v-if="store.map.isActive" class="status-badge status-badge-open m-0">
              Happening now
            </span>
            <dl v-if="days.length" class="festival-dash-days">
              <div v-for="(day, i) in days" :key="i" class="festival-dash-day">
                <dt>
                  <font-awesome-icon :icon="['fad', 'calendar-days']" class="text-highlight" />
                  {{ day.label }}
                </dt>
                <dd>{{ day.when }}</dd>
              </div>
            </dl>
          </div>

          <details v-if="store.map.description" class="festival-dash-about">
            <summary>About this festival</summary>
            <MarkdownText class="game-details" :source="store.map.description" />
          </details>

          <div class="festival-dash-visitor">
            <template v-if="visitorLabel && !editingVisitor">
              <span>
                <font-awesome-icon :icon="['fad', 'user']" class="text-highlight" /> Visiting as
                <strong>{{ visitorLabel }}</strong>
              </span>
              <span class="festival-dash-visitor-actions">
                <button
                  v-if="!store.identityFromCard"
                  type="button"
                  class="btn-neutral btn-sm"
                  @click="editVisitor"
                >
                  Change
                </button>
                <button type="button" class="btn-caution btn-sm" @click="forget">
                  Forget this device
                </button>
              </span>
            </template>
            <form v-else class="festival-dash-form" @submit.prevent="saveVisitor">
              <p class="text-sm m-0">
                Who's visiting? Your character name finds your raffle entries and fills in your
                sign-ups. It's kept on this device only.
              </p>
              <input
                v-model="visitorName"
                placeholder="Character name"
                aria-label="Character name"
                autocomplete="off"
              />
              <WorldPicker v-model="visitorWorld" />
              <div class="form-actions">
                <button type="submit" class="btn-confirm">Continue</button>
                <button
                  v-if="editingVisitor"
                  type="button"
                  class="btn-neutral"
                  @click="editingVisitor = false"
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </header>

        <!-- The tiles: a summary on every screen, the section picker on a phone. -->
        <nav class="festival-dash-tiles" aria-label="Festival sections">
          <button
            v-for="tile in tiles"
            :key="tile.key"
            type="button"
            class="festival-dash-tile"
            :class="{ 'is-current': current === tile.key }"
            :aria-current="current === tile.key ? 'true' : undefined"
            @click="goTo(tile.key)"
          >
            <!-- Wrapped, so the icon-button rule in base.css (inline-flex for a
                 button whose direct child is an icon) leaves the tile's grid be. -->
            <span class="festival-dash-tile-icon">
              <font-awesome-icon :icon="['fad', tile.icon]" />
            </span>
            <span class="festival-dash-tile-label">{{ tile.label }}</span>
            <span class="festival-dash-tile-value">{{ tile.value }}</span>
          </button>
        </nav>

        <div class="festival-dash-sections">
          <!-- The plan -->
          <FestivalDashSection
            id="map"
            title="Festival Map"
            icon="map-location-dot"
            :summary="tally.map"
            :collapsed="collapsed.has('map')"
            :current="current === 'map'"
            @toggle="toggle('map')"
          >
            <FestivalMapWidget
              :map="store.map"
              :collected-occupant-ids="store.collectedOccupantIds"
            >
              <template #rally-action="{ collected }">
                <form
                  v-if="store.card && !collected"
                  class="stamp-entry mt-8"
                  @submit.prevent="stampFromStall"
                >
                  <input
                    v-model="stallCode"
                    class="stamp-entry-input"
                    placeholder="Stamp password"
                    aria-label="Stamp password for this stall"
                    autocomplete="off"
                    :disabled="stampRallies.submitting"
                  />
                  <button
                    type="submit"
                    class="btn-confirm btn-sm"
                    :disabled="stampRallies.submitting || !stallCode.trim()"
                  >
                    <font-awesome-icon :icon="['fas', 'stamp']" /> Stamp
                  </button>
                </form>
                <button
                  v-else-if="!store.card && rally"
                  type="button"
                  class="link-btn"
                  @click="goTo('stamps')"
                >
                  {{ rally.signupOpen ? 'Get a stamp card' : 'Find your stamp card' }}
                </button>
              </template>
              <template #raffle-action="{ occupant }">
                <button
                  v-if="occupant.raffle"
                  type="button"
                  class="link-btn"
                  @click="goTo('raffles', `festival-raffle-${occupant.raffle.id}`)"
                >
                  See the raffle
                </button>
              </template>
              <template #gpose-action="{ occupant }">
                <button
                  v-if="occupant.gposeBooth"
                  type="button"
                  class="link-btn"
                  @click="goTo('gpose', `gpose-booth-${occupant.gposeBooth.id}`)"
                >
                  Sign up for a gpose
                </button>
              </template>
            </FestivalMapWidget>
          </FestivalDashSection>

          <!-- The stamp rally -->
          <FestivalDashSection
            v-if="rally"
            id="stamps"
            :title="rally.title"
            icon="stamp"
            :summary="tally.stamps"
            :collapsed="collapsed.has('stamps')"
            :current="current === 'stamps'"
            @toggle="toggle('stamps')"
          >
            <StampCardPanel v-if="store.card" :token="store.cardToken" :show-details="false" />

            <template v-else>
              <MarkdownText
                v-if="rally.details"
                class="game-details mb-16"
                :source="rally.details"
              />
              <form v-if="rally.signupOpen" class="festival-dash-form" @submit.prevent="signUp">
                <p class="m-0">
                  Get a stamp card: visit the marked stalls, enter each stall's password here, and
                  collect a prize when your card is done.
                </p>
                <input
                  v-model="signupName"
                  placeholder="Character name"
                  aria-label="Character name"
                  autocomplete="off"
                />
                <WorldPicker v-model="signupWorld" />
                <TurnstileWidget
                  v-if="turnstileSiteKey"
                  ref="signupWidget"
                  :site-key="turnstileSiteKey"
                  @verified="signupTurnstile = $event"
                  @expired="signupTurnstile = ''"
                  @error="signupTurnstile = ''"
                />
                <div class="form-actions">
                  <button
                    type="submit"
                    class="btn-confirm"
                    :disabled="
                      stampRallies.submitting ||
                      !signupName.trim() ||
                      !signupWorld.trim() ||
                      (!!turnstileSiteKey && !signupTurnstile)
                    "
                  >
                    <LoadingSpinner v-if="stampRallies.submitting" label="Signing up..." />
                    <template v-else>Get my stamp card</template>
                  </button>
                  <button
                    type="button"
                    class="btn-view"
                    :disabled="store.busy || !signupName.trim() || !signupWorld.trim()"
                    @click="findCard"
                  >
                    I already have one
                  </button>
                </div>
              </form>
              <div v-else class="festival-dash-form">
                <p class="m-0">
                  Staff hand out the cards for this rally. Already have one? Open the card link you
                  were given and choose <strong>Open the festival dashboard</strong>, or find it by
                  the name it was issued under.
                </p>
                <input
                  v-model="signupName"
                  placeholder="Character name"
                  aria-label="Character name"
                  autocomplete="off"
                />
                <WorldPicker v-model="signupWorld" />
                <div class="form-actions">
                  <button
                    type="button"
                    class="btn-view"
                    :disabled="store.busy || !signupName.trim() || !signupWorld.trim()"
                    @click="findCard"
                  >
                    Find my card
                  </button>
                </div>
              </div>
            </template>
          </FestivalDashSection>

          <!-- The garapon -->
          <FestivalDashSection
            v-if="garapon"
            id="garapon"
            :title="garapon.garapon.title"
            icon="ferris-wheel"
            :summary="tally.garapon"
            :collapsed="collapsed.has('garapon')"
            :current="current === 'garapon'"
            @toggle="toggle('garapon')"
          >
            <!-- Without the drawing link there is nothing to spin: say why, and
                 how to get there. -->
            <p v-if="!garapon.player" class="form-alert form-alert-info" role="status">
              <font-awesome-icon :icon="['fas', 'circle-info']" class="form-alert-icon" />
              <span v-if="garapon.hasPairedLink">
                <strong>{{ garapon.pairedDrawsLeft }}</strong>
                {{ garapon.pairedDrawsLeft === 1 ? 'draw is' : 'draws are' }} waiting on your
                drawing link, which isn't on this device. Open that link here and choose
                <strong>Open the festival dashboard</strong> to draw from this page.
              </span>
              <span v-else-if="store.card">
                Your stamp card didn't come with garapon draws. Ask a member of staff.
              </span>
              <span v-else-if="rally?.signupOpen">
                Get a stamp card and you'll get
                {{ garapon.defaultDraws }}
                {{ garapon.defaultDraws === 1 ? 'draw' : 'draws' }} with it.
              </span>
              <span v-else>Draws come with a stamp card from staff.</span>
            </p>
            <GaraponPanel :token="store.garaponToken" :show-details="!garapon.player" />
          </FestivalDashSection>

          <!-- The raffles -->
          <FestivalDashSection
            v-if="raffles.length"
            id="raffles"
            title="Raffles"
            icon="ticket"
            :summary="tally.raffles"
            :collapsed="collapsed.has('raffles')"
            :current="current === 'raffles'"
            @toggle="toggle('raffles')"
          >
            <div class="festival-raffle-grid">
              <FestivalRaffleCard
                v-for="row in raffles"
                :key="row.raffle.id"
                :row="row"
                :turnstile-site-key="turnstileSiteKey"
              />
            </div>
          </FestivalDashSection>

          <!-- Ema plaques -->
          <FestivalDashSection
            v-if="data?.ema"
            id="ema"
            :title="data.ema.title || 'Ema Plaques'"
            icon="torii-gate"
            :summary="tally.ema"
            :collapsed="collapsed.has('ema')"
            :current="current === 'ema'"
            @toggle="toggle('ema')"
          >
            <FestivalEmaSection :ema="data.ema" :turnstile-site-key="turnstileSiteKey" />
          </FestivalDashSection>

          <!-- Gpose booths -->
          <FestivalDashSection
            v-if="store.gposeBooths.length"
            id="gpose"
            :title="
              store.gposeBooths.length === 1 ? store.gposeBooths[0]!.booth.title : 'Gpose Booths'
            "
            icon="camera-retro"
            :summary="tally.gpose"
            :collapsed="collapsed.has('gpose')"
            :current="current === 'gpose'"
            @toggle="toggle('gpose')"
          >
            <p class="text-sm text-muted m-0">
              Keep this page open: when it's your turn it rings, vibrates and flashes its tab until
              you dismiss it.
            </p>
            <div class="festival-raffle-grid">
              <GposeBoothPanel
                v-for="b in store.gposeBooths"
                :key="b.booth.id"
                :booth="b.booth"
                :signups="b.signups"
                :visitor-name="store.visitorName"
                :visitor-world="store.visitorWorld"
                :fixed-visitor="!!store.visitorName"
                :turnstile-site-key="turnstileSiteKey"
                :closed="!b.open"
                @signed-up="onGposeSignedUp"
              />
            </div>
          </FestivalDashSection>
        </div>
      </template>
    </div>
  </div>
</template>
