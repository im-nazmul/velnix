<script setup lang="ts">
import { computed } from "vue";
import type { Snapshot, Page, Change } from "../types";
import { version } from "../types";
import Icon from "./Icon.vue";
const props = defineProps<{ data: Snapshot }>();
const emit = defineEmits<{ navigate: [Page]; review: [Change] }>();
const projects = computed(() => props.data.projects ?? []);
const selections = computed(() =>
    projects.value.reduce(
        (sum, p) =>
            sum +
            ["php", "node", "bun", "python"].filter((k) => version(p, k))
                .length,
        0,
    ),
);
const active = computed(
    () => (props.data.workloads ?? []).filter((w) => w.alive).length,
);
</script>
<template>
    <div class="intro">
        <div>
            <p class="eyebrow">YOUR LOCAL ENVIRONMENT</p>
            <h1>Your workspace, in focus.</h1>
            <p class="muted">A little less setup. A little more building.</p>
        </div>
        <button class="primary" @click="emit('navigate', 'projects')">
            <Icon name="plus" /> Link a project
        </button>
    </div>
    <div v-if="!data.state?.initialized" class="notice">
        <div>
            <strong>Make room for your first project.</strong>
            <p>
                Initialize Velnix’s private registry to start managing local
                links. Your project files stay yours.
            </p>
        </div>
        <button
            class="secondary"
            @click="emit('review', { operation: 'initialize' })"
        >
            Review initialization <Icon name="arrow" />
        </button>
    </div>
    <div class="metrics">
        <article>
            <span class="metric-icon"><Icon name="projects" /></span>
            <p>Linked projects</p>
            <strong>{{ projects.length }}</strong
            ><small>In your local registry</small>
        </article>
        <article>
            <span class="metric-icon"><Icon name="services" /></span>
            <p>Active services</p>
            <strong>{{ active }}</strong
            ><small>Verified owned workloads</small>
        </article>
        <article>
            <span class="metric-icon"><Icon name="runtimes" /></span>
            <p>Runtime selections</p>
            <strong>{{ selections }}</strong
            ><small>Across linked projects</small>
        </article>
        <article>
            <span class="metric-icon"><Icon name="certificates" /></span>
            <p>Local certificates</p>
            <strong>{{ data.certificate?.names.length ?? 0 }}</strong
            ><small>{{
                data.certificate?.phase === "ready"
                    ? "Names in the current certificate"
                    : "No ready certificate recorded"
            }}</small>
        </article>
    </div>
    <div class="overview-grid">
        <section class="panel">
            <div class="panel-heading">
                <div>
                    <h2>Your projects</h2>
                    <p class="muted">Pick up where you left off.</p>
                </div>
                <button
                    class="text-button"
                    @click="emit('navigate', 'projects')"
                >
                    View all <Icon name="arrow" :size="16" />
                </button>
            </div>
            <div v-if="!projects.length" class="empty">
                <span class="empty-icon"
                    ><Icon name="projects" :size="30"
                /></span>
                <h3>A fresh workspace</h3>
                <p>
                    Link an existing directory to keep its domain<br />and
                    runtime selections in one place.
                </p>
                <button class="secondary" @click="emit('navigate', 'projects')">
                    Link your first project
                </button>
            </div>
            <div v-else class="project-list">
                <button
                    v-for="p in projects.slice(0, 5)"
                    :key="p.name"
                    @click="emit('navigate', 'projects')"
                >
                    <span class="project-avatar">{{
                        p.name.slice(0, 2).toUpperCase()
                    }}</span
                    ><span
                        ><strong>{{ p.name }}</strong
                        ><small>{{ p.domain }}</small></span
                    ><Icon name="arrow" :size="17" />
                </button>
            </div>
        </section>
        <section class="panel">
            <div class="panel-heading">
                <div>
                    <h2>Environment</h2>
                    <p class="muted">Native tools. Clear ownership.</p>
                </div>
                <span class="badge">LOCAL</span>
            </div>
            <dl class="environment">
                <div>
                    <dt>Platform</dt>
                    <dd>
                        {{ data.platform?.distribution ?? "Unavailable" }}
                        {{ data.platform?.distribution_version }}
                    </dd>
                </div>
                <div>
                    <dt>Architecture</dt>
                    <dd>{{ data.platform?.architecture ?? "Unavailable" }}</dd>
                </div>
                <div>
                    <dt>Environment</dt>
                    <dd>
                        {{
                            data.platform
                                ? data.platform.wsl
                                    ? "WSL"
                                    : "Native Linux"
                                : "Unavailable"
                        }}
                    </dd>
                </div>
                <div>
                    <dt>Registry</dt>
                    <dd>
                        <span
                            class="status-dot"
                            :class="{
                                on:
                                    data.state?.initialized &&
                                    !data.state?.recovery_required,
                            }"
                        ></span
                        >{{
                            data.state?.recovery_required
                                ? "Needs recovery"
                                : data.state?.initialized
                                  ? "Initialized"
                                  : "Not initialized"
                        }}
                    </dd>
                </div>
            </dl>
            <div class="soft-note">
                <Icon name="lock" />
                <p>
                    Everything stays local. Changes always start with a plan you
                    can review.
                </p>
            </div>
            <button
                class="text-button"
                @click="emit('navigate', 'diagnostics')"
            >
                Open diagnostics <Icon name="arrow" :size="16" />
            </button>
        </section>
    </div>
    <section class="panel compact">
        <div class="section-inline">
            <span class="metric-icon"><Icon name="mail" /></span>
            <div>
                <h3>A quiet place for development mail</h3>
                <p class="muted">
                    Inspect captured messages without sending anything to the
                    outside world.
                </p>
            </div>
            <button class="secondary" @click="emit('navigate', 'mail')">
                Open inbox <Icon name="arrow" :size="16" />
            </button>
        </div>
    </section>
</template>
