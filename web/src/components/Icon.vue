<script setup lang="ts">
import type { Component } from "vue";
import {
    Activity,
    ArrowRight,
    Check,
    CodeXml,
    Download,
    FolderOpen,
    LayoutDashboard,
    LockKeyhole,
    LogOut,
    Mail,
    Menu,
    Plus,
    RefreshCw,
    Search,
    Server,
    ShieldCheck,
    Sun,
    X,
} from "@lucide/vue";

withDefaults(defineProps<{ name: string; size?: number }>(), { size: 20 });

const icons: Record<string, Component> = {
    overview: LayoutDashboard,
    projects: FolderOpen,
    runtimes: CodeXml,
    services: Server,
    certificates: ShieldCheck,
    mail: Mail,
    diagnostics: Activity,
    plus: Plus,
    arrow: ArrowRight,
    refresh: RefreshCw,
    menu: Menu,
    close: X,
    logout: LogOut,
    sun: Sun,
    check: Check,
    search: Search,
    download: Download,
    lock: LockKeyhole,
};
</script>

<template>
    <component
        :is="icons[name] ?? LayoutDashboard"
        :size="size"
        :stroke-width="1.6"
        aria-hidden="true"
    />
</template>
