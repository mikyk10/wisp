<!-- eslint-disable vue/max-attributes-per-line, vue/html-self-closing, vue/singleline-html-element-content-newline, vue/html-closing-bracket-spacing -->
<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 120 120"
    :width="size"
    :height="size"
    class="wisp-logo"
    aria-label="WiSP logo"
    role="img"
  >
    <defs>
      <!-- Outer frame bevel: light source top-left -->
      <linearGradient id="hl_top" x1="0" y1="0" x2="1" y2="0">
        <stop offset="0" stop-color="#7a8faa" stop-opacity="1.0"/>
        <stop offset="1" stop-color="#3a4a5e" stop-opacity="0.2"/>
      </linearGradient>
      <linearGradient id="hl_left" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#7a8faa" stop-opacity="1.0"/>
        <stop offset="1" stop-color="#3a4a5e" stop-opacity="0.2"/>
      </linearGradient>
      <linearGradient id="sh_right" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#0d1018" stop-opacity="0.4"/>
        <stop offset="1" stop-color="#050608" stop-opacity="1.0"/>
      </linearGradient>
      <linearGradient id="sh_bottom" x1="0" y1="0" x2="1" y2="0">
        <stop offset="0" stop-color="#0d1018" stop-opacity="0.4"/>
        <stop offset="1" stop-color="#050608" stop-opacity="1.0"/>
      </linearGradient>
      <!-- Display inner shadow: recessed screen -->
      <linearGradient id="inner_top" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#020304" stop-opacity="0.9"/>
        <stop offset="1" stop-color="#020304" stop-opacity="0.0"/>
      </linearGradient>
      <linearGradient id="inner_left" x1="0" y1="0" x2="1" y2="0">
        <stop offset="0" stop-color="#020304" stop-opacity="0.9"/>
        <stop offset="1" stop-color="#020304" stop-opacity="0.0"/>
      </linearGradient>
      <!-- Mat groove bevel: reversed direction (top-left is shadow = recessed feel) -->
      <linearGradient id="mat_top" x1="0" y1="0" x2="1" y2="0">
        <stop offset="0" stop-color="#030406" stop-opacity="0.9"/>
        <stop offset="1" stop-color="#030406" stop-opacity="0.2"/>
      </linearGradient>
      <linearGradient id="mat_left" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#030406" stop-opacity="0.9"/>
        <stop offset="1" stop-color="#030406" stop-opacity="0.2"/>
      </linearGradient>
      <linearGradient id="mat_right" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#3a4e62" stop-opacity="0.2"/>
        <stop offset="1" stop-color="#3a4e62" stop-opacity="0.7"/>
      </linearGradient>
      <linearGradient id="mat_bottom" x1="0" y1="0" x2="1" y2="0">
        <stop offset="0" stop-color="#3a4e62" stop-opacity="0.2"/>
        <stop offset="1" stop-color="#3a4e62" stop-opacity="0.7"/>
      </linearGradient>
      <!-- Clip path for dots -->
      <clipPath id="photo_clip">
        <rect x="25" y="25" width="70" height="70" rx="1"/>
      </clipPath>
    </defs>

    <!-- 1. Background -->
    <rect x="0" y="0" width="120" height="120" fill="#0d1117"/>
    <!-- 2. Drop shadow -->
    <rect x="3" y="3" width="117" height="117" rx="4" fill="#07090f"/>
    <!-- 3. Frame body -->
    <rect x="0" y="0" width="120" height="120" rx="4" fill="#222b3a"/>
    <!-- 4. Outer bevel edges (4px) -->
    <rect x="0" y="0" width="120" height="4" fill="url(#hl_top)"/>
    <rect x="0" y="0" width="4" height="120" fill="url(#hl_left)"/>
    <rect x="116" y="0" width="4" height="120" fill="url(#sh_right)"/>
    <rect x="0" y="116" width="120" height="4" fill="url(#sh_bottom)"/>
    <!-- 5. Display area -->
    <rect x="16" y="16" width="88" height="88" rx="2" fill="#0d1520"/>
    <!-- 6. Display inner shadow (10px fade) -->
    <rect x="16" y="16" width="88" height="10" fill="url(#inner_top)"/>
    <rect x="16" y="16" width="10" height="88" fill="url(#inner_left)"/>
    <!-- 7. Mat groove bevel (reversed, 3px wide) -->
    <rect x="20" y="20" width="80" height="3" fill="url(#mat_top)"/>
    <rect x="20" y="20" width="3" height="80" fill="url(#mat_left)"/>
    <rect x="97" y="20" width="3" height="80" fill="url(#mat_right)"/>
    <rect x="20" y="97" width="80" height="3" fill="url(#mat_bottom)"/>
    <!-- 8. Photo area -->
    <rect x="25" y="25" width="70" height="70" rx="1" fill="#090d14"/>
    <!-- 9. Dot group (W-shape, clipped by photo_clip) -->
    <g clip-path="url(#photo_clip)">
      <!-- Anchor dots (5 points): W skeleton -->
      <circle cx="30.1" cy="30.8" r="4.5" fill="#3ee8c0" opacity="0.98"/>
      <circle cx="88.6" cy="32.2" r="4.0" fill="#f5c800" opacity="0.95"/>
      <circle cx="58.9" cy="48.4" r="3.5" fill="#3ee8c0" opacity="0.82"/>
      <circle cx="41.2" cy="74.0" r="4.2" fill="#ff7043" opacity="0.90"/>
      <circle cx="81.2" cy="76.8" r="4.2" fill="#a855f7" opacity="0.88"/>
      <!-- Bridge dots (4 points): skeleton connectors -->
      <circle cx="38.6" cy="50.1" r="2.8" fill="#f03e6e" opacity="0.76"/>
      <circle cx="51.1" cy="62.9" r="2.6" fill="#ff7043" opacity="0.73"/>
      <circle cx="82.9" cy="52.7" r="2.8" fill="#f5c800" opacity="0.74"/>
      <circle cx="71.6" cy="62.3" r="2.6" fill="#a855f7" opacity="0.71"/>
      <!-- Noise dots (2 points): natural scatter -->
      <circle cx="57.4" cy="32.2" r="1.8" fill="#f5c800" opacity="0.58"/>
      <circle cx="71.7" cy="86.0" r="1.6" fill="#f03e6e" opacity="0.55"/>
    </g>
  </svg>
</template>

<script setup lang="ts">
interface Props {
  size?: number
}

withDefaults(defineProps<Props>(), {
  size: 36,
})
</script>

<style scoped>
.wisp-logo {
  display: block;
  flex-shrink: 0;
}
</style>
