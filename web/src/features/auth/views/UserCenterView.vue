<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AlertDeliveriesPanel from '@/features/alerts/AlertDeliveriesPanel.vue'
import AlertRulesPanel from '@/features/alerts/AlertRulesPanel.vue'
import { useAuthStore } from '@/features/auth/stores/auth'
import { compressAvatar } from '@/features/auth/utils/compressImage'
import AssetCenterPanel from '@/features/portfolio/AssetCenterPanel.vue'
import { useThemeStore } from '@/stores/theme'

type UserTab = 'profile' | 'alerts' | 'portfolio'

const TABS: { id: UserTab; label: string }[] = [
  { id: 'portfolio', label: '资产中心' },
  { id: 'alerts', label: '价格告警' },
  { id: 'profile', label: '账户资料' },
]

const TAB_IDS = new Set<UserTab>(TABS.map((t) => t.id))

const auth = useAuthStore()
const themeStore = useThemeStore()
const router = useRouter()
const route = useRoute()

const activeTab = computed<UserTab>(() => {
  const tab = String(route.params.tab || 'portfolio') as UserTab
  return TAB_IDS.has(tab) ? tab : 'portfolio'
})

function selectTab(tab: UserTab) {
  if (tab === activeTab.value) return
  void router.push({ name: 'user', params: { tab } })
}

const profileMsg = ref('')
const passwordMsg = ref('')
const profileError = ref('')
const passwordError = ref('')
const avatarError = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarUploading = ref(false)

const form = reactive({
  displayName: '',
  avatarUrl: '',
  email: '',
  wechatPushToken: '',
})

const pwd = reactive({
  oldPassword: '',
  newPassword: '',
  confirm: '',
})

const themeLabel = computed(() => (themeStore.mode === 'dark' ? '切换浅色模式' : '切换深色模式'))

onMounted(async () => {
  await auth.refreshMe()
  syncForm()
})

function syncForm() {
  const u = auth.user
  if (!u) return
  form.displayName = u.displayName ?? ''
  form.avatarUrl = u.avatarUrl ?? ''
  form.email = u.email ?? ''
  form.wechatPushToken = u.wechatPushToken ?? ''
}

function avatarSrc(url: string) {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('data:')) return url
  return url.startsWith('/') ? url : `/${url}`
}

function pickAvatar() {
  avatarInput.value?.click()
}

async function onAvatarSelected(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  avatarError.value = ''
  avatarUploading.value = true
  try {
    const compressed = await compressAvatar(file)
    const me = await auth.uploadAvatar(compressed)
    form.avatarUrl = me.avatarUrl ?? ''
    profileMsg.value = '头像已更新'
  } catch (e) {
    avatarError.value = e instanceof Error ? e.message : '头像上传失败'
  } finally {
    avatarUploading.value = false
  }
}

async function saveProfile() {
  profileMsg.value = ''
  profileError.value = ''
  try {
    await auth.updateProfile({
      displayName: form.displayName.trim(),
      avatarUrl: form.avatarUrl.trim(),
      email: form.email.trim(),
      wechatPushToken: form.wechatPushToken.trim(),
    })
    profileMsg.value = '资料已保存'
    syncForm()
  } catch (e) {
    profileError.value = e instanceof Error ? e.message : '保存失败'
  }
}

async function savePassword() {
  passwordMsg.value = ''
  passwordError.value = ''
  if (pwd.newPassword.length < 6) {
    passwordError.value = '新密码至少 6 位'
    return
  }
  if (pwd.newPassword !== pwd.confirm) {
    passwordError.value = '两次输入的新密码不一致'
    return
  }
  try {
    await auth.changePassword(pwd.oldPassword, pwd.newPassword)
    passwordMsg.value = '密码已更新'
    pwd.oldPassword = ''
    pwd.newPassword = ''
    pwd.confirm = ''
  } catch (e) {
    passwordError.value = e instanceof Error ? e.message : '修改失败'
  }
}

async function onLogout() {
  await auth.logout()
  await router.replace('/login')
}

function goHome() {
  void router.push('/')
}
</script>

