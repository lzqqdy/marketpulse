<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { RouterView } from 'vue-router'
import AlertToastHost from '@/features/alerts/AlertToastHost.vue'

const ChatDrawer = defineAsyncComponent(() => import('@/features/ai/components/ChatDrawer.vue'))
</script>

<template>
  <div class="app-shell">
    <RouterView />
    <AlertToastHost />
    <ChatDrawer />
  </div>
</template>
