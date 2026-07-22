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
      redirect: '/user/profile',
    },
    {
      path: '/user/:tab',
      name: 'user',
      component: () => import('@/features/auth/views/UserCenterView.vue'),
      meta: { requiresAuth: true },
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

const USER_TABS = new Set(['profile', 'alerts', 'portfolio'])

router.beforeEach(async (to) => {
  if (to.name === 'user') {
    const tab = String(to.params.tab || '')
    if (!USER_TABS.has(tab)) {
      return { name: 'user', params: { tab: 'profile' }, replace: true }
    }
  }
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
    return { name: 'user', params: { tab: 'profile' } }
  }
  return true
})

export default router
