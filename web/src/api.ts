import type {
  ApiErrorEnvelope,
  Book,
  Bookmark,
  Collection,
  Library,
  Note,
  Progress,
  ScanStatus,
  Settings,
} from './types'

export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function request<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, {
    ...init,
    headers: {
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...(init?.headers ?? {}),
    },
  })
  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  const body = text ? (JSON.parse(text) as unknown) : undefined
  if (!res.ok) {
    const env = body as ApiErrorEnvelope | undefined
    const code = env?.error?.code ?? 'unknown'
    const message = env?.error?.message ?? `HTTP ${res.status}`
    throw new ApiError(res.status, code, message)
  }
  return body as T
}

// encode each segment so slashes are preserved as routing separators.
function encodePath(path: string): string {
  return path.split('/').map(encodeURIComponent).join('/')
}

export function getLibrary(): Promise<Library> {
  return request<Library>('/api/library')
}

export function getBook(path: string): Promise<Book> {
  return request<Book>(`/api/books/${encodePath(path)}`)
}

export function putProgress(
  path: string,
  currentPage: number,
  totalPages: number,
): Promise<Progress> {
  return request<Progress>(`/api/books/${encodePath(path)}/progress`, {
    method: 'PUT',
    body: JSON.stringify({ currentPage, totalPages }),
  })
}

export function addBookmark(
  path: string,
  page: number,
  label?: string,
): Promise<Bookmark> {
  return request<Bookmark>(`/api/books/${encodePath(path)}/bookmarks`, {
    method: 'POST',
    body: JSON.stringify({ page, label: label ?? null }),
  })
}

export async function deleteBookmark(id: number): Promise<void> {
  await request<void>(`/api/bookmarks/${id}`, { method: 'DELETE' })
}

export function listNotes(path: string): Promise<Note[]> {
  return request<Note[]>(`/api/books/${encodePath(path)}/notes`)
}

export function addNote(
  path: string,
  page: number,
  body: string,
  x?: number,
  y?: number,
): Promise<Note> {
  const payload: { page: number; body: string; x?: number; y?: number } = {
    page,
    body,
  }
  if (x !== undefined && y !== undefined) {
    payload.x = x
    payload.y = y
  }
  return request<Note>(`/api/books/${encodePath(path)}/notes`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export interface PatchNotePayload {
  body?: string
  page?: number
  x?: number
  y?: number
  clearPosition?: boolean
}

export function patchNote(id: number, payload: PatchNotePayload): Promise<Note> {
  return request<Note>(`/api/notes/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  })
}

export async function deleteNote(id: number): Promise<void> {
  await request<void>(`/api/notes/${id}`, { method: 'DELETE' })
}

export async function postScan(): Promise<
  { started: boolean; scanId: string } | { conflict: true }
> {
  try {
    return await request<{ started: boolean; scanId: string }>('/api/scan', {
      method: 'POST',
    })
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      return { conflict: true }
    }
    throw err
  }
}

export function getScan(): Promise<ScanStatus> {
  return request<ScanStatus>('/api/scan')
}

export function bookFileUrl(path: string): string {
  return `/books/${encodePath(path)}`
}

export function getSettings(): Promise<Settings> {
  return request<Settings>('/api/settings')
}

export function putSettings(libraryDir: string): Promise<Settings> {
  return request<Settings>('/api/settings', {
    method: 'PUT',
    body: JSON.stringify({ libraryDir }),
  })
}

export function getCollections(): Promise<Collection[]> {
  return request<Collection[]>('/api/collections')
}

export function createCollection(
  name: string,
  parentId: number | null,
): Promise<Collection> {
  return request<Collection>('/api/collections', {
    method: 'POST',
    body: JSON.stringify({ name, parentId }),
  })
}

export function patchCollection(
  id: number,
  body: { name?: string; changeParent?: boolean; parentId?: number | null },
): Promise<Collection> {
  return request<Collection>(`/api/collections/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
}

export async function deleteCollection(id: number): Promise<void> {
  await request<void>(`/api/collections/${id}`, { method: 'DELETE' })
}

export async function addBookToCollection(
  id: number,
  path: string,
): Promise<void> {
  await request<void>(`/api/collections/${id}/books`, {
    method: 'POST',
    body: JSON.stringify({ path }),
  })
}

export async function removeBookFromCollection(
  id: number,
  path: string,
): Promise<void> {
  await request<void>(`/api/collections/${id}/books/${encodePath(path)}`, {
    method: 'DELETE',
  })
}

// uploadBook posts a multipart form to /api/upload. Uses XMLHttpRequest so
// callers can wire upload progress events; the optional `onProgress` callback
// receives a 0..1 fraction.
export function uploadBook(
  file: File,
  collectionIds?: number[],
  onProgress?: (fraction: number) => void,
): Promise<Book> {
  const form = new FormData()
  form.append('file', file)
  if (collectionIds) {
    for (const id of collectionIds) form.append('collectionIds', String(id))
  }
  return new Promise<Book>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/upload')
    if (onProgress) {
      xhr.upload.onprogress = (ev): void => {
        if (ev.lengthComputable) onProgress(ev.loaded / ev.total)
      }
    }
    xhr.onload = (): void => {
      const text = xhr.responseText
      let parsed: unknown
      try {
        parsed = text ? JSON.parse(text) : undefined
      } catch {
        reject(new ApiError(xhr.status, 'unknown', `HTTP ${xhr.status}`))
        return
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(parsed as Book)
        return
      }
      const env = parsed as ApiErrorEnvelope | undefined
      const code = env?.error?.code ?? 'unknown'
      const message = env?.error?.message ?? `HTTP ${xhr.status}`
      reject(new ApiError(xhr.status, code, message))
    }
    xhr.onerror = (): void => {
      reject(new ApiError(0, 'network', 'network error'))
    }
    xhr.send(form)
  })
}

export function putBookStatus(
  path: string,
  currentlyReading: boolean,
): Promise<Progress> {
  return request<Progress>(`/api/books/${encodePath(path)}/status`, {
    method: 'PUT',
    body: JSON.stringify({ currentlyReading }),
  })
}

export function putBookHidden(path: string, hidden: boolean): Promise<Book> {
  return request<Book>(`/api/books/${encodePath(path)}/hidden`, {
    method: 'PUT',
    body: JSON.stringify({ hidden }),
  })
}
