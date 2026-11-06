<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from "vue";
import type { Plan } from "../types";
import Icon from "./Icon.vue";
const props = defineProps<{ plan: Plan; busy: boolean }>();
const emit = defineEmits<{ close: []; apply: [] }>();
const dialog = ref<HTMLDialogElement>();
let prior: HTMLElement | null = null;
const detail = computed(
  () =>
    (props.plan.review.detail.Plan ?? props.plan.review.detail) as Record<
      string,
      unknown
    >,
);
const effects = computed(() =>
  Array.isArray(detail.value.effects) ? (detail.value.effects as string[]) : [],
);
onMounted(() => {
  prior = document.activeElement as HTMLElement;
  dialog.value?.showModal();
});
onBeforeUnmount(() => {
  dialog.value?.close();
  prior?.focus();
});
function keys(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    if (!props.busy) emit("close");
  }
}
</script>
<template>
  <dialog
    ref="dialog"
    class="review-dialog"
    aria-labelledby="plan-title"
    @cancel.prevent="!busy && emit('close')"
    @keydown="keys"
  >
    <div class="panel-heading">
      <div>
        <p class="eyebrow">REVIEW BEFORE APPLYING</p>
        <h2 id="plan-title">
          {{ plan.review.change.operation.replaceAll("-", " ") }}
        </h2>
      </div>
      <button
        aria-label="Close review"
        class="icon-button"
        :disabled="busy"
        @click="emit('close')"
      >
        <Icon name="close" />
      </button>
    </div>
    <p class="muted">Only the changes in this exact plan will be approved.</p>
    <div class="review-target">
      <span>Target</span
      ><strong>{{
        plan.review.change.name ||
        (plan.review.change.operation === "prune-mail"
          ? "Owned development inbox"
          : "Private Velnix state")
      }}</strong>
    </div>
    <ul class="effects">
      <li v-for="effect in effects" :key="effect">
        <Icon name="check" :size="16" />{{ effect }}
      </li>
    </ul>
    <div v-for="note in plan.review.notes" :key="note" class="soft-note">
      {{ note }}
    </div>
    <details>
      <summary>Exact backend plan and identity</summary>
      <pre class="code-block">{{
        JSON.stringify(plan.review.detail, null, 2)
      }}</pre>
      <p class="mono small">Plan {{ plan.review.id }}</p>
    </details>
    <p class="muted small">
      This preview expires at {{ new Date(plan.expires).toLocaleTimeString() }}.
      A changed environment requires another review.
    </p>
    <div class="dialog-actions">
      <button class="secondary" :disabled="busy" @click="emit('close')">
        Cancel</button
      ><button class="primary" :disabled="busy" @click="emit('apply')">
        {{ busy ? "Applying…" : "Apply this exact plan"
        }}<Icon v-if="!busy" name="arrow" />
      </button>
    </div>
  </dialog>
</template>
