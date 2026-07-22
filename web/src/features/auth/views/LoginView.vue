<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/features/auth/stores/auth'
import { useThemeStore } from '@/stores/theme'

const auth = useAuthStore()
const themeStore = useThemeStore()
const router = useRouter()
const route = useRoute()

const phone = ref('')
const password = ref('')
const submitting = ref(false)
const localError = ref('')

async function onSubmit() {
  localError.value = ''
  submitting.value = true
  try {
    await auth.login(phone.value.trim(), password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/user'
    await router.replace(redirect || '/user')
  } catch (e) {
    localError.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-glow" aria-hidden="true" />
    <header class="auth-top">
      <RouterLink class="brand" to="/">MarketPulse</RouterLink>
      <button type="button" class="ghost-btn theme-btn" :aria-label="themeStore.mode === 'dark' ? '浅色' : '深色'" @click="themeStore.toggle">
        {{ themeStore.mode === 'dark' ? '浅色' : '深色' }}
      </button>
    </header>

    <section class="auth-card">
      <p class="eyebrow">个人行情看板</p>
      <h1>登录</h1>
      <p class="hint">手机号 + 密码登录。账号由管理员录入，暂不开放注册。</p>

      <form class="auth-form" @submit.prevent="onSubmit">
        <label class="field">
          <span>手机号</span>
          <input v-model="phone" type="tel" inputmode="numeric" autocomplete="username" maxlength="11" placeholder="11 位手机号" required />
        </label>
        <label class="field">
          <span>密码</span>
          <input v-model="password" type="password" autocomplete="current-password" placeholder="密码" required />
        </label>
        <p v-if="localError || auth.error" class="form-error">{{ localError || auth.error }}</p>
        <button type="submit" class="primary-btn" :disabled="submitting || auth.loading">
          {{ submitting ? '登录中…' : '登录' }}
        </button>
      </form>

      <RouterLink class="back-link" to="/">返回首页</RouterLink>
    </section>
  </div>
</template>

<style scoped>
.auth-page {
  position: relative;
  min-height: calc(100vh - 36px);
  display: flex;
  flex-direction: column;
  gap: 28px;
  overflow: hidden;
}

.auth-glow {
  pointer-events: none;
  position: absolute;
  inset: -20% -10% auto;
  height: 55%;
  background:
    radial-gradient(ellipse 70% 60% at 20% 30%, color-mix(in srgb, var(--coin) 22%, transparent), transparent 70%),
    radial-gradient(ellipse 55% 50% at 85% 20%, color-mix(in srgb, var(--chart-2) 16%, transparent), transparent 65%);
  z-index: 0;
}

.auth-top,
.auth-card {
  position: relative;
  z-index: 1;
}

.auth-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-strong);
  text-decoration: none;
  letter-spacing: 0.02em;
}

.theme-btn {
  padding: 6px 10px;
  font-size: 12px;
  color: var(--muted);
}

.auth-card {
  width: min(100%, 420px);
  margin: 0 auto;
  padding: 24px 20px;
  background: color-mix(in srgb, var(--card) 92%, transparent);
  border: 1px solid color-mix(in srgb, var(--line) 85%, transparent);
  border-radius: var(--radius);
  box-shadow: 0 16px 40px var(--shadow);
  backdrop-filter: blur(10px);
}

.eyebrow {
  margin: 0 0 6px;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--coin);
}

.auth-card h1 {
  margin: 0;
  font-size: 24px;
  color: var(--text-strong);
}

.hint {
  margin: 8px 0 18px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.5;
}

.auth-form {
  display: grid;
  gap: 12px;
}

.primary-btn {
  width: 100%;
  margin-top: 4px;
  padding: 12px 14px;
  font-size: 15px;
}

.back-link {
  display: inline-block;
  margin-top: 16px;
  font-size: 13px;
  color: var(--muted);
  text-decoration: none;
}

.back-link:hover {
  color: var(--coin);
}
</style>
