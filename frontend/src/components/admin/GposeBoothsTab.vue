<script setup lang="ts">
/**
 * Admin Gpose Booth manager, in the standard manager model:
 *
 *   - list: the open booths, then the closed ones, each with its counts.
 *   - detail: one booth's sign-ups. A basic booth lists who is in line in the order
 *     they signed up, with Call (alerts the person on their dashboard and booth page),
 *     Complete, No-Show and Cancel; a timeslot booth lists its bookings by time with
 *     Complete, No-Show and Cancel, and the waitlist with the times each one's shoot
 *     fits now - a no-show's freed slot among them. Finished ones can be undone, and a
 *     completed one takes its gpose: uploaded here, the visitor views and downloads it
 *     on their festival dashboard and the booth page for a day, then it is deleted.
 *   - form: the create/edit form (GposeBoothFormTab).
 *   - types: the site's list of gpose types (Solo, Couple, Group...).
 *
 * All state and actions come from the gpose store.
 */
import { computed, reactive, ref, useTemplateRef } from 'vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AdminPanel from '@/components/common/ui/AdminPanel.vue'
import ManagerView from '@/components/common/ui/ManagerView.vue'
import SubPageHeader from '@/components/common/ui/SubPageHeader.vue'
import ListRow from '@/components/common/ui/ListRow.vue'
import DataTable, { type DataColumn } from '@/components/common/ui/DataTable.vue'
import EmptyState from '@/components/common/ui/EmptyState.vue'
import GposeBoothFormTab from './GposeBoothFormTab.vue'
import { formatGposeSlot, formatGposeTime, gposePriceLabel, useGposeStore } from '@/stores/gpose'
import { formatServerTimestamp } from '@/lib/datetime'
import type { GposeBooth, GposeSignup } from '@/api/client'

const gpose = useGposeStore()

type Screen = 'list' | 'detail' | 'form' | 'types'
const screen = ref<Screen>('list')

const booth = computed(() => gpose.selectedBooth)
const timeslot = computed(() => booth.value?.mode === 'timeslot')
const isOpen = computed(() => booth.value?.status === 'open')

const STATUS: Record<string, { label: string; badge: string }> = {
  waiting: { label: 'Waiting', badge: 'badge--accent' },
  completed: { label: 'Completed', badge: 'badge--success' },
  no_show: { label: 'No-show', badge: 'badge--danger' },
  canceled: { label: 'Canceled', badge: 'badge--muted' },
}

function modeLabel(b: GposeBooth): string {
  return b.mode === 'timeslot' ? 'Timeslot' : 'Basic'
}

function countsLabel(b: GposeBooth): string {
  const parts =
    b.mode === 'basic'
      ? [`${b.bookedCount} of ${b.maxSignups} places taken`]
      : [`${b.bookedCount} booked`]
  if (b.maxWaitlist > 0) {
    parts.push(`${b.waitlistCount} of ${b.maxWaitlist} on the waitlist`)
  }
  return parts.join(' - ')
}

// -- The sign-up tables ------------------------------------------------------------

/** In line (basic) or booked (timeslot): waiting and holding a place or slot. */
const booked = computed(() => {
  const rows = gpose.signups.filter((s) => s.status === 'waiting' && !s.waitlisted)
  return timeslot.value
    ? [...rows].sort((a, b) => (a.slotStart ?? '').localeCompare(b.slotStart ?? ''))
    : rows
})
const waitlist = computed(() => gpose.signups.filter((s) => s.status === 'waiting' && s.waitlisted))
const finished = computed(() => gpose.signups.filter((s) => s.status !== 'waiting'))
/** The open booth with its counts taken from the sign-up list, which stays live. */
const liveCounts = computed<GposeBooth | null>(() =>
  booth.value
    ? {
        ...booth.value,
        bookedCount: gpose.signups.filter(
          (s) => !s.waitlisted && (s.status === 'waiting' || s.status === 'completed'),
        ).length,
        waitlistCount: waitlist.value.length,
      }
    : null,
)