<template>
  <div class="user-page">
    <header class="user-top">
      <div class="user-top-left">
        <RouterLink class="brand" to="/">MarketPulse</RouterLink>
        <span class="crumb">/ 用户中心</span>
      </div>
    </header>

    <div class="user-tabs" role="tablist" aria-label="用户中心分组">
      <button
        v-for="tab in TABS"
        :key="tab.id"
        type="button"
        class="user-tab"
        role="tab"
        :class="{ active: activeTab === tab.id }"
        :aria-selected="activeTab === tab.id"
        @click="selectTab(tab.id)"
      >
        {{ tab.label }}
      </button>
    </div>

    <div class="user-body">
      <template v-if="activeTab === 'profile'">
        <section class="user-card">
          <div class="card-head">
            <button
              type="button"
              class="avatar avatar-btn"
              :aria-label="avatarUploading ? '头像上传中' : '点击上传头像'"
              :disabled="avatarUploading || auth.loading"
              @click="pickAvatar"
            >
              <img v-if="form.avatarUrl" :src="avatarSrc(form.avatarUrl)" alt="" />
              <span v-else>{{ (form.displayName || auth.user?.phone || '?').slice(0, 1) }}</span>
              <span class="avatar-mask">{{ avatarUploading ? '上传中' : '更换' }}</span>
            </button>
            <div>
              <h1>{{ form.displayName || '未设置名称' }}</h1>
              <p class="phone">手机号 {{ auth.user?.phone }}（不可修改）</p>
              <p class="avatar-hint">点击头像上传（jpg / png / webp / gif）；大图会自动压缩，上限约 10MB</p>
              <p v-if="avatarError" class="form-error">{{ avatarError }}</p>
            </div>
            <input
              ref="avatarInput"
              class="avatar-file"
              type="file"
              accept="image/jpeg,image/png,image/webp,image/gif"
              @change="onAvatarSelected"
            />
          </div>

          <h2>基本信息</h2>
          <form class="form-grid" @submit.prevent="saveProfile">
            <label class="field">
              <span>名称</span>
              <input v-model="form.displayName" type="text" maxlength="64" placeholder="显示名称" />
            </label>
            <label class="field">
              <span>邮箱</span>
              <input v-model="form.email" type="email" maxlength="128" placeholder="name@example.com" />
            </label>
            <label class="field">
              <span>微信推送 Token</span>
              <input
                v-model="form.wechatPushToken"
                type="text"
                maxlength="256"
                placeholder="用于后续告警推送"
              />
            </label>
            <p v-if="profileError" class="form-error">{{ profileError }}</p>
            <p v-else-if="profileMsg" class="form-ok">{{ profileMsg }}</p>
            <button type="submit" class="primary-btn" :disabled="auth.loading">保存资料</button>
          </form>
        </section>

        <section class="user-card">
          <h2>修改密码</h2>
          <form class="form-grid" @submit.prevent="savePassword">
            <label class="field">
              <span>原密码</span>
              <input v-model="pwd.oldPassword" type="password" autocomplete="current-password" required />
            </label>
            <label class="field">
              <span>新密码</span>
              <input v-model="pwd.newPassword" type="password" autocomplete="new-password" required />
            </label>
            <label class="field">
              <span>确认新密码</span>
              <input v-model="pwd.confirm" type="password" autocomplete="new-password" required />
            </label>
            <p v-if="passwordError" class="form-error">{{ passwordError }}</p>
            <p v-else-if="passwordMsg" class="form-ok">{{ passwordMsg }}</p>
            <button type="submit" class="primary-btn" :disabled="auth.loading">更新密码</button>
          </form>
        </section>
      </template>

      <template v-else-if="activeTab === 'alerts'">
        <AlertRulesPanel />
        <AlertDeliveriesPanel />
      </template>

      <template v-else>
        <AssetCenterPanel />
      </template>
    </div>

    <aside class="user-dock" aria-label="页面工具">
      <div class="dock-rail">
        <button type="button" class="dock-btn" aria-label="返回首页" @click="goHome">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M4 10.5 12 4l8 6.5" />
            <path d="M7 10v9h10v-9" />
          </svg>
        </button>
        <button type="button" class="dock-btn" :aria-label="themeLabel" @click="themeStore.toggle">
          <svg v-if="themeStore.mode === 'dark'" viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2" />
            <path d="M12 20v2" />
            <path d="m4.93 4.93 1.41 1.41" />
            <path d="m17.66 17.66 1.41 1.41" />
            <path d="M2 12h2" />
            <path d="M20 12h2" />
            <path d="m6.34 17.66-1.41 1.41" />
            <path d="m19.07 4.93-1.41 1.41" />
          </svg>
          <svg v-else viewBox="0 0 24 24" aria-hidden="true">
            <path d="M20.4 14.4A7.6 7.6 0 0 1 9.6 3.6 8.4 8.4 0 1 0 20.4 14.4Z" />
          </svg>
        </button>
        <button type="button" class="dock-btn" aria-label="退出登录" @click="onLogout">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M10 7V5a1 1 0 0 1 1-1h8v16h-8a1 1 0 0 1-1-1v-2" />
            <path d="M14 12H4" />
            <path d="m7 9-3 3 3 3" />
          </svg>
        </button>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.user-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  text-align: left;
  padding-right: 8px;
  min-width: 0;
  max-width: 100%;
  box-sizing: border-box;
}

