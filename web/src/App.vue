<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { APIError, csrf, request } from "./api";
import type { Change, Page, Plan, Snapshot } from "./types";
import Icon from "./components/Icon.vue";
import OverviewPanel from "./components/OverviewPanel.vue";
import ProjectsPanel from "./components/ProjectsPanel.vue";
import RuntimesPanel from "./components/RuntimesPanel.vue";
import MailPanel from "./components/MailPanel.vue";
import PlanDialog from "./components/PlanDialog.vue";
const navigation: { id: Page; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "projects", label: "Projects" },
    { id: "runtimes", label: "Runtimes" },
    { id: "services", label: "Owned services" },
    { id: "certificates", label: "Certificates" },
    { id: "mail", label: "Mail" },
    { id: "diagnostics", label: "Diagnostics" },
];
function currentPage(): Page {
    return navigation.some((n) => n.id === location.hash.slice(1))
        ? (location.hash.slice(1) as Page)
        : "overview";
}
const page = ref<Page>(currentPage()),
    auth = ref(false),
    checking = ref(true),
    loginCSRF = ref(""),
    secret = ref(""),
    loginError = ref(""),
    logging = ref(false),
    menu = ref(false),
    theme = ref(
        matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
    );
const data = ref<Snapshot | null>(null),
    loading = ref(false),
    error = ref(""),
    flash = ref(""),
    plan = ref<Plan | null>(null),
    planning = ref(false),
    applying = ref(false),
    refresh = ref(0);
const label = computed(
    () => navigation.find((n) => n.id === page.value)?.label ?? "Overview",
);
let controller: AbortController | undefined,
    sequence = 0;
