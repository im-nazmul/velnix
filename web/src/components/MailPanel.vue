<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from "vue";
import { request } from "../api";
import { bytes, type MailPage, type Message, type Change } from "../types";
import Icon from "./Icon.vue";
const props = defineProps<{ refresh: number }>();
const emit = defineEmits<{ review: [Change] }>();
const listing = ref<MailPage | null>(null),
  selected = ref<Message | null>(null),
  error = ref(""),
  busy = ref(false),
  offset = ref(0);
let controller: AbortController | undefined,
  sequence = 0;
async function load() {
  controller?.abort();
  controller = new AbortController();
  const n = ++sequence;
  busy.value = true;
  error.value = "";
  selected.value = null;
  try {
    const p = await request<MailPage>(
      `/api/mail?offset=${offset.value}`,
      undefined,
      controller.signal,
    );
    if (n === sequence) listing.value = p;
  } catch (e) {
    if (
      n === sequence &&
      !(e instanceof DOMException && e.name === "AbortError")
    ) {
      error.value = (e as Error).message;
      listing.value = null;
    }
  } finally {
    if (n === sequence) busy.value = false;
  }
}
async function show(id: string) {
  controller?.abort();
  controller = new AbortController();
  const n = ++sequence;
  busy.value = true;
  error.value = "";
  try {
    const m = await request<Message>(
      `/api/mail/${encodeURIComponent(id)}`,
      undefined,
      controller.signal,
    );
    if (n === sequence) selected.value = m;
  } catch (e) {
    if (
      n === sequence &&
      !(e instanceof DOMException && e.name === "AbortError")
    )
      error.value = (e as Error).message;
  } finally {
    if (n === sequence) busy.value = false;
  }
}
watch(() => props.refresh, load, { immediate: true });
onBeforeUnmount(() => {
  sequence++;
  controller?.abort();
});
function page(delta: number) {
  offset.value = Math.max(0, offset.value + delta);
  void load();
}
</script>
<template>
  <div class="intro">
    <div>
      <p class="eyebrow">DEVELOPMENT MAIL</p>
      <h1>Inbox, without the send.</h1>
      <p class="muted">
        Plain-text inspection. Local capture. No external delivery.
      </p>
    </div>
    <button
      class="secondary"
      :disabled="!listing || busy"
      @click="emit('review', { operation: 'prune-mail' })"
    >
      Review retention prune
    </button>
  </div>
  <div v-if="error" role="alert" class="notice warning">{{ error }}</div>
  <p v-if="busy" role="status" class="muted">Reading local inbox…</p>
  <section v-if="!listing && !busy" class="panel empty">
    <Icon name="mail" :size="35" />
    <h2>Your inbox is unavailable.</h2>
    <p>Start the separately reviewed foreground capture in a terminal.</p>
    <code>velnix mail serve --dry-run</code>
  </section>
  <div v-if="listing" class="mail-layout">
    <section class="panel inbox">
      <div class="panel-heading">
        <h2>
          Inbox <span class="badge">{{ listing.total }}</span>
        </h2>
      </div>
      <div v-if="!listing.messages.length" class="empty">
        <Icon name="mail" :size="30" />
        <h3>Nothing here yet.</h3>
        <p>Messages appear while local capture is running.</p>
      </div>
      <button
        v-for="m in listing.messages"
        :key="m.ID"
        class="message-row"
        :class="{ selected: selected?.id === m.ID }"
        :disabled="busy"
        @click="show(m.ID)"
      >
        <strong>{{ m.Subject || "(No subject)" }}</strong
        ><span>{{ m.From.Name || m.From.Address }}</span
        ><small
          >{{ new Date(m.Created).toLocaleString() }} ·
          {{ bytes(m.Size) }}</small
        >
      </button>
      <div class="pagination">
        <button
          class="text-button"
          :disabled="offset === 0 || busy"
          @click="page(-50)"
        >
          Previous</button
        ><span class="muted"
          >{{ offset + 1 }}–{{ Math.min(offset + 50, listing.total) }}</span
        ><button
          class="text-button"
          :disabled="offset + 50 >= listing.total || offset + 50 > 1000 || busy"
          @click="page(50)"
        >
          Next
        </button>
      </div>
    </section>
    <section class="panel message-detail">
      <template v-if="selected"
        ><p class="eyebrow">PLAIN-TEXT MESSAGE</p>
        <h2>{{ selected.headers.Subject || "(No subject)" }}</h2>
        <dl class="environment">
          <div
            v-for="k in [
              'From',
              'To',
              'Cc',
              'Bcc',
              'Return-Path',
              'Date',
            ].filter((k) => selected?.headers[k])"
            :key="k"
          >
            <dt>{{ k }}</dt>
            <dd>{{ selected.headers[k] }}</dd>
          </div>
        </dl>
        <div v-if="selected.notice" class="soft-note">
          {{ selected.notice }}
        </div>
        <pre class="message-text">{{ selected.text }}</pre>
        <p v-if="selected.truncated" class="muted">
          Text truncated at the display limit.
        </p>
        <div v-if="selected.attachments.length" class="attachments">
          <h3>Attachments</h3>
          <a
            v-for="a in selected.attachments"
            :key="a.id"
            :href="`/api/mail/${encodeURIComponent(selected.id)}/attachment/${encodeURIComponent(a.id)}`"
            class="attachment"
            ><Icon name="download" /><span
              >{{ a.name || "Unnamed attachment"
              }}<small>{{ bytes(a.size) }} · {{ a.content_type }}</small></span
            ></a
          >
        </div>
        <p class="muted small">
          HTML preview is unavailable in this release. Attachments download as
          files and are never opened automatically.
        </p></template
      >
      <div v-else class="empty">
        <Icon name="mail" :size="35" />
        <h2>A closer look.</h2>
        <p>Select a message to inspect its plain text and attachments.</p>
      </div>
    </section>
  </div>
</template>
