<template>
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.6"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] || paths.grid" />
  </svg>
</template>
<script setup lang="ts">
defineProps<{ name: string }>();
const paths: Record<string, string> = {
  grid: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  network:
    "M12 3a3 3 0 1 0 0 6 3 3 0 0 0 0-6 M5 15a3 3 0 1 0 0 6 3 3 0 0 0 0-6 M19 15a3 3 0 1 0 0 6 3 3 0 0 0 0-6 M12 9v3 M5 15v-3h14v3",
  link: "M10 13a5 5 0 0 0 7 .4l3-3a5 5 0 0 0-7-7l-2 2 M14 11a5 5 0 0 0-7-.4l-3 3a5 5 0 0 0 7 7l2-2",
  pulse: "M2 12h5l3-8 4 16 3-8h5",
  rules: "M4 5h16 M4 12h16 M4 19h16 M8 3v4 M16 10v4 M10 17v4",
  search: "M10.5 3a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15 M16 16l5 5",
  arrow: "M5 12h14 M13 6l6 6-6 6",
  diagonal: "M6 18 18 6 M6 6h12v12",
  plus: "M12 5v14 M5 12h14",
  refresh:
    "M20 7v5h-5 M4 17v-5h5 M5.5 7a8 8 0 0 1 13-1L20 9 M4 15l1.5 3a8 8 0 0 0 13-1",
  chevron: "m8 5 7 7-7 7",
  down: "m6 9 6 6 6-6",
  close: "m6 6 12 12 M6 18 18 6",
  command:
    "M9 9V6a3 3 0 1 0-3 3h12a3 3 0 1 0-3-3v12a3 3 0 1 0 3-3H6a3 3 0 1 0 3 3V9z",
  settings: "M4 7h16 M4 17h16 M8 4v6 M16 14v6",
  moon: "M20.5 13A9 9 0 0 1 11 3.5 9 9 0 1 0 20.5 13z",
  sun: "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1.5 1.5 M17.5 17.5 19 19 M5 19l1.5-1.5 M17.5 6.5 19 5",
  logout: "M9 4H4v16h5 M10 12h11 M17 8l4 4-4 4",
  play: "m9 5 11 7-11 7z",
  pause: "M9 5v14 M15 5v14",
  globe:
    "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20 M2 12h20 M12 2c-6 5-6 15 0 20 M12 2c6 5 6 15 0 20",
  clock: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 7v5l4 2",
  check: "m5 12 4 4L19 6",
  menu: "M4 6h16 M4 12h16 M4 18h16",
};
</script>