async function challenge() {
    try {
        loginCSRF.value = (
            await request<{ csrf: string }>("/api/challenge")
        ).csrf;
    } catch (e) {
        loginError.value = (e as Error).message;
    }
}
function expired() {
    if (!auth.value) return;
    auth.value = false;
    csrf.value = "";
    data.value = null;
    plan.value = null;
    sequence++;
    controller?.abort();
    loginError.value =
        "Your session ended. Restart velnix ui for a fresh one-time login.";
    loginCSRF.value = "";
}
async function load() {
    if (!auth.value) return;
    controller?.abort();
    controller = new AbortController();
    const n = ++sequence;
    loading.value = true;
    error.value = "";
    data.value = null;
    try {
        const value = await request<Snapshot>(
            `/api/read/${page.value}`,
            undefined,
            controller.signal,
        );
        if (n === sequence) data.value = value;
    } catch (e) {
        if (
            n === sequence &&
            !(e instanceof DOMException && e.name === "AbortError")
        )
            error.value = (e as Error).message;
    } finally {
        if (n === sequence) loading.value = false;
    }
}
async function login() {
    logging.value = true;
    loginError.value = "";
    const token = secret.value;
    secret.value = "";
    try {
        const result = await request<{ csrf: string }>(
            "/api/login",
            { token },
            undefined,
            loginCSRF.value,
        );
        csrf.value = result.csrf;
        auth.value = true;
        loginCSRF.value = "";
        await load();
    } catch (e) {
        loginError.value = (e as Error).message;
    } finally {
        logging.value = false;
    }
}
async function logout() {
    try {
        await request("/api/logout", {});
        expired();
        loginError.value =
            "Signed out. Restart velnix ui to create a fresh one-time login.";
    } catch (e) {
        error.value = (e as Error).message;
    }
}
function navigate(target: Page) {
    page.value = target;
    location.hash = target;
    menu.value = false;
    flash.value = "";
    plan.value = null;
}
function hashChanged() {
    page.value = currentPage();
    menu.value = false;
}
watch(page, load);
async function review(change: Change) {
    if (planning.value) return;
    planning.value = true;
    error.value = "";
    flash.value = "";
    try {
        plan.value = await request<Plan>("/api/plan", change);
    } catch (e) {
        error.value = (e as Error).message;
    } finally {
        planning.value = false;
    }
}
async function apply() {
    if (!plan.value || applying.value) return;
    applying.value = true;
    const selected = plan.value;
    try {
        await request("/api/apply", {
            ticket: selected.ticket,
            id: selected.review.id,
        });
        flash.value = "The reviewed change was applied.";
    } catch (e) {
        error.value =
            e instanceof APIError && e.uncertain
                ? "The outcome is uncertain. Inspect current state before reviewing another plan."
                : (e as Error).message;
    } finally {
        applying.value = false;
        plan.value = null;
        const saved = error.value;
        await load();
        if (saved) error.value = saved;
        refresh.value++;
    }
}
async function refreshPage() {
    await load();
    refresh.value++;
}
const certificateDate = computed(() =>
    data.value?.certificate
        ? new Date(data.value.certificate.expires).toLocaleString()
        : "",
);
onMounted(async () => {
    window.addEventListener("hashchange", hashChanged);
    window.addEventListener("velnix-session-expired", expired);
    try {
        csrf.value = (await request<{ csrf: string }>("/api/session")).csrf;
        auth.value = true;
        await load();
    } catch {
        await challenge();
    } finally {
        checking.value = false;
    }
});
onBeforeUnmount(() => {
    window.removeEventListener("hashchange", hashChanged);
    window.removeEventListener("velnix-session-expired", expired);
    controller?.abort();
});
</script>
<template>
    <div class="application" :data-theme="theme">
        <div v-if="checking" class="initial-load" role="status">
            Opening your local workspace…
        </div>
        <main v-else-if="!auth" class="login-layout">
            <section class="login-story">
                <a class="brand" href="#overview"
                    ><img
                        src="/favicon.svg"
                        alt=""
                        width="44"
                        height="44"
                    /><span>velnix<span class="brand-dot">.</span></span></a
                >
                <div>
                    <p class="eyebrow">BUILT FOR YOUR OWN MACHINE</p>
                    <h1>
                        Your tools.<br />Your projects.<br />Your workspace.
                    </h1>
                    <p>
                        A native home for local development.<br />Everything you
                        need, a little closer.
                    </p>
                </div>
                <p class="story-footer">
                    <span class="status-dot on"></span> LOCAL BY DESIGN
                </p>
            </section>
            <section class="login-form-section">
                <form class="login-form" @submit.prevent="login">
                    <span class="empty-icon"
                        ><Icon name="lock" :size="28"
                    /></span>
                    <p class="eyebrow">PRIVATE LOCAL SESSION</p>
                    <h2>Welcome to your workspace.</h2>
                    <p class="muted">
                        Read the one-time secret file shown in your Velnix
                        terminal, then paste its contents below.
                    </p>
                    <label
                        >One-time login secret<input
                            v-model="secret"
                            type="password"
                            autocomplete="off"
                            spellcheck="false"
                            required
                            minlength="64"
                            maxlength="64"
                            :disabled="!loginCSRF || logging"
                            placeholder="Paste your 64-character secret"
                            autofocus /></label
                    ><button
                        class="primary"
                        type="submit"
                        :disabled="!loginCSRF || logging"
                    >
                        {{ logging ? "Opening workspace…" : "Open workspace"
                        }}<Icon name="arrow" />
                    </button>
                    <p v-if="loginError" class="notice warning" role="alert">
                        {{ loginError }}
                    </p>
                    <div class="soft-note">
                        <Icon name="lock" />
                        <p>
                            This session stays on your machine. The secret is
                            single-use and expires after five minutes.
                        </p>
                    </div>
                </form>
            </section>
        </main>
        <template v-else
            ><div v-if="menu" class="nav-scrim" @click="menu = false"></div>
            <aside class="sidebar" :class="{ open: menu }">
                <a class="brand" href="#overview" @click="navigate('overview')"
                    ><img
                        src="/favicon.svg"
                        width="36"
                        height="36"
                        alt=""
                    /><span>velnix<span class="brand-dot">.</span></span></a
                >
                <p class="nav-caption">WORKSPACE</p>
                <nav aria-label="Workspace">
                    <a
                        v-for="item in navigation"
                        :key="item.id"
                        :href="`#${item.id}`"
                        :class="{ active: page === item.id }"
                        :aria-current="page === item.id ? 'page' : undefined"
                        @click.prevent="navigate(item.id)"
                        ><Icon :name="item.id" /><span>{{ item.label }}</span
                        ><span
                            v-if="item.id === 'overview'"
                            class="nav-marker"
                            aria-hidden="true"
                            >●</span
                        ></a
                    >
                </nav>
                <div class="sidebar-bottom">
                    <div class="local-card">
                        <span class="status-dot on"></span>
                        <div>
                            <strong>On your machine</strong
                            ><small>Local session · Loopback only</small>
                        </div>
                    </div>
                    <button class="sidebar-logout" @click="logout">
                        <Icon name="logout" /> Sign out</button
                    ><small class="sidebar-footnote"
                        >NATIVE LINUX DEVELOPMENT</small
                    >
                </div>
            </aside>
            <div class="workspace">
                <header class="topbar">
                    <div class="breadcrumb">
                        <button
                            class="icon-button menu-toggle"
                            aria-label="Open navigation"
                            :aria-expanded="menu"
                            @click="menu = !menu"
                        >
                            <Icon name="menu" /></button
                        ><span>Workspace</span><span class="slash">/</span
                        ><strong>{{ label }}</strong>
                    </div>
                    <div class="header-actions">
                        <span class="connection"
                            ><span class="status-dot on"></span>Local
                            connection</span
                        ><button
                            class="icon-button"
                            :aria-label="
                                theme === 'dark'
                                    ? 'Use light theme'
                                    : 'Use dark theme'
                            "
                            @click="theme = theme === 'dark' ? 'light' : 'dark'"
                        >
                            <Icon name="sun" /></button
                        ><button
                            class="secondary refresh"
                            :disabled="loading || applying"
                            @click="refreshPage"
                        >
                            <Icon name="refresh" :size="16" />Refresh
                        </button>
                    </div>
                </header>
                <main id="main-content" class="content">
                    <div v-if="error" role="alert" class="notice warning">
                        {{ error }}
                    </div>
                    <div v-if="flash" role="status" class="notice success">
                        <Icon name="check" />{{ flash }}
                    </div>
                    <div v-if="planning" role="status" class="notice">
                        Preparing the exact backend plan…
                    </div>
                    <div v-if="loading" role="status" class="loading-panel">
                        <span class="loading-dot"></span>Reading your local
                        environment…
                    </div>
                    <template v-if="data && !loading"
                        ><div
                            v-for="issue in data.issues"
                            :key="issue.area"
                            class="issue"
                        >
                            <strong>{{ issue.area }}</strong>
                            {{ issue.message }}
                        </div>
                        <p v-if="data.truncated" class="notice warning">
                            This view is limited to the first 200 projects. Use
                            the CLI to inspect the full registry.
                        </p>
                        <OverviewPanel
                            v-if="page === 'overview'"
                            :data="data"
                            @navigate="navigate"
                            @review="review"
                        />
                        <ProjectsPanel
                            v-if="page === 'projects'"
                            :data="data"
                            @review="review"
                        />
                        <RuntimesPanel
                            v-if="page === 'runtimes'"
                            :data="data"
                            @review="review"
                        />
                        <template v-if="page === 'services'"
                            ><div class="intro">
                                <div>
                                    <p class="eyebrow">
                                        EXPLICIT PROCESS OWNERSHIP
                                    </p>
                                    <h1>A clear view of what’s running.</h1>
                                    <p class="muted">
                                        Only workloads owned by Velnix.
                                        Lifecycle stays in their foreground
                                        terminal.
                                    </p>
                                </div>
                            </div>
                            <div class="projects-grid">
                                <article
                                    v-for="w in data.workloads ?? []"
                                    :key="w.kind + w.name"
                                    class="panel service-card"
                                >
                                    <span class="metric-icon"
                                        ><Icon
                                            :name="
                                                w.kind === 'Mail'
                                                    ? 'mail'
                                                    : 'services'
                                            "
                                            :size="25"
                                    /></span>
                                    <h2>{{ w.kind }} {{ w.name }}</h2>
                                    <span
                                        class="badge"
                                        :class="{ running: w.alive }"
                                        ><span
                                            class="status-dot"
                                            :class="{ on: w.alive }"
                                        ></span
                                        >{{
                                            w.alive ? "Running" : w.phase
                                        }}</span
                                    >
                                    <p v-if="w.port" class="mono muted">
                                        127.0.0.1:{{ w.port }}
                                    </p>
                                    <p
                                        v-if="w.recovery_required"
                                        class="notice warning"
                                    >
                                        Recovery required. Inspect the existing
                                        CLI workflow.
                                    </p>
                                    <p v-if="w.reason" class="muted">
                                        {{ w.reason }}
                                    </p>
                                    <p class="small muted">
                                        Start, stop and reload through the
                                        existing reviewed CLI/TUI session.
                                    </p>
                                </article>
                            </div>
                            <div class="soft-note">
                                <Icon name="lock" />
                                <p>
                                    System databases and externally managed
                                    services are outside this view. No
                                    system-service commands are executed.
                                </p>
                            </div></template
                        >
                        <template v-if="page === 'certificates'"
                            ><div class="intro">
                                <div>
                                    <p class="eyebrow">LOCAL HTTPS</p>
                                    <h1>Public certificate evidence.</h1>
                                    <p class="muted">
                                        Inspect names and expiry. Private keys
                                        and trust changes stay out of the
                                        dashboard.
                                    </p>
                                </div>
                            </div>
                            <section
                                v-if="!data.certificate"
                                class="panel empty"
                            >
                                <Icon name="certificates" :size="36" />
                                <h2>No local certificate recorded.</h2>
                                <p>
                                    Use the existing reviewed certificate
                                    workflow in your terminal.
                                </p>
                                <code>velnix tls status</code>
                            </section>
                            <section v-else class="panel">
                                <div class="panel-heading">
                                    <h2>Current local certificate</h2>
                                    <span class="badge">{{
                                        data.certificate.phase
                                    }}</span>
                                </div>
                                <dl class="environment">
                                    <div>
                                        <dt>Names</dt>
                                        <dd>
                                            <span
                                                v-for="name in data.certificate
                                                    .names"
                                                :key="name"
                                                class="badge"
                                                >{{ name }}</span
                                            >
                                        </dd>
                                    </div>
                                    <div>
                                        <dt>Expires</dt>
                                        <dd>{{ certificateDate }}</dd>
                                    </div>
                                    <div>
                                        <dt>CA SHA-256</dt>
                                        <dd class="path mono">
                                            {{ data.certificate.fingerprint }}
                                        </dd>
                                    </div>
                                </dl>
                                <p class="soft-note">
                                    Recorded evidence does not establish browser
                                    or system trust. Issuance and trust-store
                                    changes require the existing CLI workflow.
                                </p>
                            </section></template
                        >
                        <MailPanel
                            v-if="page === 'mail'"
                            :refresh="refresh"
                            @review="review"
                        />
                        <template v-if="page === 'diagnostics'"
                            ><div class="intro">
                                <div>
                                    <p class="eyebrow">READ-ONLY DIAGNOSTICS</p>
                                    <h1>A little clarity goes a long way.</h1>
                                    <p class="muted">
                                        Platform and configuration findings,
                                        without running providers or repairs.
                                    </p>
                                </div>
                            </div>
                            <section class="panel">
                                <div
                                    v-for="check in data.checks ?? []"
                                    :key="check.name"
                                    class="check-row"
                                >
                                    <span
                                        class="check-symbol"
                                        :class="check.status"
                                        ><Icon
                                            :name="
                                                check.status === 'ok'
                                                    ? 'check'
                                                    : 'diagnostics'
                                            "
                                    /></span>
                                    <div>
                                        <h3>{{ check.name }}</h3>
                                        <p class="muted">{{ check.detail }}</p>
                                    </div>
                                    <span class="badge">{{
                                        check.status
                                    }}</span>
                                </div>
                            </section>
                            <div class="soft-note">
                                <Icon name="lock" />
                                <p>
                                    Need to investigate further? Use
                                    <code>velnix doctor</code> or
                                    <code>velnix config validate</code>. This
                                    page never repairs state automatically.
                                </p>
                            </div></template
                        >
                    </template>
                </main>
                <footer class="workspace-footer">
                    <span>Keep building. Velnix keeps things local.</span
                    ><span>NATIVE · DAEMONLESS BY DEFAULT</span>
                </footer>
            </div>
            <PlanDialog
                v-if="plan"
                :plan="plan"
                :busy="applying"
                @close="plan = null"
                @apply="apply"
        /></template>
    </div>
</template>
