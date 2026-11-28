<script setup lang="ts">
/**
 * One gpose booth as a visitor sees it: what it offers and for how much, where they
 * stand in it, and a sign-up form. The booth's own page and the festival dashboard
 * both render it.
 *
 * A basic booth puts them next in line, or on its waitlist once its places are taken;
 * a timeslot booth offers the start times the server says each type fits, and the
 * waitlist beside them. On the dashboard the visitor is already known, so the form
 * doesn't ask who they are.
 *
 * Once a shoot is done, staff may upload its gpose: it shows under that sign-up to view
 * and download, for a day. The dashboard keeps a closed booth while one lasts, with no
 * form.
 */
import { computed, ref, useTemplateRef, watch } from 'vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import MarkdownText from '@/components/common/MarkdownText.vue'
import TurnstileWidget from '@/components/common/TurnstileWidget.vue'
import WorldPicker from '@/components/common/ui/WorldPicker.vue'
import type { PublicGposeBooth, PublicGposeSignup } from '@/api/client'
import { formatGposeTime, gposePriceLabel, gposeStanding, useGposeStore } from '@/stores/gpose'

const props = withDefaults(
  defineProps<{
    booth: PublicGposeBooth
    signups: readonly PublicGposeSignup[]
    /** Who the visitor is, when the page knows: it fills the form in. */
    visitorName?: string
    visitorWorld?: string
    /** The visitor can't be changed here (the dashboard says who they are). */
    fixedVisitor?: boolean
    /** Cloudflare Turnstile site key ('' = the check is off). */
    turnstileSiteKey: string
    /** The booth takes no sign-ups: the dashboard shows it for a gpose still there. */
    closed?: boolean
  }>(),
  { visitorName: '', visitorWorld: '', fixedVisitor: false, closed: false },
)

const emit = defineEmits<{
  'signed-up': [signup: PublicGposeSignup, name: string, world: string]
}>()

const store = useGposeStore()

const STATUS_BADGE: Record<string, string> = {
  waiting: 'badge--accent',
  completed: 'badge--success',
  no_show: 'badge--danger',
  canceled: 'badge--muted',
}

const timeslot = computed(() => props.booth.mode === 'timeslot')
const knownVisitor = computed(
  () => props.fixedVisitor && !!props.visitorName && !!props.visitorWorld,
)
const waiting = computed(() => props.signups.filter((s) => s.status === 'waiting').length)
const atLimit = computed(() => waiting.value >= props.booth.perPerson)

const name = ref(props.visitorName)
const world = ref(props.visitorWorld)
const typeId = ref(props.booth.types[0]?.typeId ?? 0)
const slotStart = ref('')
const joinWaitlist = ref(false)
const turnstileToken = ref('')
const turnstile = useTemplateRef<InstanceType<typeof TurnstileWidget>>('turnstile')

const chosen = computed(() => props.booth.types.find((t) => t.typeId === typeId.value))
/** The times the chosen type can start, as the server offered them. */
const starts = computed(() => chosen.value?.starts ?? [])
const hasWaitlist = computed(() => props.booth.maxWaitlist > 0 && props.booth.waitlistLeft > 0)

// The first free time is picked to start with; a time picked for one type may not
// fit another, and a list re-read may drop it.
watch(
  [typeId, starts],
  () => {
    if (!starts.value.includes(slotStart.value)) {
      slotStart.value = starts.value[0] ?? ''
    }
  },
  { immediate: true },
)
watch(
  () => [props.visitorName, props.visitorWorld] as const,
  ([n, w]) => {
    if (props.fixedVisitor || !name.value) {
      name.value = n
    }
    if (props.fixedVisitor || !world.value) {
      world.value = w
    }
  },
)

/** Whether there is anywhere for a new sign-up to go. */
const open = computed(() => {
  if (timeslot.value) {
    return props.booth.types.some((t) => t.starts.length > 0) || hasWaitlist.value
  }
  return props.booth.placesLeft > 0 || hasWaitlist.value
})

const ready = computed(
  () =>
    !!chosen.value &&
    !!name.value.trim() &&
    !!world.value.trim() &&
    (!props.turnstileSiteKey || !!turnstileToken.value) &&
    (!timeslot.value || joinWaitlist.value || !!slotStart.value),
)

function typeLabel(t: { name: string; price: number; minutes: number }): string {
  const parts = [t.name]
  if (t.minutes) {
    parts.push(`${t.minutes} min`)
  }
  const price = gposePriceLabel(t.price)
  if (price) {
    parts.push(price)
  }
  return parts.join(' - ')
}

async function submit(): Promise<void> {
  const n = name.value.trim()
  const w = world.value.trim()
  const made = await store.signUp(props.booth, {
    characterName: n,
    world: w,
    typeId: typeId.value,
    slotStart: timeslot.value && !joinWaitlist.value ? slotStart.value : undefined,
    waitlist: timeslot.value && joinWaitlist.value,
    turnstileToken: turnstileToken.value,
  })
  // The token is single-use, either way.
  turnstileToken.value = ''
  turnstile.value?.reset()
  if (made) {
    joinWaitlist.value = false
    emit('signed-up', made, n, w)
  }
}
</script>

