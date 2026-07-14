import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import * as authApi from '@/features/auth/api/auth'
import type { UpdateProfileInput, User } from '@/features/auth/types/user'

const TOKEN_KEY = 'marketpulse-auth-token'
const USER_KEY = 'marketpulse-auth-user'

function readStoredUser(): User | null {
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as User) : null
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) ?? '')
  const user = ref<User | null>(readStoredUser())
  const loading = ref(false)
  const error = ref('')

  const isLoggedIn = computed(() => !!token.value)

  function persist(nextToken: string, nextUser: User | null) {
    token.value = nextToken
    user.value = nextUser
    if (nextToken) localStorage.setItem(TOKEN_KEY, nextToken)
    else localStorage.removeItem(TOKEN_KEY)
    if (nextUser) localStorage.setItem(USER_KEY, JSON.stringify(nextUser))
    else localStorage.removeItem(USER_KEY)
  }

  async function login(phone: string, password: string) {
    loading.value = true
    error.value = ''
    try {
      const res = await authApi.login(phone, password)
      persist(res.token, res.user)
      return res.user
    } catch (e) {
      error.value = e instanceof Error ? e.message : '登录失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    const t = token.value
    persist('', null)
    if (t) {
      try {
        await authApi.logout(t)
      } catch {
        /* ignore */
      }
    }
  }

  async function refreshMe() {
    if (!token.value) return null
    loading.value = true
    error.value = ''
    try {
      const me = await authApi.fetchMe(token.value)
      persist(token.value, me)
      return me
    } catch (e) {
      persist('', null)
      error.value = e instanceof Error ? e.message : '会话已失效'
      return null
    } finally {
      loading.value = false
    }
  }

  async function updateProfile(input: UpdateProfileInput) {
    if (!token.value) throw new Error('未登录')
    loading.value = true
    error.value = ''
    try {
      const me = await authApi.updateMe(token.value, input)
      persist(token.value, me)
      return me
    } catch (e) {
      error.value = e instanceof Error ? e.message : '保存失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function changePassword(oldPassword: string, newPassword: string) {
    if (!token.value) throw new Error('未登录')
    loading.value = true
    error.value = ''
    try {
      await authApi.changePassword(token.value, oldPassword, newPassword)
    } catch (e) {
      error.value = e instanceof Error ? e.message : '修改密码失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function uploadAvatar(file: File) {
    if (!token.value) throw new Error('未登录')
    loading.value = true
    error.value = ''
    try {
      const me = await authApi.uploadAvatar(token.value, file)
      persist(token.value, me)
      return me
    } catch (e) {
      error.value = e instanceof Error ? e.message : '头像上传失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    token,
    user,
    loading,
    error,
    isLoggedIn,
    login,
    logout,
    refreshMe,
    updateProfile,
    changePassword,
    uploadAvatar,
  }
})
