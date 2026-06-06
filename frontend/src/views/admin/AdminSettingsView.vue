<script setup lang="ts">
/** App settings editor (system tab). Mirrors POST /api/settings validation. */
import { onMounted, ref } from 'vue'
import { apiGet, apiPost } from '@/api/client'
import { useToast } from '@/composables/useToast'
import type { AppSettings, SettingsResponse } from '@/types/models'

const { show } = useToast()

const settings = ref<AppSettings>({
  app_title: '',
  default_draw_delay: '0',
  frequent_winner_threshold: '3',
  frequent_winner_hours: '12',
  header_font: '',
  google_fonts_api_key: '',
})
const loading = ref(true)
const saving = ref(false)

onMounted(async () => {
  try {
    const data = await apiGet<SettingsResponse>('settings')
    settings.value = { ...settings.value, ...data.settings }
  } catch (e) {
    show(e instanceof Error ? e.message : 'Failed to load settings', 'error')
  } finally {
    loading.value = false
  }
})

/** Client-side guard matching the server's numeric ranges. */
function validate(): string | null {
  const n = (v: string) => Number.parseInt(v, 10)
  const delay = n(settings.value.default_draw_delay)
  if (Number.isNaN(delay) || delay < 0 || delay > 60) return 'Draw delay must be 0–60.'
  const thr = n(settings.value.frequent_winner_threshold)
  if (Number.isNaN(thr) || thr < 1 || thr > 100) return 'Winner threshold must be 1–100.'
  const hrs = n(settings.value.frequent_winner_hours)
  if (Number.isNaN(hrs) || hrs < 1 || hrs > 168) return 'Winner hours must be 1–168.'
  return null
}

async function save() {
  const err = validate()
  if (err) {
    show(err, 'error')
    return
  }
  saving.value = true
  try {
    await apiPost('settings', { settings: settings.value })
    document.title = settings.value.app_title || 'Bingo'
    show('Settings saved', 'success')
  } catch (e) {
    show(e instanceof Error ? e.message : 'Save failed', 'error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section>
    <h2>Settings</h2>
    <p v-if="loading" class="muted">Loading…</p>
    <form v-else class="form" @submit.prevent="save">
      <label>
        App title
        <input v-model="settings.app_title" />
      </label>
      <label>
        Default draw delay (seconds, 0–60)
        <input v-model="settings.default_draw_delay" type="number" min="0" max="60" />
      </label>
      <label>
        Frequent-winner threshold (wins, 1–100)
        <input v-model="settings.frequent_winner_threshold" type="number" min="1" max="100" />
      </label>
      <label>
        Frequent-winner window (hours, 1–168)
        <input v-model="settings.frequent_winner_hours" type="number" min="1" max="168" />
      </label>
      <label>
        Header font (Google Font family)
        <input v-model="settings.header_font" placeholder="Arapey" />
      </label>
      <label>
        Google Fonts API key (optional)
        <input v-model="settings.google_fonts_api_key" autocomplete="off" />
      </label>
      <div>
        <button type="submit" :disabled="saving">{{ saving ? 'Saving…' : 'Save settings' }}</button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 460px;
}
label {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
input {
  width: 100%;
}
.muted {
  color: #9a9ab0;
}
</style>
