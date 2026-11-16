<script setup lang="ts">
/**
 * One labelled form control. Wraps the control (default slot) with a label and
 * optional help text, giving every admin form the same field rhythm. Controls
 * placed in the default slot stretch full-width automatically (see the `.field`
 * rule in utilities.css).
 *
 * The label can be passed as the `label` prop or, when it needs markup, via the
 * `#label` slot. Help text likewise supports a `help` prop (plain string) or a
 * `#help` slot for markup such as links.
 *
 * The label names its control: `htmlFor` ties it to a control by id, and without it a
 * field holding exactly one form control is tied to that one, which gets an id if it
 * has none. A field holding several (a group of checkboxes) is left for its own
 * markup to name, since one label can't name each of them. A control in a toolbar (the
 * Markdown editor's text-style picker) is a tool for the field, not its value, so it
 * is not counted.
 */
import { onMounted, onUpdated, ref, useId, useTemplateRef } from 'vue'

const props = defineProps<{
  label?: string
  /** Marks the field required: appends a `*` to the label. */
  required?: boolean
  /** Plain-text help shown beneath the control. Use the #help slot for markup. */
  help?: string
  /** Associates the label with a control via `for`/`id`. */
  htmlFor?: string
}>()

const root = useTemplateRef<HTMLDivElement>('root')
const fallbackId = useId()
const linkedId = ref<string>()

/** Ties the label to the field's one control when no `htmlFor` names it. */
function linkControl(): void {
  if (props.htmlFor || !root.value) {
    linkedId.value = undefined
    return
  }
  const controls = [
    ...root.value.querySelectorAll<HTMLElement>('input:not([type="hidden"]), select, textarea'),
  ].filter((control) => !control.closest('[role="toolbar"]'))
  const control = controls.length === 1 ? controls[0] : undefined
  if (!control) {
    linkedId.value = undefined
    return
  }
  if (!control.id) {
    control.id = fallbackId
  }
  linkedId.value = control.id
}

onMounted(linkControl)
onUpdated(linkControl)
</script>

<template>
  <div ref="root" class="field">
    <label v-if="label || $slots.label" class="field-label" :for="htmlFor ?? linkedId">
      <slot name="label">{{ label }}</slot><span
        v-if="required"
        class="form-field__required"
        aria-hidden="true"
        >*</span
      >
    </label>
    <slot />
    <small v-if="$slots.help || help" class="form-field__help">
      <slot name="help">{{ help }}</slot>
    </small>
  </div>
</template>
