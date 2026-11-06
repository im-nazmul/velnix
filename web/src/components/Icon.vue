<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: number }>(), { size: 20 });
const paths: Record<string, string> = {
  overview: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  projects: "M3 7h6l2-3h5l3 3h2v13H3z M3 10h18",
  runtimes: "m8 5-6 7 6 7 M16 5l6 7-6 7 M14 3l-4 18",
  services: "M4 3h16v7H4z M4 14h16v7H4z M7 6h1 M7 17h1 M16 6h1 M16 17h1",
  certificates: "M12 3 3 7v6c0 4 9 8 9 8s9-4 9-8V7z m-5 9 3 3 5-6",
  mail: "M3 5h18v14H3z m0 0 9 8 9-8",
  diagnostics: "M3 12h4l3-8 4 16 3-8h4",
  plus: "M12 5v14 M5 12h14",
  arrow: "M5 12h14 m-6-6 6 6-6 6",
  refresh: "M20 7a8 8 0 1 0 1 8 M20 3v5h-5",
  menu: "M4 6h16 M4 12h16 M4 18h16",
  close: "m6 6 12 12 M6 18 18 6",
  logout: "M10 3H4v18h6 M9 12h12 m-5-5 5 5-5 5",
  sun: "M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10 M12 1v2 M12 21v2 M1 12h2 M21 12h2 M4 4l2 2 M18 18l2 2 M4 20l2-2 M18 6l2-2",
  check: "m5 12 4 4L19 6",
  search: "M10 3a7 7 0 1 0 0 14 7 7 0 0 0 0-14 m5 12 6 6",
  download: "M12 3v12 m-5-5 5 5 5-5 M4 16v5h16v-5",
  lock: "M6 10h12v11H6z M8 10V6a4 4 0 0 1 8 0v4",
};
</script>
<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.6"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] ?? paths.overview" />
  </svg>
</template>