const bookedColumns = computed<DataColumn[]>(() => [
  timeslot.value ? { key: 'time', label: 'Time' } : { key: 'place', label: '#', align: 'center' },
  { key: 'character', label: 'Character' },
  { key: 'type', label: 'Type' },
  { key: 'created', label: 'Signed up' },
  { key: 'paid', label: 'Paid', align: 'center' },
  { key: 'actions', label: '', align: 'right' },
])
const waitlistColumns: DataColumn[] = [
  { key: 'place', label: '#', align: 'center' },
  { key: 'character', label: 'Character' },
  { key: 'type', label: 'Type' },
  { key: 'created', label: 'Signed up' },
  { key: 'actions', label: '', align: 'right' },
]
const finishedColumns = computed<DataColumn[]>(() => [
  { key: 'status', label: 'Status' },
  { key: 'character', label: 'Character' },
  { key: 'type', label: 'Type' },
  timeslot.value ? { key: 'time', label: 'Time' } : { key: 'created', label: 'Signed up' },
  { key: 'paid', label: 'Paid', align: 'center' },
  { key: 'photo', label: 'Gpose' },
  { key: 'actions', label: '', align: 'right' },
])

function typeLabel(s: GposeSignup): string {
  const price = gposePriceLabel(s.price)
  return price ? `${s.typeName} (${price})` : s.typeName
}

/** The start time picked for each waitlisted sign-up, by its id. */
const assignTo = reactive(new Map<number, string>())
function assignChoice(s: GposeSignup): string {
  const picked = assignTo.get(s.id)
  return picked && s.assignableStarts.includes(picked) ? picked : (s.assignableStarts[0] ?? '')
}
async function assign(s: GposeSignup): Promise<void> {
  const at = assignChoice(s)
  if (at && (await gpose.changeSignup(s, { slotStart: at }))) {
    assignTo.delete(s.id)
  }
}

function setStatus(s: GposeSignup, status: GposeSignup['status']): void {
  void gpose.changeSignup(s, { status })
}

function togglePaid(s: GposeSignup): void {
  void gpose.changeSignup(s, { paid: !s.paid })
}

// -- Gposes ------------------------------------------------------------------------

/** The pictures a gpose is taken in, as the server judges them. */
const PHOTO_ACCEPT = 'image/png,image/jpeg,image/webp'

/** Hidden <input type="file"> behind every Upload and Replace button, and the sign-up
 *  the pick is for: a bare file input can't be themed (see BookClubTab). */
const photoInput = useTemplateRef<HTMLInputElement>('photoInput')
const photoFor = ref<GposeSignup | null>(null)

function pickPhoto(s: GposeSignup): void {
  photoFor.value = s
  photoInput.value?.click()
}

async function onPhotoPicked(e: Event): Promise<void> {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  const s = photoFor.value
  input.value = ''
  photoFor.value = null
  if (file && s) {
    await gpose.uploadPhoto(s, file)
  }
}

// -- Navigation --------------------------------------------------------------------

function openNew(): void {
  gpose.newBoothForm()
  screen.value = 'form'
}
function openBooth(b: GposeBooth): void {
  void gpose.viewBooth(b)
  screen.value = 'detail'
}
function editSelected(): void {
  if (booth.value) {
    gpose.editBoothForm(booth.value)
    screen.value = 'form'
  }
}
function openTypes(): void {
  void gpose.loadTypes()
  screen.value = 'types'
}
function backToList(): void {
  gpose.selectedBooth = null
  screen.value = 'list'
}
function onFormDone(): void {
  screen.value = booth.value ? 'detail' : 'list'
  if (booth.value) {
    void gpose.loadSignups(booth.value.id)
  }
}
async function deleteSelected(): Promise<void> {
  if (booth.value && (await gpose.deleteBooth(booth.value.id))) {
    screen.value = 'list'
  }
}

// -- Types -------------------------------------------------------------------------

const newTypeName = ref('')
/** The type being renamed, and the name typed so far. */
const renamingId = ref<number | null>(null)
const renameTo = ref('')

