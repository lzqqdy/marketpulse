import type { LoginResult, UpdateProfileInput, User } from '@/features/auth/types/user'

async function parseError(res: Response): Promise<string> {
  const body = await res.json().catch(() => ({}))
  return body?.error?.message ?? `HTTP ${res.status}`
}

export async function login(phone: string, password: string): Promise<LoginResult> {
  const res = await fetch('/api/v1/users/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ phone, password }),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function logout(token: string): Promise<void> {
  await fetch('/api/v1/users/logout', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
  })
}

export async function fetchMe(token: string): Promise<User> {
  const res = await fetch('/api/v1/users/me', {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function updateMe(token: string, body: UpdateProfileInput): Promise<User> {
  const res = await fetch('/api/v1/users/me', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}

export async function changePassword(
  token: string,
  oldPassword: string,
  newPassword: string,
): Promise<void> {
  const res = await fetch('/api/v1/users/me/password', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ oldPassword, newPassword }),
  })
  if (!res.ok) throw new Error(await parseError(res))
}

export async function uploadAvatar(token: string, file: File): Promise<User> {
  const body = new FormData()
  body.append('file', file)
  const res = await fetch('/api/v1/users/me/avatar', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body,
  })
  if (!res.ok) throw new Error(await parseError(res))
  return res.json()
}
