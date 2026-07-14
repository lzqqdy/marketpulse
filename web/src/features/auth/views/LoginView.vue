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
    <header class="auth-top">
      <RouterLink class="brand" to="/">MarketPulse</RouterLink>
      <button type="button" class="theme-btn" :aria-label="themeStore.mode === 'dark' ? '浅色' : '深色'" @click="themeStore.toggle">
        {{ themeStore.mode === 'dark' ? '浅色' : '深色' }}
      </button>
    </header>

    <section class="auth-card">
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
  min-height: calc(100vh - 36px);
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.auth-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-strong);
  text-decoration: none;
}

.theme-btn {
  border: 1px solid var(--line);
  background: var(--card);
  color: var(--muted);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  cursor: pointer;
}

.auth-card {
  width: min(100%, 420px);
  margin: 0 auto;
  padding: 22px 18px;
  background: var(--card);
  border-radius: 8px;
  text-align: left;
}

.auth-card h1 {
  margin: 0;
  font-size: 22px;
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

.field {
  display: grid;
  gap: 6px;
  font-size: 13px;
  color: var(--muted);
}

.field input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--line);
  background: var(--bg);
  color: var(--text);
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 15px;
}

.field input:focus {
  outline: 1px solid var(--coin);
}

.form-error {
  margin: 0;
  font-size: 12px;
  color: var(--warning);
}

.primary-btn {
  margin-top: 4px;
  border: 0;
  border-radius: 6px;
  background: var(--coin);
  color: #111;
  font-weight: 700;
  padding: 11px 14px;
  cursor: pointer;
}

.primary-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.back-link {
  display: inline-block;
  margin-top: 16px;
  font-size: 13px;
  color: var(--muted);
}
</style>