async function addType(): Promise<void> {
  if (await gpose.createType(newTypeName.value)) {
    newTypeName.value = ''
  }
}
function startRename(id: number, name: string): void {
  renamingId.value = id
  renameTo.value = name
}
function cancelRename(): void {
  renamingId.value = null
}
async function saveRename(): Promise<void> {
  if (renamingId.value !== null && (await gpose.renameType(renamingId.value, renameTo.value))) {
    renamingId.value = null
  }
}
</script>

<template>
  <div class="tab-body">
    <!-- -- Form -------------------------------------------------------------------- -->
    <GposeBoothFormTab v-if="screen === 'form'" @saved="onFormDone" @cancel="onFormDone" />

    <!-- -- Types ------------------------------------------------------------------- -->
    <AdminPanel v-else-if="screen === 'types'">
      <SubPageHeader :icon="['fad', 'camera-retro']" title="Gpose Types" @back="screen = 'list'" />
      <p class="text-sm text-muted">
        The kinds of gpose every booth can offer. Each booth sets its own price and length for the
        ones it offers.
      </p>
      <form class="flex-toolbar mb-16" @submit.prevent="addType">
        <input
          v-model="newTypeName"
          class="flex-1 min-w-160"
          placeholder="New type, such as Solo or Couple"
          aria-label="New gpose type"
          maxlength="60"
        />
        <button type="submit" class="btn-confirm btn-sm" :disabled="!newTypeName.trim()">
          <font-awesome-icon :icon="['fas', 'plus']" /> Add Type
        </button>
      </form>
      <LoadingSpinner v-if="gpose.typesLoading && !gpose.types.length" block label="Loading..." />
      <div v-else-if="gpose.types.length" class="list-rows">
        <ListRow v-for="t in gpose.types" :key="t.id">
          <input
            v-if="renamingId === t.id"
            v-model="renameTo"
            :aria-label="`New name for ${t.name}`"
            maxlength="60"
            @keyup.enter="saveRename"
          />
          <template v-else>
            <strong>{{ t.name }}</strong>
            <span class="text-sm text-muted">
              - offered by {{ t.boothCount }} {{ t.boothCount === 1 ? 'booth' : 'booths' }}
            </span>
          </template>
          <template #actions>
            <template v-if="renamingId === t.id">
              <button type="button" class="btn-confirm btn-sm" @click="saveRename">Save</button>
              <button type="button" class="btn-neutral btn-sm" @click="cancelRename">Cancel</button>
            </template>
            <template v-else>
              <button
                type="button"
                class="btn-confirm btn-sm"
                :aria-label="`Rename ${t.name}`"
                title="Rename"
                @click="startRename(t.id, t.name)"
              >
                <font-awesome-icon :icon="['fas', 'pen-to-square']" />
              </button>
              <button
                type="button"
                class="btn-danger btn-sm"
                :aria-label="`Delete ${t.name}`"
                :title="t.boothCount ? 'Take it off every booth first' : 'Delete'"
                :disabled="t.boothCount > 0"
                @click="gpose.deleteType(t)"
              >
                <font-awesome-icon :icon="['fas', 'trash']" />
              </button>
            </template>
          </template>
        </ListRow>
      </div>
      <EmptyState v-else text="No gpose types yet." />
    </AdminPanel>

    <!-- -- Detail ------------------------------------------------------------------ -->
    <AdminPanel v-else-if="screen === 'detail' && booth">
      <SubPageHeader @back="backToList">
        {{ booth.title }}
        <span :class="['status-badge', 'status-badge-' + booth.status]">{{ booth.status }}</span>
        <span class="badge badge--accent">{{ modeLabel(booth) }}</span>
      </SubPageHeader>
      <div class="flex-toolbar flex-end mb-16">
        <button type="button" class="btn-confirm btn-sm" @click="editSelected">
          <font-awesome-icon :icon="['fas', 'pen-to-square']" /> Edit
        </button>
        <button type="button" class="btn-caution btn-sm" @click="gpose.setBoothClosed(isOpen)">
          <font-awesome-icon :icon="['fas', isOpen ? 'lock' : 'rotate']" />
          {{ isOpen ? 'Close' : 'Reopen' }}
        </button>
        <button type="button" class="btn-danger btn-sm" @click="deleteSelected">
          <font-awesome-icon :icon="['fas', 'trash']" /> Delete
        </button>
      </div>
      <p class="text-sm text-muted">
        {{ countsLabel(liveCounts ?? booth) }}
        <template v-if="booth.festivalMapName"> - {{ booth.festivalMapName }}</template>
        <template v-if="booth.stallName"> ({{ booth.stallName }})</template>
      </p>

      <LoadingSpinner v-if="gpose.signupsLoading" block label="Loading sign-ups..." />
      <template v-else>
        <h4 class="section-heading">
          <font-awesome-icon :icon="['fad', 'camera-retro']" />
          {{ timeslot ? 'Booked' : 'In Line' }} ({{ booked.length }})
        </h4>
        <DataTable v-if="booked.length" :columns="bookedColumns" :rows="booked" row-key="id">
          <template #cell-place="{ row }">{{ row.position }}</template>
          <template #cell-time="{ row }">{{
            formatGposeSlot(row.slotStart, row.slotEnd)
          }}</template>
          <template #cell-character="{ row }">
            {{ row.characterName }} @ {{ row.world }}
            <span v-if="row.calledAt" class="badge badge--warning">
              <font-awesome-icon :icon="['fas', 'bell']" /> Called
            </span>
          </template>
          <template #cell-type="{ row }">{{ typeLabel(row) }}</template>
          <template #cell-created="{ row }">
            <span class="text-sm">{{ formatServerTimestamp(row.createdAt) }}</span>
          </template>
          <template #cell-paid="{ row }">
            <button
              v-if="row.price"
              type="button"
              :class="['btn-sm', row.paid ? 'btn-confirm' : 'btn-neutral']"
              :disabled="gpose.changing !== null"
              @click="togglePaid(row)"
            >
              {{ row.paid ? 'Paid' : 'Unpaid' }}
            </button>
            <span v-else class="text-muted">-</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="row-actions">
              <button
                v-if="!timeslot"
                type="button"
                class="btn-action btn-sm"
                :disabled="gpose.changing !== null"
                title="Alert them on their festival dashboard and booth page"
                @click="gpose.callSignup(row)"
              >
                <font-awesome-icon :icon="['fas', 'bell']" /> Call
              </button>
              <button
                type="button"
                class="btn-confirm btn-sm"
                :disabled="gpose.changing !== null"
                @click="setStatus(row, 'completed')"
              >
                <font-awesome-icon :icon="['fas', 'circle-check']" /> Complete
              </button>
              <button
                type="button"
                class="btn-caution btn-sm"
                :disabled="gpose.changing !== null"
                @click="setStatus(row, 'no_show')"
              >
                <font-awesome-icon :icon="['fas', 'user-xmark']" /> No-Show
              </button>
              <button
                type="button"
                class="btn-danger btn-sm"
                :disabled="gpose.changing !== null"
                :aria-label="`Cancel ${row.characterName}'s sign-up`"
                title="Cancel"
                @click="setStatus(row, 'canceled')"
              >
                <font-awesome-icon :icon="['fas', 'ban']" />
              </button>
            </div>
          </template>
        </DataTable>
        <EmptyState v-else :text="timeslot ? 'No bookings waiting.' : 'Nobody in line.'" />

        <template v-if="booth.maxWaitlist > 0 || waitlist.length">
          <h4 class="section-heading mt-20">
            <font-awesome-icon :icon="['fad', 'clipboard-list']" /> Waitlist ({{ waitlist.length }})
          </h4>
          <DataTable
            v-if="waitlist.length"
            :columns="waitlistColumns"
            :rows="waitlist"
            row-key="id"
          >
            <template #cell-place="{ row }">{{ row.position }}</template>
            <template #cell-character="{ row }">
              {{ row.characterName }} @ {{ row.world }}
              <span v-if="row.calledAt" class="badge badge--warning">
                <font-awesome-icon :icon="['fas', 'bell']" /> Called
              </span>
            </template>
            <template #cell-type="{ row }">
              {{ typeLabel(row) }}
              <span v-if="row.minutes" class="text-sm text-muted">{{ row.minutes }} min</span>
            </template>
            <template #cell-created="{ row }">
              <span class="text-sm">{{ formatServerTimestamp(row.createdAt) }}</span>
            </template>
            <template #cell-actions="{ row }">
              <div class="row-actions">
                <template v-if="timeslot">
                  <select
                    v-if="row.assignableStarts.length"
                    :value="assignChoice(row)"
                    :aria-label="`Slot for ${row.characterName}`"
                    @change="assignTo.set(row.id, ($event.target as HTMLSelectElement).value)"
                  >
                    <option v-for="at in row.assignableStarts" :key="at" :value="at">
                      {{ formatGposeTime(at) }}
                    </option>
                  </select>
                  <span v-else class="text-sm text-muted">No slot fits yet</span>
                  <button
                    type="button"
                    class="btn-action btn-sm"
                    :disabled="gpose.changing !== null || !row.assignableStarts.length"
                    @click="assign(row)"
                  >
                    <font-awesome-icon :icon="['fas', 'calendar-check']" /> Assign
                  </button>
                </template>
                <button
                  v-else
                  type="button"
                  class="btn-action btn-sm"
                  :disabled="gpose.changing !== null"
                  @click="gpose.callSignup(row)"
                >
                  <font-awesome-icon :icon="['fas', 'bell']" /> Call
                </button>
                <button
                  type="button"
                  class="btn-danger btn-sm"
                  :disabled="gpose.changing !== null"
                  :aria-label="`Cancel ${row.characterName}'s sign-up`"
                  title="Cancel"
                  @click="setStatus(row, 'canceled')"
                >
                  <font-awesome-icon :icon="['fas', 'ban']" />
                </button>
              </div>
            </template>
          </DataTable>
          <EmptyState v-else text="Nobody on the waitlist." />
        </template>

        <template v-if="finished.length">
          <h4 class="section-heading mt-20">
            <font-awesome-icon :icon="['fad', 'circle-check']" /> Finished ({{ finished.length }})
          </h4>
          <DataTable :columns="finishedColumns" :rows="finished" row-key="id">
            <template #cell-status="{ row }">
              <span :class="['badge', STATUS[row.status]?.badge]">{{
                STATUS[row.status]?.label
              }}</span>
            </template>
            <template #cell-character="{ row }">{{ row.characterName }} @ {{ row.world }}</template>
            <template #cell-type="{ row }">{{ typeLabel(row) }}</template>
            <template #cell-time="{ row }">
              {{ row.slotStart ? formatGposeSlot(row.slotStart, row.slotEnd) : 'Waitlist' }}
            </template>
            <template #cell-created="{ row }">
              <span class="text-sm">{{ formatServerTimestamp(row.createdAt) }}</span>
            </template>
            <template #cell-paid="{ row }">
              <button
                v-if="row.price"
                type="button"
                :class="['btn-sm', row.paid ? 'btn-confirm' : 'btn-neutral']"
                :disabled="gpose.changing !== null"
                @click="togglePaid(row)"
              >
                {{ row.paid ? 'Paid' : 'Unpaid' }}
              </button>
              <span v-else class="text-muted">-</span>
            </template>
            <template #cell-photo="{ row }">
              <span v-if="row.status !== 'completed'" class="text-muted">-</span>
              <LoadingSpinner
                v-else-if="gpose.photoUploading === row.id"
                :label="`Uploading... ${gpose.photoProgress}%`"
              />
              <div v-else-if="row.photoUrl" class="flex-toolbar flex-toolbar--tight">
                <a
                  class="btn-view btn-sm"
                  :href="row.photoUrl"
                  target="_blank"
                  rel="noopener"
                  :aria-label="`View ${row.characterName}'s gpose`"
                  title="View"
                >
                  <font-awesome-icon :icon="['fas', 'eye']" />
                </a>
                <button
                  type="button"
                  class="btn-action btn-sm"
                  :disabled="gpose.photoUploading !== null"
                  :aria-label="`Replace ${row.characterName}'s gpose`"
                  title="Replace"
                  @click="pickPhoto(row)"
                >
                  <font-awesome-icon :icon="['fas', 'rotate']" />
                </button>
                <button
                  type="button"
                  class="btn-danger btn-sm"
                  :aria-label="`Delete ${row.characterName}'s gpose`"
                  title="Delete"
                  @click="gpose.deletePhoto(row)"
                >
                  <font-awesome-icon :icon="['fas', 'trash']" />
                </button>
                <span class="text-sm text-muted nowrap">
                  Until {{ formatGposeTime(row.photoExpiresAt) }}
                </span>
              </div>
              <button
                v-else
                type="button"
                class="btn-action btn-sm"
                :disabled="gpose.photoUploading !== null"
                :aria-label="`Upload ${row.characterName}'s gpose`"
                title="They can download it for a day"
                @click="pickPhoto(row)"
              >
                <font-awesome-icon :icon="['fas', 'upload']" /> Upload
              </button>
            </template>
            <template #cell-actions="{ row }">
              <div class="row-actions">
                <button
                  type="button"
                  class="btn-neutral btn-sm"
                  :disabled="gpose.changing !== null"
                  title="Put it back as waiting"
                  @click="setStatus(row, 'waiting')"
                >
                  <font-awesome-icon :icon="['fas', 'arrow-rotate-left']" /> Undo
                </button>
                <button
                  type="button"
                  class="btn-danger btn-sm"
                  :aria-label="`Delete ${row.characterName}'s sign-up`"
                  title="Delete"
                  @click="gpose.deleteSignup(row)"
                >
                  <font-awesome-icon :icon="['fas', 'trash']" />
                </button>
              </div>
            </template>
          </DataTable>
          <input
            ref="photoInput"
            type="file"
            :accept="PHOTO_ACCEPT"
            aria-label="Gpose image"
            hidden
            @change="onPhotoPicked"
          />
        </template>
      </template>
    </AdminPanel>

    <!-- -- List -------------------------------------------------------------------- -->
    <ManagerView v-else title="Gpose Booth" :icon="['fad', 'camera-retro']">
      <template #actions>
        <button type="button" class="btn-view btn-sm" @click="openTypes">
          <font-awesome-icon :icon="['fas', 'tag']" /> Manage Types
        </button>
        <button type="button" class="btn-confirm btn-sm" @click="openNew">
          <font-awesome-icon :icon="['fas', 'plus']" /> New Booth
        </button>
      </template>

      <LoadingSpinner
        v-if="gpose.boothsLoading && !gpose.booths.length"
        block
        label="Loading booths..."
      />
      <template v-else>
        <h4 class="section-heading">
          <font-awesome-icon :icon="['fad', 'camera-retro']" /> Open Booths
        </h4>
        <div v-if="gpose.openBooths.length" class="list-rows">
          <ListRow v-for="b in gpose.openBooths" :key="b.id">
            <strong>{{ b.title }}</strong>
            <span class="badge badge--accent">{{ modeLabel(b) }}</span>
            <div class="text-sm text-muted">
              {{ countsLabel(b) }}
              <template v-if="b.festivalMapName"> - {{ b.festivalMapName }}</template>
            </div>
            <template #actions>
              <button type="button" class="btn-view btn-sm" @click="openBooth(b)">
                <font-awesome-icon :icon="['fas', 'eye']" /> Sign-ups
              </button>
            </template>
          </ListRow>
        </div>
        <EmptyState v-else text="No open booths." />

        <template v-if="gpose.closedBooths.length">
          <h4 class="section-heading mt-20">
            <font-awesome-icon :icon="['fad', 'lock']" /> Closed Booths
          </h4>
          <div class="list-rows">
            <ListRow v-for="b in gpose.closedBooths" :key="b.id">
              <strong>{{ b.title }}</strong>
              <div class="text-sm text-muted">{{ countsLabel(b) }}</div>
              <template #actions>
                <button type="button" class="btn-view btn-sm" @click="openBooth(b)">
                  <font-awesome-icon :icon="['fas', 'eye']" /> View
                </button>
              </template>
            </ListRow>
          </div>
        </template>
      </template>
    </ManagerView>
  </div>
</template>
