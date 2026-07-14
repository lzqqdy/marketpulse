/** Resize & compress an image for avatar upload (max edge / JPEG). */
export async function compressAvatar(file: File, maxEdge = 1280, quality = 0.86): Promise<File> {
  if (!file.type.startsWith('image/')) {
    return file
  }
  // Already small enough — keep original format
  if (file.size <= 900 * 1024) {
    return file
  }

  const bitmap = await createImageBitmap(file)
  try {
    const scale = Math.min(1, maxEdge / Math.max(bitmap.width, bitmap.height))
    const w = Math.max(1, Math.round(bitmap.width * scale))
    const h = Math.max(1, Math.round(bitmap.height * scale))

    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) return file
    ctx.drawImage(bitmap, 0, 0, w, h)

    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob(resolve, 'image/jpeg', quality)
    })
    if (!blob || blob.size >= file.size) {
      return file
    }
    const base = file.name.replace(/\.[^.]+$/, '') || 'avatar'
    return new File([blob], `${base}.jpg`, { type: 'image/jpeg', lastModified: Date.now() })
  } finally {
    bitmap.close()
  }
}
