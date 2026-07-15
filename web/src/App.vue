<script setup lang="ts">
import { RouterView } from 'vue-router'
import AlertToastHost from '@/features/alerts/AlertToastHost.vue'
</script>

<template>
  <div class="app-shell">
    <RouterView />
    <AlertToastHost />
  </div>
</template>
