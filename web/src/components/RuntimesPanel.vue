<script setup lang="ts">
import { computed, ref } from "vue";
import type { Snapshot, Change } from "../types";
import { version } from "../types";
import Icon from "./Icon.vue";
const props = defineProps<{ data: Snapshot }>();
const emit = defineEmits<{ review: [Change] }>();
const kind = ref("php"),
  project = ref(""),
  executable = ref(""),
  exact = ref("");
const inventories = computed(() =>
  (props.data.runtimes ?? []).map((r) => ({
    kind: r.kind,
    candidates: Array.isArray(r.value) ? r.value : r.value.candidates,
    issues: Array.isArray(r.value) ? [] : r.value.issues,
  })),
);
</script>
<template>
  <div class="intro">
    <div>
      <p class="eyebrow">EXISTING INSTALLATIONS</p>
      <h1>The right tools, per project.</h1>
      <p class="muted">
        Discovery is passive. Detected tools stay externally owned.
      </p>
    </div>
    <span class="badge">NO AUTOMATIC PROBES</span>
  </div>
  <div class="runtime-layout">
    <section class="panel">
      <div class="panel-heading">
        <div>
          <h2>Detected candidates</h2>
          <p class="muted">Paths are hints, not verified versions.</p>
        </div>
      </div>
      <div v-for="r in inventories" :key="r.kind" class="inventory">
        <h3>{{ r.kind }}</h3>
        <p v-if="!r.candidates.length" class="muted">
          No candidates found in supported directories.
        </p>
        <button
          v-for="c in r.candidates"
          :key="c.path"
          class="candidate"
          @click="
            kind = r.kind;
            executable = c.path;
          "
        >
          <span class="code-icon"><Icon name="runtimes" :size="17" /></span
          ><span
            ><code>{{ c.path }}</code
            ><small
              >External · {{ c.probed ? "Verified" : "Not probed" }}</small
            ></span
          ><Icon name="plus" :size="16" />
        </button>
        <p v-for="i in r.issues" :key="i.directory" class="muted">
          {{ i.directory }}: {{ i.reason }}
        </p>
      </div>
    </section>
    <form
      class="panel form-stack"
      @submit.prevent="
        emit('review', {
          operation: 'select-runtime',
          name: project,
          kind,
          executable,
          version: exact,
        })
      "
    >
      <h2>Select a runtime</h2>
      <p class="muted">
        Apply verifies the exact executable’s version before storing the
        selection.
      </p>
      <label
        >Project<select v-model="project" required>
          <option value="" disabled>Choose a linked project</option>
          <option
            v-for="p in data.projects ?? []"
            :key="p.name"
            :value="p.name"
          >
            {{ p.name }}
          </option>
        </select></label
      ><label
        >Runtime<select v-model="kind">
          <option value="php">PHP</option>
          <option value="node">Node.js</option>
          <option value="bun">Bun</option>
          <option value="python">Python</option>
        </select></label
      ><label
        >Absolute executable path<input
          v-model="executable"
          required
          pattern="/.*"
          placeholder="/usr/bin/php" /></label
      ><label
        >Exact installed version<input
          v-model="exact"
          required
          placeholder="e.g. 8.3.12" /></label
      ><button class="primary" :disabled="!data.projects?.length" type="submit">
        Review selection <Icon name="arrow" />
      </button>
    </form>
  </div>
  <section class="panel">
    <div class="panel-heading">
      <div>
        <h2>Project selections</h2>
        <p class="muted">
          Stored identity evidence for your next explicit execution.
        </p>
      </div>
    </div>
    <p v-if="!data.projects?.length" class="muted">
      Link a project to manage its selections.
    </p>
    <div v-for="p in data.projects ?? []" :key="p.name" class="selection-row">
      <strong>{{ p.name }}</strong>
      <div>
        <template v-for="k in ['php', 'node', 'bun', 'python']" :key="k"
          ><span v-if="version(p, k)" class="selection-chip"
            >{{ k }} {{ version(p, k)
            }}<button
              :aria-label="`Review clearing ${k} for ${p.name}`"
              @click="
                emit('review', {
                  operation: 'clear-runtime',
                  name: p.name,
                  kind: k,
                })
              "
            >
              <Icon name="close" :size="13" /></button></span></template
        ><span
          v-if="!['php', 'node', 'bun', 'python'].some((k) => version(p, k))"
          class="muted"
          >No runtime selections</span
        >
      </div>
    </div>
  </section>
</template>
