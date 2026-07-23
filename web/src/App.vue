<script setup lang="ts">
import { RouterView } from 'vue-router'
import AlertToastHost from '@/features/alerts/AlertToastHost.vue'
import ChatDrawer from '@/features/ai/components/ChatDrawer.vue'
</script>

<template>
  <div class="app-shell">
    <RouterView />
    <AlertToastHost />
    <ChatDrawer />
  </div>
</template>