<template>
  <article :id="`gpose-booth-${booth.id}`" class="gpose-booth-panel card card--padded">
    <h4 class="gpose-booth-panel-title">
      <font-awesome-icon :icon="['fad', 'camera-retro']" /> {{ booth.title }}
    </h4>
    <p v-if="booth.stallName" class="text-sm text-muted m-0">
      <font-awesome-icon :icon="['fad', 'store']" /> {{ booth.stallName }}
    </p>
    <MarkdownText v-if="booth.description" class="game-details" :source="booth.description" />

    <ul class="gpose-booth-panel-types">
      <li v-for="t in booth.types" :key="t.typeId">{{ typeLabel(t) }}</li>
    </ul>
    <p v-if="!closed" class="text-sm text-muted m-0">
      <template v-if="!timeslot">
        {{ booth.placesLeft }} of {{ booth.maxSignups }}
        {{ booth.maxSignups === 1 ? 'place' : 'places' }} left
      </template>
      <template v-else>Book a time below</template>
      <template v-if="booth.maxWaitlist > 0">
        - {{ booth.waitlistLeft }} left on the waitlist
      </template>
    </p>

    <!-- Where the visitor stands. -->
    <section v-if="signups.length" aria-label="Your sign-ups">
      <h5 class="section-heading">Your sign-ups</h5>
      <ul class="list-rows">
        <li v-for="s in signups" :key="s.id" class="gpose-booth-panel-mine">
          <span>
            <strong>{{ s.typeName }}</strong>
            <template v-if="s.price"> - {{ gposePriceLabel(s.price) }}</template>
          </span>
          <span :class="['badge', STATUS_BADGE[s.status]]">{{ gposeStanding(s) }}</span>
          <span v-if="s.price && s.paid" class="badge badge--success">Paid</span>
          <span v-if="s.status === 'waiting' && s.calledAt" class="badge badge--warning">
            <font-awesome-icon :icon="['fas', 'bell']" /> Called up
          </span>
          <figure v-if="s.photoUrl" class="captioned-figure gpose-booth-panel-photo">
            <figcaption>Your gpose is ready</figcaption>
            <a :href="s.photoUrl" target="_blank" rel="noopener">
              <img :src="s.photoUrl" :alt="`Your ${s.typeName} gpose from ${booth.title}`" />
            </a>
            <p class="text-sm text-muted m-0">
              Save it by {{ formatGposeTime(s.photoExpiresAt) }}: it's deleted after that.
            </p>
            <div class="form-actions">
              <a class="btn-action btn-sm" :href="s.photoUrl" download>
                <font-awesome-icon :icon="['fas', 'download']" /> Download
              </a>
            </div>
          </figure>
        </li>
      </ul>
    </section>

    <!-- Signing up. -->
    <p v-if="closed" class="text-sm text-muted m-0">This booth is closed.</p>
    <p v-else-if="atLimit" class="text-sm text-muted m-0">
      You have {{ waiting === 1 ? 'a sign-up' : `${waiting} sign-ups` }} waiting here, the most this
      booth allows at once.
    </p>
    <p v-else-if="!open" class="text-sm text-muted m-0">This booth is full right now.</p>
    <form v-else class="festival-dash-form" @submit.prevent="submit">
      <p v-if="knownVisitor" class="text-sm m-0">
        Signing up as <strong>{{ visitorName }} @ {{ visitorWorld }}</strong>
      </p>
      <template v-else>
        <input
          v-model="name"
          placeholder="Character name"
          aria-label="Character name"
          autocomplete="off"
        />
        <WorldPicker v-model="world" />
      </template>
      <label class="field">
        <span class="field-label">Type of gpose</span>
        <select v-model.number="typeId">
          <option v-for="t in booth.types" :key="t.typeId" :value="t.typeId">
            {{ typeLabel(t) }}
          </option>
        </select>
      </label>
      <template v-if="timeslot">
        <label v-if="!joinWaitlist" class="field">
          <span class="field-label">Time</span>
          <select v-model="slotStart" :disabled="!starts.length">
            <option v-if="!starts.length" value="">No times left for this type</option>
            <option v-for="at in starts" :key="at" :value="at">{{ formatGposeTime(at) }}</option>
          </select>
        </label>
        <label v-if="hasWaitlist" class="gpose-booth-panel-check">
          <input v-model="joinWaitlist" type="checkbox" />
          Join the waitlist instead, for a slot someone misses
        </label>
      </template>
      <p v-else-if="booth.placesLeft === 0" class="text-sm m-0">
        The places are taken: you'll join the waitlist.
      </p>
      <TurnstileWidget
        v-if="turnstileSiteKey"
        ref="turnstile"
        :site-key="turnstileSiteKey"
        @verified="turnstileToken = $event"
        @expired="turnstileToken = ''"
        @error="turnstileToken = ''"
      />
      <div class="form-actions">
        <button type="submit" class="btn-confirm" :disabled="!ready || store.signingUp">
          <LoadingSpinner v-if="store.signingUp" label="Signing up..." />
          <template v-else>Sign up</template>
        </button>
      </div>
    </form>
  </article>
</template>
