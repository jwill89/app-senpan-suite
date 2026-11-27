<script setup lang="ts">
/**
 * Admin Gpose Booth create/edit form: the booth's title and description, how it takes
 * sign-ups, the types it offers (from the site's list) with their prices and - for a
 * timeslot booth - their lengths, the open times, and the festival and stall it is
 * filed under.
 *
 *   Basic Signup     - a most-sign-ups limit, then an optional waitlist.
 *   Timeslot Signup  - start times every N minutes inside the open times, each type
 *                      taking its own length, with an optional waitlist beside them.
 *
 * Hosted as a Back sub-page of the Gpose Booth manager: it emits `saved` on a
 * successful save and `cancel` to return to the list.
 */
import { computed, onMounted } from 'vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import MarkdownEditor from '@/components/common/MarkdownEditor.vue'
import AdminPanel from '@/components/common/ui/AdminPanel.vue'
import SubPageHeader from '@/components/common/ui/SubPageHeader.vue'
import FormField from '@/components/common/ui/FormField.vue'
import FormRow from '@/components/common/ui/FormRow.vue'
import FormActions from '@/components/common/ui/FormActions.vue'
import EmptyState from '@/components/common/ui/EmptyState.vue'
import { useGposeStore, type GposeMode } from '@/stores/gpose'
import { occupantListLabel, stallCaption } from '@/lib/festivalmap'

const emit = defineEmits<{ saved: []; cancel: [] }>()
const gpose = useGposeStore()

onMounted(() => gpose.loadFormSources())

const MODES: { value: GposeMode; label: string; help: string }[] = [
  {
    value: 'basic',
    label: 'Basic Signup',
    help: 'People sign up in order up to the most you take, then join the waitlist. You work down the list.',
  },
  {
    value: 'timeslot',
    label: 'Timeslot Signup',
    help: 'People book a start time that fits their type inside your open times. The waitlist fills a slot someone misses.',
  },
]

const form = computed(() => gpose.boothForm)
const timeslot = computed(() => form.value?.mode === 'timeslot')
const modeHelp = computed(() => MODES.find((m) => m.value === form.value?.mode)?.help ?? '')

/** The site's types the booth doesn't offer yet, for the Add picker. */
const unused = computed(() =>
  gpose.types.filter((t) => !form.value?.types.some((bt) => bt.typeId === t.id)),
)

function typeName(id: number): string {
  return gpose.types.find((t) => t.id === id)?.name ?? 'Unknown type'
}

/** Adds the picked type, and puts the picker back on its prompt for the next one. */
function addType(select: HTMLSelectElement): void {
  const id = Number(select.value)
  if (form.value && id) {
    form.value.types.push({ typeId: id, price: 0, minutes: 15 })
  }
  select.value = ''
}

function removeType(i: number): void {
  form.value?.types.splice(i, 1)
}

function addWindow(): void {
  form.value?.windows.push({ startAt: '', endAt: '' })
}

function removeWindow(i: number): void {
  form.value?.windows.splice(i, 1)
}

function setFestivalMap(value: string): void {
  void gpose.setFestivalMap(value ? Number(value) : null)
}

function setOccupant(value: string): void {
  if (form.value) {
    form.value.occupantId = value ? Number(value) : null
  }
}

const canSave = computed(
  () =>
    !!form.value?.title.trim() &&
    form.value.types.length > 0 &&
    (!timeslot.value || form.value.windows.length > 0),
)

async function save(): Promise<void> {
  if (await gpose.saveBooth()) {
    emit('saved')
  }
}

function cancel(): void {
  gpose.cancelBoothForm()
  emit('cancel')
}
</script>

