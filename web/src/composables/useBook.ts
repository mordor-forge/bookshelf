import { computed, onBeforeUnmount, onMounted, ref, shallowRef, type ComputedRef } from 'vue'
import {
  addBookmark as apiAddBookmark,
  addNote as apiAddNote,
  deleteBookmark as apiDeleteBookmark,
  deleteNote as apiDeleteNote,
  getBook,
  patchNote as apiPatchNote,
  putBookStatus,
  putProgress,
} from '../api'
import type { Book, Bookmark, Note, Status } from '../types'

const PROGRESS_DEBOUNCE_MS = 500

export function useBook(path: string) {
  const book = shallowRef<Book | null>(null)
  const bookmarks = ref<Bookmark[]>([])
  const notes = ref<Note[]>([])
  const currentPage = ref(1)
  const totalPages = ref(0)
  const loading = ref(true)
  const error = ref<string | null>(null)
  const status = ref<Status>('never_started')
  const currentlyReading = ref(false)

  let debounceHandle: ReturnType<typeof setTimeout> | null = null
  let pendingFlush: (() => Promise<void>) | null = null

  async function flushNow(): Promise<void> {
    if (debounceHandle !== null) {
      clearTimeout(debounceHandle)
      debounceHandle = null
    }
    if (pendingFlush) {
      const fn = pendingFlush
      pendingFlush = null
      await fn()
    }
  }

  function scheduleProgress(page: number, total: number): void {
    currentPage.value = page
    if (total > totalPages.value) {
      totalPages.value = total
    }
    if (debounceHandle !== null) {
      clearTimeout(debounceHandle)
    }
    pendingFlush = async () => {
      try {
        await putProgress(path, page, totalPages.value)
      } catch (err) {
        // silent — next debounce will retry on the next change.
        console.error('progress write failed', err)
      }
    }
    debounceHandle = setTimeout(() => {
      void flushNow()
    }, PROGRESS_DEBOUNCE_MS)
  }

  async function addBookmark(page: number, label?: string): Promise<void> {
    const bm = await apiAddBookmark(path, page, label && label.length > 0 ? label : undefined)
    // keep sorted by page asc, id asc.
    const next = [...bookmarks.value, bm].sort(
      (a, b) => a.page - b.page || a.id - b.id,
    )
    bookmarks.value = next
  }

  async function removeBookmark(id: number): Promise<void> {
    await apiDeleteBookmark(id)
    bookmarks.value = bookmarks.value.filter((b) => b.id !== id)
  }

  function sortNotes(list: Note[]): Note[] {
    return [...list].sort((a, b) => a.page - b.page || a.id - b.id)
  }

  async function addNote(
    page: number,
    body: string,
    x?: number,
    y?: number,
  ): Promise<void> {
    const n = await apiAddNote(path, page, body, x, y)
    notes.value = sortNotes([...notes.value, n])
  }

  async function updateNote(
    id: number,
    payload: {
      body?: string
      page?: number
      x?: number
      y?: number
      clearPosition?: boolean
    },
  ): Promise<void> {
    const prev = notes.value
    const idx = prev.findIndex((n) => n.id === id)
    if (idx !== -1) {
      const base = prev[idx]
      const optimistic: Note = {
        ...base,
        body: payload.body ?? base.body,
        page: payload.page ?? base.page,
        x: payload.clearPosition ? null : (payload.x ?? base.x),
        y: payload.clearPosition ? null : (payload.y ?? base.y),
      }
      notes.value = sortNotes(prev.map((n) => (n.id === id ? optimistic : n)))
    }
    try {
      const updated = await apiPatchNote(id, payload)
      notes.value = sortNotes(
        notes.value.map((n) => (n.id === id ? updated : n)),
      )
    } catch (err) {
      notes.value = prev
      throw err
    }
  }

  async function removeNote(id: number): Promise<void> {
    const prev = notes.value
    notes.value = notes.value.filter((n) => n.id !== id)
    try {
      await apiDeleteNote(id)
    } catch (err) {
      notes.value = prev
      throw err
    }
  }

  const notesByPage: ComputedRef<Map<number, Note[]>> = computed(() => {
    const map = new Map<number, Note[]>()
    for (const n of notes.value) {
      const arr = map.get(n.page)
      if (arr) arr.push(n)
      else map.set(n.page, [n])
    }
    return map
  })

  // fire-and-forget flush, used in unload handler where async cannot block.
  function flushBeacon(): void {
    if (!pendingFlush) return
    try {
      const data = JSON.stringify({
        currentPage: currentPage.value,
        totalPages: totalPages.value,
      })
      const blob = new Blob([data], { type: 'application/json' })
      const url = `/api/books/${path.split('/').map(encodeURIComponent).join('/')}/progress`
      // sendBeacon only does POST; fall back to a sync fetch keepalive PUT.
      void fetch(url, { method: 'PUT', body: blob, keepalive: true })
    } catch {
      // best effort.
    }
    pendingFlush = null
    if (debounceHandle !== null) {
      clearTimeout(debounceHandle)
      debounceHandle = null
    }
  }

  onMounted(async () => {
    try {
      const b = await getBook(path)
      book.value = b
      bookmarks.value = b.bookmarks
      notes.value = sortNotes(b.notes ?? [])
      if (b.progress) {
        if (b.progress.currentPage > 0) {
          currentPage.value = b.progress.currentPage
        }
        totalPages.value = b.progress.totalPages
        if (b.progress.status) status.value = b.progress.status
        if (typeof b.progress.currentlyReading === 'boolean') {
          currentlyReading.value = b.progress.currentlyReading
        }
      }
      if (b.status) status.value = b.status
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
    window.addEventListener('beforeunload', flushBeacon)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', flushBeacon)
    void flushNow()
  })

  async function setCurrentlyReading(value: boolean): Promise<void> {
    const prev = currentlyReading.value
    const prevStatus = status.value
    currentlyReading.value = value
    try {
      const progress = await putBookStatus(path, value)
      if (progress.status) status.value = progress.status
      if (typeof progress.currentlyReading === 'boolean') {
        currentlyReading.value = progress.currentlyReading
      }
    } catch (err) {
      currentlyReading.value = prev
      status.value = prevStatus
      throw err
    }
  }

  return {
    book,
    bookmarks,
    notes,
    notesByPage,
    currentPage,
    totalPages,
    loading,
    error,
    status,
    currentlyReading,
    scheduleProgress,
    addBookmark,
    removeBookmark,
    addNote,
    updateNote,
    removeNote,
    setCurrentlyReading,
    flushNow,
  }
}
