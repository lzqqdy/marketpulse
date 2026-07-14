export interface User {
  id: number
  phone: string
  displayName: string
  avatarUrl: string
  email: string
  wechatPushToken: string
  createdAt: string
  updatedAt: string
}

export interface LoginResult {
  token: string
  expiresAt: string
  user: User
}

export interface UpdateProfileInput {
  displayName?: string
  avatarUrl?: string
  email?: string
  wechatPushToken?: string
}