<template>
  <AdminPanel>
    <SubPageHeader
      :icon="['fad', 'camera-retro']"
      :title="`${form && form.id ? 'Edit' : 'New'} Gpose Booth`"
      @back="cancel"
    />
    <template v-if="form">
      <FormField label="Title" required>
        <input v-model="form.title" placeholder="Booth title" aria-label="Booth title" />
      </FormField>
      <FormField label="Description" help="Shown to visitors - a good place to say how to pay.">
        <MarkdownEditor
          v-model="form.description"
          min-height="120px"
          placeholder="Description (supports markdown)"
        />
      </FormField>

      <FormField label="Sign-up Type" :help="modeHelp">
        <div class="toggle-group">
          <button
            v-for="m in MODES"
            :key="m.value"
            type="button"
            class="toggle-btn"
            :class="{ 'is-active': form.mode === m.value }"
            @click="form.mode = m.value"
          >
            {{ m.label }}
          </button>
        </div>
      </FormField>

      <FormRow>
        <FormField v-if="!timeslot" label="Most Sign-ups" help="How many people the booth takes.">
          <input v-model.number="form.maxSignups" type="number" min="1" max="1000" />
        </FormField>
        <FormField v-else label="Start Times Every" help="Minutes between the times offered.">
          <input v-model.number="form.slotStep" type="number" min="5" max="240" step="5" />
        </FormField>
        <FormField label="Waitlist" help="The most on the waitlist; 0 for no waitlist.">
          <input v-model.number="form.maxWaitlist" type="number" min="0" max="1000" />
        </FormField>
        <FormField
          label="Sign-ups Per Person"
          help="How many one character may have waiting at once."
        >
          <input v-model.number="form.perPerson" type="number" min="1" max="20" />
        </FormField>
      </FormRow>

      <!-- The types the booth offers. -->
      <FormField
        label="Types Offered"
        :help="
          timeslot
            ? 'Each type with its price (0 for none) and how long one takes.'
            : 'Each type with its price in gil (0 for none).'
        "
      >
        <EmptyState
          v-if="!gpose.types.length && !gpose.typesLoading"
          text="There are no gpose types yet. Add them from Manage Types."
        />
        <div class="stack">
          <div v-for="(t, i) in form.types" :key="t.typeId" class="stack-row">
            <span class="field-label field-label--inline flex-2 min-w-120">{{
              typeName(t.typeId)
            }}</span>
            <input
              v-model.number="t.price"
              type="number"
              min="0"
              step="1"
              class="flex-1 min-w-100"
              :aria-label="`Price of ${typeName(t.typeId)} in gil`"
            />
            <input
              v-if="timeslot"
              v-model.number="t.minutes"
              type="number"
              min="1"
              max="600"
              class="flex-fixed-96"
              :aria-label="`Minutes one ${typeName(t.typeId)} gpose takes`"
            />
            <span v-if="timeslot" class="text-sm text-muted">min</span>
            <button
              type="button"
              class="btn-danger btn-sm"
              :aria-label="`Remove ${typeName(t.typeId)}`"
              title="Remove this type"
              @click="removeType(i)"
            >
              <font-awesome-icon :icon="['fas', 'trash']" />
            </button>
          </div>
        </div>
        <select
          v-if="unused.length"
          class="mt-8"
          value=""
          aria-label="Add a type"
          @change="addType($event.target as HTMLSelectElement)"
        >
          <option value="">+ Add a type...</option>
          <option v-for="t in unused" :key="t.id" :value="t.id">{{ t.name }}</option>
        </select>
      </FormField>

      <!-- When a timeslot booth is open. -->
      <FormField
        v-if="timeslot"
        label="Open Times"
        help="When the booth is open, in your local time. Add more than one to close for part of the festival."
      >
        <div class="stack">
          <div v-for="(w, i) in form.windows" :key="i" class="stack-row">
            <input
              v-model="w.startAt"
              type="datetime-local"
              class="flex-1 min-w-160"
              :aria-label="`Open time ${i + 1} starts`"
            />
            <span class="text-sm text-muted">to</span>
            <input
              v-model="w.endAt"
              type="datetime-local"
              class="flex-1 min-w-160"
              :aria-label="`Open time ${i + 1} ends`"
            />
            <button
              type="button"
              class="btn-danger btn-sm"
              :aria-label="`Remove open time ${i + 1}`"
              title="Remove this open time"
              @click="removeWindow(i)"
            >
              <font-awesome-icon :icon="['fas', 'trash']" />
            </button>
          </div>
        </div>
        <button type="button" class="btn-confirm btn-sm mt-8" @click="addWindow">
          <font-awesome-icon :icon="['fas', 'plus']" /> Add Open Time
        </button>
      </FormField>

      <!-- Festival Map: files the booth under a festival and, optionally, pins it to a
           stall so the public plan links to it. -->
      <FormRow>
        <FormField
          label="Festival Map"
          help="Optional. Shows the booth on that festival's dashboard."
        >
          <select
            :value="form.festivalMapId ?? ''"
            aria-label="Linked festival map"
            @change="setFestivalMap(($event.target as HTMLSelectElement).value)"
          >
            <option value="">Not part of a festival</option>
            <option v-for="m in gpose.festivalMaps" :key="m.id" :value="m.id">{{ m.title }}</option>
          </select>
        </FormField>
        <FormField
          v-if="form.festivalMapId != null"
          label="Stall"
          help="Optional. The stall's panel on the map links to the booth while it is open."
        >
          <select
            :value="form.occupantId ?? ''"
            aria-label="Festival map stall"
            @change="setOccupant(($event.target as HTMLSelectElement).value)"
          >
            <option value="">Not pinned to a stall</option>
            <option v-for="o in gpose.mapStalls" :key="o.id" :value="o.id">
              {{ occupantListLabel(o) }} ({{ stallCaption(o) || 'Other' }})
            </option>
          </select>
        </FormField>
      </FormRow>

      <FormActions align="start">
        <button type="button" class="btn-neutral" :disabled="gpose.saving" @click="cancel">
          Cancel
        </button>
        <button
          type="button"
          class="btn-confirm"
          :disabled="!canSave || gpose.saving"
          @click="save"
        >
          <LoadingSpinner v-if="gpose.saving" label="Saving..." />
          <template v-else>Save Booth</template>
        </button>
      </FormActions>
    </template>
  </AdminPanel>
</template>
