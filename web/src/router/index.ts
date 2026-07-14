import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/features/auth/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/features/market/views/MarketDashboard.vue'),
      meta: { public: true },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/features/auth/views/LoginView.vue'),
      meta: { public: true, guestOnly: true },
    },
    {
      path: '/user',
      name: 'user',
      component: () => import('@/features/auth/views/UserCenterView.vue'),
      meta: { requiresAuth: true },
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth) {
    if (!auth.isLoggedIn) {
      return { name: 'login', query: { redirect: to.fullPath } }
    }
    if (!auth.user) {
      const me = await auth.refreshMe()
      if (!me) return { name: 'login', query: { redirect: to.fullPath } }
    }
  }
  if (to.meta.guestOnly && auth.isLoggedIn) {
    return { name: 'user' }
  }
  return true
})

export default router
