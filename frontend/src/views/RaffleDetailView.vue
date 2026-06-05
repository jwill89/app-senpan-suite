<script setup lang="ts">
/**
 * Public raffle detail: prize image, sanitized markdown description/rules, and
 * either a sign-up form (open raffles) or the winner announcement (closed).
 */
import { computed, ref, watch } from 'vue'
import { apiGet, apiPost } from '@/api/client'
import { renderMarkdown } from '@/lib/markdown'
import { raffleImageSrc } from '@/lib/assets'
import type {
  Raffle,
  RaffleDetailResponse,
  RaffleEnterResponse,
  RaffleEntry,
} from '@/types/models'

const props = defineProps<{ id: string }>()

const raffle = ref<Raffle | null>(null)
const totalEntries = ref(0)
const winnerEntry = ref<RaffleEntry | null>(null)
const loading = ref(true)
const error = ref('')

const form = ref({ characterName: '', world: '', numEntries: 1 })
const result = ref<RaffleEnterResponse | null>(null)
const submitting = ref(false)
const formError = ref('')

const isOpen = computed(() => raffle.value?.status === 'open')
const descriptionHtml = computed(() => renderMarkdown(raffle.value?.description))
const rulesHtml = computed(() => renderMarkdown(raffle.value?.rules))
const instructionsHtml = computed(() => renderMarkdown(result.value?.signup_instructions))
const totalCost = computed(
  () => Math.max(1, form.value.numEntries) * (raffle.value?.cost_per_entry ?? 0),
)

async function load() {
  loading.value = true
  error.value = ''
  result.value = null
  try {
    const data = await apiGet<RaffleDetailResponse>(`raffles/${encodeURIComponent(props.id)}`)
    raffle.value = data.raffle
    totalEntries.value = data.total_entries ?? 0
    winnerEntry.value = data.winner_entry ?? null
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Raffle not found'
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (!raffle.value) return
  formError.value = ''
  const characterName = form.value.characterName.trim()
  const world = form.value.world.trim()
  if (!characterName || !world) {
    formError.value = 'Character name and world are required.'
    return
  }
  submitting.value = true
  try {
    result.value = await apiPost<RaffleEnterResponse>(`raffles/${raffle.value.id}/enter`, {
      character_name: characterName,
      world,
      num_entries: Math.max(1, form.value.numEntries),
    })
  } catch (e) {
    formError.value = e instanceof Error ? e.message : 'Sign-up failed'
  } finally {
    submitting.value = false
  }
}

watch(() => props.id, load, { immediate: true })
</script>

<template>
  <div class="detail">
    <RouterLink class="back" :to="{ name: 'raffles' }">← All raffles</RouterLink>

    <p v-if="loading" class="muted">Loading…</p>
    <p v-else-if="error" class="error">{{ error }}</p>

    <template v-else-if="raffle">
      <img
        v-if="raffle.prize_image"
        :src="raffleImageSrc(raffle.prize_image)"
        alt=""
        class="prize"
        @error="($event.target as HTMLImageElement).style.display = 'none'"
      />

      <h1>{{ raffle.title }}</h1>

      <!-- description/rules are admin-authored markdown, sanitized before render -->
      <div v-if="descriptionHtml" class="prose" v-html="descriptionHtml" />
      <div v-if="rulesHtml" class="prose rules">
        <h3>Rules</h3>
        <div v-html="rulesHtml" />
      </div>

      <!-- Closed raffle: announce the winner -->
      <section v-if="!isOpen" class="panel">
        <h2>Raffle closed</h2>
        <p v-if="winnerEntry">
          🎉 Winner: <strong>{{ winnerEntry.character_name }}</strong>
          <span class="muted"> ({{ winnerEntry.world }})</span>
        </p>
        <p v-else class="muted">This raffle is closed.</p>
        <p class="muted small">{{ totalEntries }} total entries</p>
      </section>

      <!-- Confirmation after a successful sign-up -->
      <section v-else-if="result" class="panel">
        <h2>You're in!</h2>
        <p>{{ result.message }}</p>
        <p>
          Entries: <strong>{{ result.total_entries }}</strong>
          <template v-if="result.total_cost > 0">
            · Total cost: <strong>{{ result.total_cost }}</strong>
          </template>
        </p>
        <div v-if="instructionsHtml" class="prose" v-html="instructionsHtml" />
      </section>

      <!-- Open raffle: sign-up form -->
      <section v-else class="panel">
        <h2>Enter this raffle</h2>
        <form class="form" @submit.prevent="submit">
          <label>
            Character name
            <input v-model="form.characterName" autocomplete="off" required />
          </label>
          <label>
            World
            <input v-model="form.world" autocomplete="off" required />
          </label>
          <label>
            Number of entries
            <input
              v-model.number="form.numEntries"
              type="number"
              min="1"
              :max="raffle.max_entries"
            />
          </label>
          <p class="muted small">
            Max {{ raffle.max_entries }} per person<template v-if="raffle.cost_per_entry > 0">
              · Total cost: <strong>{{ totalCost }}</strong></template
            >
          </p>
          <p v-if="formError" class="error">{{ formError }}</p>
          <button type="submit" :disabled="submitting">
            {{ submitting ? 'Signing up…' : 'Sign up' }}
          </button>
        </form>
      </section>
    </template>
  </div>
</template>

<style scoped>
.back {
  display: inline-block;
  margin-bottom: 16px;
  color: #9a9ab0;
  text-decoration: none;
  font-size: 0.9rem;
}
.back:hover {
  color: #fff;
}
.prize {
  width: 100%;
  max-height: 320px;
  object-fit: cover;
  border-radius: 14px;
  background: #14141d;
}
h1 {
  margin: 16px 0 8px;
}
.prose {
  color: #d4d4e0;
  line-height: 1.55;
}
.prose.rules {
  margin-top: 8px;
}
.prose :deep(h3) {
  margin-bottom: 4px;
}
.panel {
  margin-top: 24px;
  border: 1px solid #26263a;
  border-radius: 14px;
  padding: 20px;
  background: #1c1c28;
}
.panel h2 {
  margin-top: 0;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.form label {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.form input {
  width: 100%;
}
.muted {
  color: #9a9ab0;
}
.small {
  font-size: 0.85rem;
}
.error {
  color: #ff6b6b;
}
</style>
