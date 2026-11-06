<script setup lang="ts">
import { computed, ref } from "vue";
import type { Snapshot, Change } from "../types";
import { version } from "../types";
import Icon from "./Icon.vue";
const props = defineProps<{ data: Snapshot }>();
const emit = defineEmits<{ review: [Change] }>();
const filter = ref(""),
  form = ref(false),
  name = ref(""),
  root = ref(""),
  documentRoot = ref(".");
const projects = computed(() =>
  (props.data.projects ?? []).filter((p) =>
    `${p.name} ${p.domain} ${p.root}`
      .toLowerCase()
      .includes(filter.value.toLowerCase()),
  ),
);
function link() {
  emit("review", {
    operation: "link",
    name: name.value,
    root: root.value,
    document_root: documentRoot.value,
  });
}
</script>
<template>
  <div class="intro">
    <div>
      <p class="eyebrow">PROJECT REGISTRY</p>
      <h1>A home for every project.</h1>
      <p class="muted">
        Link existing directories. Keep source files exactly where they belong.
      </p>
    </div>
    <button
      class="primary"
      :disabled="!data.state?.initialized"
      @click="form = !form"
    >
      <Icon :name="form ? 'close' : 'plus'" />{{
        form ? "Close form" : "Link project"
      }}
    </button>
  </div>
  <div v-if="!data.state?.initialized" class="notice">
    <p>
      Initialize your private registry from Overview before linking a project.
    </p>
    <button
      class="secondary"
      @click="emit('review', { operation: 'initialize' })"
    >
      Review initialization
    </button>
  </div>
  <form v-if="form" class="panel form-grid" @submit.prevent="link">
    <h2 class="full">Link an existing directory</h2>
    <label
      >Project name<input
        v-model="name"
        required
        pattern="[a-z0-9][a-z0-9-]*"
        maxlength="63"
        placeholder="my-project" /></label
    ><label
      >Document root<input
        v-model="documentRoot"
        required
        placeholder="public" /></label
    ><label class="full"
      >Absolute project path<input
        v-model="root"
        required
        pattern="/.*"
        placeholder="/home/you/projects/my-project"
    /></label>
    <p class="muted full">
      This creates a registry link and reserves a .test name. DNS and active web
      routes need their existing CLI setup.
    </p>
    <button class="primary" type="submit">
      Review link <Icon name="arrow" />
    </button>
  </form>
  <div class="toolbar">
    <label class="search"
      ><Icon name="search" /><input
        v-model="filter"
        aria-label="Filter projects"
        placeholder="Find a project…" /></label
    ><span class="muted">{{ projects.length }} projects</span>
  </div>
  <section v-if="!projects.length" class="panel empty">
    <Icon name="projects" :size="35" />
    <h2>
      {{ filter ? "No matching projects" : "Your next idea starts here." }}
    </h2>
    <p>
      {{
        filter
          ? "Try a different name, domain or path."
          : "Link a directory to see its local domain and selected runtimes."
      }}
    </p>
  </section>
  <div v-else class="projects-grid">
    <article v-for="p in projects" :key="p.name" class="panel project-card">
      <div class="section-inline">
        <span class="project-avatar">{{
          p.name.slice(0, 2).toUpperCase()
        }}</span>
        <div>
          <h2>{{ p.name }}</h2>
          <p class="mono muted">{{ p.domain }}</p>
        </div>
        <span class="badge">LINKED</span>
      </div>
      <p class="path">{{ p.root }}</p>
      <div class="runtime-chips">
        <span
          v-for="kind in ['php', 'node', 'bun', 'python'].filter((k) =>
            version(p, k),
          )"
          :key="kind"
          class="badge"
          >{{ kind }} {{ version(p, kind) }}</span
        ><span
          v-if="!['php', 'node', 'bun', 'python'].some((k) => version(p, k))"
          class="muted"
          >No runtime selected</span
        >
      </div>
      <details>
        <summary>Project details</summary>
        <dl class="environment">
          <div>
            <dt>Document root</dt>
            <dd class="path">{{ p.document_root }}</dd>
          </div>
          <div>
            <dt>Source ownership</dt>
            <dd>External directory</dd>
          </div>
          <div v-if="p.proxy_port">
            <dt>Proxy target</dt>
            <dd>127.0.0.1:{{ p.proxy_port }}</dd>
          </div>
        </dl>
      </details>
      <div class="card-footer">
        <span class="muted">Registry link only</span
        ><button
          class="danger text-button"
          @click="emit('review', { operation: 'unlink', name: p.name })"
        >
          Review unlink
        </button>
      </div>
    </article>
  </div>
</template>