.user-top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.user-top-left {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.brand {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-strong);
  text-decoration: none;
}

.crumb {
  font-size: 13px;
  color: var(--muted);
}

.user-tabs {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  border-bottom: 1px solid var(--line);
  margin-bottom: 2px;
}

.user-tab {
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--text);
  padding: 8px 12px;
  font-size: 13px;
  cursor: pointer;
}

.user-tab.active {
  color: var(--coin);
  border-bottom-color: var(--coin);
  font-weight: 600;
}

.user-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
  max-width: 100%;
}

.user-card {
  background: var(--card);
  border-radius: 8px;
  padding: 16px;
  min-width: 0;
  max-width: 100%;
  box-sizing: border-box;
}

.card-head {
  display: flex;
  gap: 14px;
  align-items: center;
  margin-bottom: 18px;
}

.avatar {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background: var(--panel);
  border: 1px solid var(--line);
  display: grid;
  place-items: center;
  overflow: hidden;
  font-size: 22px;
  font-weight: 700;
  color: var(--coin);
  position: relative;
  flex-shrink: 0;
}

.avatar-btn {
  padding: 0;
  cursor: pointer;
}

.avatar-btn:disabled {
  cursor: wait;
  opacity: 0.85;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-mask {
  position: absolute;
  inset: auto 0 0;
  padding: 4px 0;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
  background: rgba(0, 0, 0, 0.55);
  opacity: 0;
  transition: opacity 0.15s ease;
}

.avatar-btn:hover .avatar-mask,
.avatar-btn:focus-visible .avatar-mask {
  opacity: 1;
}

.avatar-hint {
  margin: 6px 0 0;
  font-size: 11px;
  color: var(--muted-2);
}

.avatar-file {
  display: none;
}

.card-head h1 {
  margin: 0;
  font-size: 20px;
  color: var(--text-strong);
}

.phone {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted);
}

.user-card h2 {
  margin: 0 0 12px;
  font-size: 15px;
  color: var(--text-strong);
}

.form-grid {
  display: grid;
  gap: 12px;
}

@media (min-width: 720px) {
  .form-grid {
    grid-template-columns: 1fr 1fr;
  }

  .form-grid .primary-btn,
  .form-grid .form-error,
  .form-grid .form-ok {
    grid-column: 1 / -1;
  }
}

.field {
  display: grid;
  gap: 6px;
  font-size: 12px;
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
  font-size: 14px;
}

.field input:focus {
  outline: 1px solid var(--coin);
}

.form-error {
  margin: 0;
}

.form-ok {
  margin: 0;
}

.primary-btn {
  justify-self: start;
}

.primary-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.user-dock {
  position: fixed;
  top: 36vh;
  right: 0;
  z-index: 50;
  pointer-events: none;
}

.dock-rail {
  display: grid;
  width: 40px;
  border: 1px solid color-mix(in srgb, var(--line) 55%, transparent);
  border-right: 0;
  border-radius: 7px 0 0 7px;
  overflow: hidden;
  background: var(--dock-bg);
  box-shadow: 0 12px 28px var(--shadow);
  backdrop-filter: blur(8px);
  pointer-events: auto;
}

.dock-btn {
  display: grid;
  place-items: center;
  width: 40px;
  height: 44px;
  border: 0;
  border-bottom: 1px solid color-mix(in srgb, var(--line) 55%, transparent);
  background: var(--dock-btn);
  color: var(--dock-icon);
  cursor: pointer;
}

.dock-btn:last-child {
  border-bottom: 0;
}

.dock-btn:hover {
  background: var(--dock-btn-active);
  color: var(--text-strong);
}

.dock-btn svg {
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

@media (max-width: 680px) {
  .user-page {
    padding-right: 0;
    padding-bottom: 72px;
  }

  .user-dock {
    top: auto;
    bottom: 72px;
  }

  .user-tabs {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0;
  }

  .user-tab {
    flex: none;
    min-width: 0;
    padding: 10px 4px;
    font-size: 12px;
    text-align: center;
  }

  .card-head {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .avatar-mask {
    opacity: 1;
  }

  .avatar-hint {
    font-size: 12px;
  }

  .primary-btn {
    width: 100%;
    justify-self: stretch;
  }
}
</style>
