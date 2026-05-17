<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as pdfjsLib from 'pdfjs-dist'
import PdfWorker from 'pdfjs-dist/build/pdf.worker.mjs?worker'
import type { PDFDocumentProxy, PDFPageProxy } from 'pdfjs-dist'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  BookOpen,
  BookOpenCheck,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Columns2,
  Maximize2,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRightClose,
  PanelRightOpen,
  RectangleVertical,
  StickyNote,
  X,
  ZoomIn,
  ZoomOut,
} from 'lucide-vue-next'
import { bookFileUrl } from '../api'
import { useBook } from '../composables/useBook'
import BookmarkPanel from '../components/BookmarkPanel.vue'
import NoteDialog from '../components/NoteDialog.vue'
import PdfOutline, { type OutlineNode } from '../components/PdfOutline.vue'
import type { Note } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

pdfjsLib.GlobalWorkerOptions.workerPort = new PdfWorker()

const props = defineProps<{ path: string }>()
const bookPath = props.path

const {
  book,
  bookmarks,
  notes,
  notesByPage,
  currentPage,
  totalPages,
  loading,
  error,
  status,
  scheduleProgress,
  addBookmark,
  removeBookmark,
  addNote,
  updateNote,
  removeNote,
  setCurrentlyReading,
} = useBook(bookPath)

const noteDialogOpen = ref(false)
const noteDialogExisting = ref<Note | null>(null)
const noteDialogPage = ref(1)
const noteDialogX = ref<number | null>(null)
const noteDialogY = ref<number | null>(null)
const hoveredNoteId = ref<number | null>(null)
let hoverCloseHandle: ReturnType<typeof setTimeout> | null = null

const noteCursorActive = ref(false)

function openCreateNote(page: number): void {
  noteDialogExisting.value = null
  noteDialogPage.value = page
  noteDialogX.value = null
  noteDialogY.value = null
  noteDialogOpen.value = true
}

function openCreateNoteAt(page: number, x: number, y: number): void {
  noteDialogExisting.value = null
  noteDialogPage.value = page
  noteDialogX.value = x
  noteDialogY.value = y
  noteDialogOpen.value = true
}

function openEditNote(n: Note): void {
  noteDialogExisting.value = n
  noteDialogPage.value = n.page
  noteDialogX.value = n.x ?? null
  noteDialogY.value = n.y ?? null
  noteDialogOpen.value = true
}

async function onNoteSave(payload: {
  id: number | null
  page: number
  body: string
  x: number | null
  y: number | null
  clearPosition: boolean
}): Promise<void> {
  try {
    if (payload.id === null) {
      await addNote(
        payload.page,
        payload.body,
        payload.x ?? undefined,
        payload.y ?? undefined,
      )
    } else {
      const existing = noteDialogExisting.value
      const patch: {
        body?: string
        page?: number
        x?: number
        y?: number
        clearPosition?: boolean
      } = { body: payload.body }
      if (!existing || existing.page !== payload.page) patch.page = payload.page
      if (payload.clearPosition) {
        patch.clearPosition = true
      } else if (payload.x !== null && payload.y !== null) {
        const xChanged = !existing || existing.x !== payload.x
        const yChanged = !existing || existing.y !== payload.y
        if (xChanged || yChanged) {
          patch.x = payload.x
          patch.y = payload.y
        }
      }
      await updateNote(payload.id, patch)
    }
    noteDialogOpen.value = false
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to save note')
  }
}

function toggleNoteCursor(): void {
  noteCursorActive.value = !noteCursorActive.value
}

function onPageWrapperPointerDown(event: PointerEvent, pageNum: number): void {
  if (!noteCursorActive.value) return
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  const rect = target.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return
  const x = (event.clientX - rect.left) / rect.width
  const y = (event.clientY - rect.top) / rect.height
  if (x < 0 || x > 1 || y < 0 || y > 1) return
  event.preventDefault()
  event.stopPropagation()
  noteCursorActive.value = false
  openCreateNoteAt(pageNum, Math.max(0, Math.min(1, x)), Math.max(0, Math.min(1, y)))
}

function onReaderKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && noteCursorActive.value) {
    noteCursorActive.value = false
  }
}

async function onNoteDelete(id: number): Promise<void> {
  try {
    await removeNote(id)
    noteDialogOpen.value = false
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to delete note')
  }
}

function showNoteHover(id: number): void {
  if (hoverCloseHandle !== null) {
    clearTimeout(hoverCloseHandle)
    hoverCloseHandle = null
  }
  hoveredNoteId.value = id
}

function hideNoteHover(): void {
  if (hoverCloseHandle !== null) clearTimeout(hoverCloseHandle)
  hoverCloseHandle = setTimeout(() => {
    hoveredNoteId.value = null
    hoverCloseHandle = null
  }, 200)
}

function notesFor(pageNum: number): Note[] {
  return notesByPage.value.get(pageNum) ?? []
}

function isPositioned(n: Note): boolean {
  return n.x !== null && n.x !== undefined && n.y !== null && n.y !== undefined
}

function gutterNotesFor(pageNum: number): Note[] {
  return notesFor(pageNum).filter((n) => !isPositioned(n))
}

function positionedNotesFor(pageNum: number): Note[] {
  return notesFor(pageNum).filter(isPositioned)
}

async function removeNoteFromPanel(id: number): Promise<void> {
  try {
    await removeNote(id)
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to delete note')
  }
}

const pdfDoc = shallowRef<PDFDocumentProxy | null>(null)
const pageInputWidth = computed<string>(() => {
  const n = pdfDoc.value?.numPages ?? totalPages.value ?? 1
  const digits = Math.max(1, String(Math.max(1, n)).length)
  // digits + ample padding for input chrome + number-spinner buttons.
  return `${digits + 8}ch`
})
const scrollEl = ref<HTMLDivElement | null>(null)
const pageWrappers = ref<HTMLDivElement[]>([])
const renderError = ref<string | null>(null)
const pdfLoading = ref(true)
const pageInput = ref(1)
const outline = ref<OutlineNode[]>([])
const outlineLoaded = ref(false)

const TOC_OPEN_KEY = 'bookshelf:reader-toc-open'
const BOOKMARKS_OPEN_KEY = 'bookshelf:reader-bookmarks-open'
const ZOOM_KEY = 'bookshelf:zoom'

const tocOpen = ref(false)
const bookmarksOpen = ref(false)
try {
  tocOpen.value = localStorage.getItem(TOC_OPEN_KEY) === 'true'
  bookmarksOpen.value = localStorage.getItem(BOOKMARKS_OPEN_KEY) === 'true'
} catch {
  // ignore
}
watch(tocOpen, (v) => {
  try { localStorage.setItem(TOC_OPEN_KEY, String(v)) } catch { /* ignore */ }
})
watch(bookmarksOpen, (v) => {
  try { localStorage.setItem(BOOKMARKS_OPEN_KEY, String(v)) } catch { /* ignore */ }
})

const ZOOM_MIN = 0.5
const ZOOM_MAX = 3.0
const ZOOM_STEP = 0.1
function clampZoom(z: number): number {
  if (!Number.isFinite(z)) return 1
  return Math.max(ZOOM_MIN, Math.min(ZOOM_MAX, Math.round(z * 100) / 100))
}
const zoom = ref(1)
try {
  const raw = localStorage.getItem(ZOOM_KEY)
  if (raw !== null) zoom.value = clampZoom(Number.parseFloat(raw))
} catch {
  // ignore
}
const zoomPercent = computed<number>({
  get: () => Math.round(zoom.value * 100),
  set: (v) => {
    const n = Number(v)
    if (!Number.isFinite(n)) return
    zoom.value = clampZoom(n / 100)
  },
})

const TWO_PAGE_KEY = 'bookshelf:two-page'
const twoPageStored = ref<boolean>(false)
try {
  twoPageStored.value = localStorage.getItem(TWO_PAGE_KEY) === 'true'
} catch {
  // ignore
}

// viewport-aware: only enable two-page above md breakpoint (768px).
const wideEnough = ref(window.innerWidth >= 768)
function onResize(): void {
  wideEnough.value = window.innerWidth >= 768
}
window.addEventListener('resize', onResize)

const twoPage = computed(() => twoPageStored.value && wideEnough.value)

function toggleTwoPage(): void {
  twoPageStored.value = !twoPageStored.value
  try {
    localStorage.setItem(TWO_PAGE_KEY, String(twoPageStored.value))
  } catch {
    // ignore
  }
}

interface PageMeta {
  baseWidth: number
  baseHeight: number
  width: number
  height: number
  rendered: boolean
  rendering: boolean
  renderedZoom: number
  task: { cancel: () => void } | null
}
const pages = ref<PageMeta[]>([])

const MAX_RENDERED = 6
const BASE_SCALE = 1.25
const renderedOrder: number[] = []

let visibilityObserver: IntersectionObserver | null = null
let renderObserver: IntersectionObserver | null = null
let suppressScrollPageUpdate = false
let initialPageApplied = false

async function loadPageDimensions(
  doc: PDFDocumentProxy,
  from: number,
  to: number,
  z: number,
): Promise<void> {
  const concurrency = 8
  let next = from
  async function worker(): Promise<void> {
    while (true) {
      const i = next++
      if (i > to) return
      try {
        const p = await doc.getPage(i)
        const vp = p.getViewport({ scale: BASE_SCALE })
        const arr = pages.value
        const cur = arr[i - 1]
        if (!cur) continue
        arr[i - 1] = {
          ...cur,
          baseWidth: vp.width,
          baseHeight: vp.height,
          width: vp.width * z,
          height: vp.height * z,
        }
      } catch {
        // ignore
      }
    }
  }
  const workers: Promise<void>[] = []
  for (let i = 0; i < concurrency; i++) workers.push(worker())
  await Promise.all(workers)
}

async function loadPdf(): Promise<void> {
  pdfLoading.value = true
  renderError.value = null
  pageWrappers.value = []
  pages.value = []
  outline.value = []
  outlineLoaded.value = false
  try {
    const task = pdfjsLib.getDocument({ url: bookFileUrl(bookPath) })
    const doc = await task.promise
    pdfDoc.value = doc
    totalPages.value = Math.max(totalPages.value, doc.numPages)

    const first = await doc.getPage(1)
    const fv = first.getViewport({ scale: BASE_SCALE })
    const z = zoom.value
    const defaultMeta = (): PageMeta => ({
      baseWidth: fv.width,
      baseHeight: fv.height,
      width: fv.width * z,
      height: fv.height * z,
      rendered: false,
      rendering: false,
      renderedZoom: 0,
      task: null,
    })
    pages.value = Array.from({ length: doc.numPages }, defaultMeta)

    if (!initialPageApplied) {
      initialPageApplied = true
      if (currentPage.value < 1) currentPage.value = 1
      if (currentPage.value > doc.numPages) currentPage.value = doc.numPages
    }
    pageInput.value = currentPage.value

    void loadOutline(doc)

    // fetch real per-page viewports in parallel batches so wrapper heights are
    // correct quickly; otherwise scroll-to-page for large books lands on the
    // wrong offset as pages later re-size and shift layout.
    const target = currentPage.value
    const numPages = doc.numPages

    // prioritize pages up to and around the target so the initial scroll is
    // accurate the first time. Then fill in the rest in the background.
    const priorityEnd = Math.min(numPages, Math.max(target + 20, 40))
    await loadPageDimensions(doc, 2, priorityEnd, zoom.value)

    await nextTick()
    setupObservers()
    if (currentPage.value > 1) {
      scrollToPage(currentPage.value)
    }

    void (async () => {
      if (priorityEnd >= numPages) return
      await loadPageDimensions(doc, priorityEnd + 1, numPages, zoom.value)
      // layout has shifted; re-anchor to the user's current page.
      scrollToPage(currentPage.value)
    })()
  } catch (err) {
    renderError.value = err instanceof Error ? err.message : String(err)
  } finally {
    pdfLoading.value = false
  }
}

async function loadOutline(doc: PDFDocumentProxy): Promise<void> {
  try {
    const raw = await doc.getOutline()
    if (!raw) {
      outline.value = []
      outlineLoaded.value = true
      return
    }
    async function resolveDest(dest: unknown): Promise<number | null> {
      try {
        let resolved: unknown = dest
        if (typeof resolved === 'string') {
          resolved = await doc.getDestination(resolved)
        }
        if (!Array.isArray(resolved) || resolved.length === 0) return null
        const ref = resolved[0]
        if (!ref) return null
        const idx = await doc.getPageIndex(ref as never)
        return idx + 1
      } catch {
        return null
      }
    }
    interface RawItem {
      title: string
      dest?: unknown
      items?: RawItem[]
    }
    async function walk(items: RawItem[]): Promise<OutlineNode[]> {
      const out: OutlineNode[] = []
      for (const it of items) {
        const page = it.dest === undefined || it.dest === null
          ? null
          : await resolveDest(it.dest)
        const children = it.items ? await walk(it.items) : []
        out.push({ title: it.title, page, children })
      }
      return out
    }
    outline.value = await walk(raw as RawItem[])
  } catch {
    outline.value = []
  } finally {
    outlineLoaded.value = true
  }
}

function pageIndex(el: Element): number | null {
  const attr = (el as HTMLElement).dataset.pageNumber
  if (!attr) return null
  const n = Number.parseInt(attr, 10)
  return Number.isFinite(n) ? n : null
}

function setupObservers(): void {
  teardownObservers()
  const root = scrollEl.value
  if (!root) return

  visibilityObserver = new IntersectionObserver(
    () => {
      if (suppressScrollPageUpdate) return
      updateCurrentFromScroll()
    },
    { root, threshold: [0, 0.01, 0.5, 1] },
  )

  renderObserver = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) continue
        const n = pageIndex(e.target)
        if (n !== null) void ensureRendered(n)
      }
    },
    { root, rootMargin: '600px 0px 600px 0px' },
  )

  for (const w of pageWrappers.value) {
    if (!w) continue
    visibilityObserver.observe(w)
    renderObserver.observe(w)
  }

  root.addEventListener('scroll', onScroll, { passive: true })
}

function teardownObservers(): void {
  visibilityObserver?.disconnect()
  renderObserver?.disconnect()
  visibilityObserver = null
  renderObserver = null
  scrollEl.value?.removeEventListener('scroll', onScroll)
}

let scrollRaf = 0
function onScroll(): void {
  if (suppressScrollPageUpdate) return
  if (scrollRaf !== 0) return
  scrollRaf = window.requestAnimationFrame(() => {
    scrollRaf = 0
    updateCurrentFromScroll()
  })
}

function updateCurrentFromScroll(): void {
  const root = scrollEl.value
  if (!root) return
  const top = root.getBoundingClientRect().top
  let bestPage = currentPage.value
  let bestDist = Number.POSITIVE_INFINITY
  for (const w of pageWrappers.value) {
    if (!w) continue
    const n = pageIndex(w)
    if (n === null) continue
    const d = Math.abs(w.getBoundingClientRect().top - top)
    if (d < bestDist) {
      bestDist = d
      bestPage = n
    }
  }
  if (bestPage !== currentPage.value) {
    pageInput.value = bestPage
    const total = pdfDoc.value?.numPages ?? totalPages.value
    scheduleProgress(bestPage, total)
  }
}

async function ensureRendered(pageNum: number): Promise<void> {
  const doc = pdfDoc.value
  if (!doc) return
  const meta = pages.value[pageNum - 1]
  if (!meta || meta.rendered || meta.rendering) return
  const wrapper = pageWrappers.value[pageNum - 1]
  if (!wrapper) return
  const canvas = wrapper.querySelector('canvas') as HTMLCanvasElement | null
  if (!canvas) return

  meta.rendering = true
  let page: PDFPageProxy
  try {
    page = await doc.getPage(pageNum)
  } catch (err) {
    meta.rendering = false
    renderError.value = err instanceof Error ? err.message : String(err)
    return
  }
  const dpr = window.devicePixelRatio || 1
  const effective = BASE_SCALE * zoom.value
  const viewport = page.getViewport({ scale: effective * dpr })
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    meta.rendering = false
    return
  }
  canvas.width = viewport.width
  canvas.height = viewport.height
  canvas.style.width = `${viewport.width / dpr}px`
  canvas.style.height = `${viewport.height / dpr}px`

  const task = page.render({ canvasContext: ctx, viewport })
  meta.task = task
  try {
    await task.promise
    meta.rendered = true
    meta.renderedZoom = zoom.value
    renderedOrder.push(pageNum)
    evictIfNeeded(pageNum)
  } catch {
    // cancelled.
  } finally {
    meta.rendering = false
    meta.task = null
  }
}

function evictIfNeeded(keep: number): void {
  while (renderedOrder.length > MAX_RENDERED) {
    let idx = -1
    for (let i = 0; i < renderedOrder.length; i++) {
      const p = renderedOrder[i]
      if (Math.abs(p - keep) > 2) {
        idx = i
        break
      }
    }
    if (idx === -1) idx = 0
    const victim = renderedOrder.splice(idx, 1)[0]
    const meta = pages.value[victim - 1]
    if (!meta) continue
    if (meta.task) {
      meta.task.cancel()
      meta.task = null
    }
    const wrapper = pageWrappers.value[victim - 1]
    const canvas = wrapper?.querySelector('canvas') as HTMLCanvasElement | null
    if (canvas) {
      const ctx = canvas.getContext('2d')
      ctx?.clearRect(0, 0, canvas.width, canvas.height)
      canvas.width = 0
      canvas.height = 0
    }
    meta.rendered = false
  }
}

function scrollToPage(pageNum: number): void {
  const doc = pdfDoc.value
  if (!doc) return
  const clamped = Math.max(1, Math.min(doc.numPages, Math.floor(pageNum)))
  const wrapper = pageWrappers.value[clamped - 1]
  const root = scrollEl.value
  if (!wrapper || !root) return

  suppressScrollPageUpdate = true
  wrapper.scrollIntoView({ behavior: 'auto', block: 'start' })
  currentPage.value = clamped
  pageInput.value = clamped
  scheduleProgress(clamped, doc.numPages)
  void ensureRendered(clamped)
  if (twoPage.value && clamped + 1 <= doc.numPages) {
    void ensureRendered(clamped + 1)
  }

  window.setTimeout(() => {
    suppressScrollPageUpdate = false
  }, 80)
}

function goTo(page: number): void {
  scrollToPage(page)
}
function goToInstant(page: number): void {
  scrollToPage(page)
}
function next(): void {
  goTo(currentPage.value + (twoPage.value ? 2 : 1))
}
function prev(): void {
  goTo(currentPage.value - (twoPage.value ? 2 : 1))
}
function goFirst(): void {
  goTo(1)
}
function goLast(): void {
  const total = pdfDoc.value?.numPages ?? totalPages.value
  if (total > 0) goTo(total)
}
function onPageInput(): void {
  goTo(pageInput.value)
}

function zoomIn(): void {
  zoom.value = clampZoom(zoom.value + ZOOM_STEP)
}
function zoomOut(): void {
  zoom.value = clampZoom(zoom.value - ZOOM_STEP)
}
function zoomReset(): void {
  zoom.value = 1
}

const isReading = computed(() => status.value === 'currently_reading')
async function toggleReading(): Promise<void> {
  try {
    await setCurrentlyReading(!isReading.value)
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to update status')
  }
}

watch(
  () => bookPath,
  () => {
    initialPageApplied = false
    void loadPdf()
  },
  { immediate: true },
)

watch(currentPage, (v) => {
  if (v !== pageInput.value) pageInput.value = v
})

watch(zoom, (z) => {
  try { localStorage.setItem(ZOOM_KEY, String(z)) } catch { /* ignore */ }
  // replace the array to force a clean reactive update of every wrapper's size.
  pages.value = pages.value.map((m) => ({
    ...m,
    width: m.baseWidth * z,
    height: m.baseHeight * z,
    rendered: false,
    rendering: false,
    renderedZoom: 0,
    task: null,
  }))
  // cancel any in-flight render tasks from the old meta.
  // (the new meta has task=null; the old tasks dangle but pdfjs will write
  // into the canvases we're about to clear, which we then overwrite.)
  for (const w of pageWrappers.value) {
    if (!w) continue
    const canvas = w.querySelector('canvas') as HTMLCanvasElement | null
    if (!canvas) continue
    canvas.width = 0
    canvas.height = 0
    canvas.style.width = ''
    canvas.style.height = ''
  }
  renderedOrder.length = 0

  const keepPage = currentPage.value
  void nextTick(() => {
    scrollToPage(keepPage)
    const doc = pdfDoc.value
    if (!doc) return
    const radius = 4
    const start = Math.max(1, keepPage - radius)
    const end = Math.min(doc.numPages, keepPage + radius)
    for (let p = start; p <= end; p++) void ensureRendered(p)
    setupObservers()
  })
})

onBeforeUnmount(() => {
  teardownObservers()
  window.removeEventListener('resize', onResize)
  for (const m of pages.value) {
    m.task?.cancel()
  }
  void pdfDoc.value?.destroy()
})

function setPageWrapper(el: Element | null, idx: number): void {
  if (el instanceof HTMLDivElement) {
    pageWrappers.value[idx] = el
  }
}

const paginatorActive = ref(true)
let paginatorIdleHandle: ReturnType<typeof setTimeout> | null = null

function bumpPaginator(): void {
  paginatorActive.value = true
  if (paginatorIdleHandle !== null) clearTimeout(paginatorIdleHandle)
  paginatorIdleHandle = setTimeout(() => {
    paginatorActive.value = false
    paginatorIdleHandle = null
  }, 2500)
}

onMounted(() => {
  bumpPaginator()
  window.addEventListener('keydown', onReaderKeydown)
})

onBeforeUnmount(() => {
  if (paginatorIdleHandle !== null) clearTimeout(paginatorIdleHandle)
  window.removeEventListener('keydown', onReaderKeydown)
})
</script>

<template>
  <Teleport defer to="#header-actions-left">
    <Button as-child variant="ghost" size="sm" aria-label="Back to library" title="Back to library">
      <router-link to="/">
        <ArrowLeft class="size-4 mr-1" /> <span class="hidden md:inline">Library</span>
      </router-link>
    </Button>
    <Button
      variant="ghost"
      size="icon"
      class="h-9 w-9"
      :aria-label="tocOpen ? 'Hide contents panel' : 'Show contents panel'"
      :title="tocOpen ? 'Hide contents panel' : 'Show contents panel'"
      @click="tocOpen = !tocOpen"
    >
      <PanelLeftClose v-if="tocOpen" class="size-4" />
      <PanelLeftOpen v-else class="size-4" />
    </Button>
    <span
      v-if="book"
      class="hidden lg:inline text-sm text-muted-foreground truncate max-w-[24rem] ml-1"
    >
      {{ book.title }}
    </span>
  </Teleport>

  <Teleport defer to="#header-actions-right">
    <div class="flex items-center gap-1 overflow-x-auto">
      <Button
        variant="outline"
        size="icon"
        class="h-9 w-9 hidden md:inline-flex"
        :aria-label="twoPage ? 'Switch to single page' : 'Switch to two-page'"
        :title="twoPage ? 'Switch to single page' : 'Switch to two-page'"
        @click="toggleTwoPage"
      >
        <Columns2 v-if="twoPage" class="size-4" />
        <RectangleVertical v-else class="size-4" />
      </Button>

      <Button
        variant="outline"
        size="icon"
        class="h-9 w-9"
        aria-label="Zoom out"
        title="Zoom out"
        :disabled="zoom <= 0.5"
        @click="zoomOut"
      >
        <ZoomOut class="size-4" />
      </Button>
      <Input
        v-model.number="zoomPercent"
        type="number"
        min="50"
        max="300"
        step="10"
        class="w-16 h-9 text-center hidden md:inline-flex"
        aria-label="Zoom percentage"
        title="Zoom percentage"
      />
      <Button
        variant="outline"
        size="icon"
        class="h-9 w-9"
        aria-label="Zoom in"
        title="Zoom in"
        :disabled="zoom >= 3"
        @click="zoomIn"
      >
        <ZoomIn class="size-4" />
      </Button>
      <Button
        variant="outline"
        size="icon"
        class="h-9 w-9 hidden md:inline-flex"
        aria-label="Reset zoom"
        title="Reset zoom"
        @click="zoomReset"
      >
        <Maximize2 class="size-4" />
      </Button>

      <Button
        :variant="noteCursorActive ? 'default' : 'outline'"
        size="icon"
        class="h-9 w-9"
        :aria-label="noteCursorActive ? 'Cancel placing note' : 'Place note at point'"
        :title="noteCursorActive ? 'Cancel placing note (Esc)' : 'Place note at point'"
        :aria-pressed="noteCursorActive"
        @click="toggleNoteCursor"
      >
        <StickyNote class="size-4" />
      </Button>

      <Button
        variant="outline"
        size="icon"
        class="h-9 w-9"
        :aria-label="isReading ? 'Stop currently reading' : 'Mark currently reading'"
        :title="isReading ? 'Stop currently reading' : 'Mark currently reading'"
        @click="toggleReading"
      >
        <BookOpenCheck v-if="isReading" class="size-4 text-primary" />
        <BookOpen v-else class="size-4" />
      </Button>

      <Button
        variant="outline"
        size="icon"
        class="h-9 w-9"
        :aria-label="bookmarksOpen ? 'Hide bookmarks panel' : 'Show bookmarks panel'"
        :title="bookmarksOpen ? 'Hide bookmarks panel' : 'Show bookmarks panel'"
        @click="bookmarksOpen = !bookmarksOpen"
      >
        <PanelRightClose v-if="bookmarksOpen" class="size-4" />
        <PanelRightOpen v-else class="size-4" />
      </Button>
    </div>
  </Teleport>

  <div class="flex h-[calc(100vh-3.5rem)] min-h-0">
    <aside
      class="flex flex-col border-r border-border bg-card/30 shrink-0 overflow-hidden transition-all duration-200"
      :class="tocOpen ? 'w-full md:w-72' : 'w-0'"
    >
      <div class="flex items-center justify-between px-3 py-3 border-b border-border min-w-[16rem]">
        <h2 class="text-sm font-semibold">Contents</h2>
        <Button
          variant="ghost"
          size="icon"
          class="h-8 w-8"
          aria-label="Close contents"
          title="Close contents"
          @click="tocOpen = false"
        >
          <X class="size-4" />
        </Button>
      </div>
      <div class="overflow-y-auto flex-1 px-2 py-2 min-w-[16rem]">
        <PdfOutline
          v-if="outline.length > 0"
          :nodes="outline"
          @jump="goToInstant"
        />
        <p v-else-if="outlineLoaded" class="text-sm text-muted-foreground">
          This document has no table of contents.
        </p>
        <p v-else class="text-sm text-muted-foreground">Loading…</p>
      </div>
    </aside>

    <div
      ref="scrollEl"
      class="reader-scroll h-full flex-1 min-w-0 overflow-auto bg-background relative"
      :class="noteCursorActive ? 'reader-note-cursor' : ''"
      @pointermove="bumpPaginator"
    >
      <p v-if="loading || pdfLoading" class="text-muted-foreground text-center py-6">
        Loading…
      </p>
      <p v-else-if="error" class="text-destructive text-center py-6">{{ error }}</p>
      <p v-else-if="renderError" class="text-destructive text-center py-6">
        {{ renderError }}
      </p>
      <div
        :class="[
          'py-4 px-2',
          twoPage
            ? 'flex flex-row flex-wrap justify-center gap-4'
            : 'flex flex-col items-center gap-4',
        ]"
      >
        <div
          v-for="(meta, i) in pages"
          :key="i"
          class="reader-page-row flex flex-row items-start gap-2"
        >
          <div
            :ref="(el) => setPageWrapper(el as Element | null, i)"
            class="reader-page relative"
            :data-page-number="i + 1"
            :style="{ width: `${meta.width}px`, height: `${meta.height}px` }"
            @pointerdown="(e) => onPageWrapperPointerDown(e, i + 1)"
          >
            <canvas class="reader-canvas" />
            <div class="page-label">{{ i + 1 }}</div>
            <div
              v-for="n in positionedNotesFor(i + 1)"
              :key="`pos-${n.id}`"
              class="absolute z-10 -translate-x-1/2 -translate-y-1/2"
              :style="{ left: `${(n.x ?? 0) * 100}%`, top: `${(n.y ?? 0) * 100}%` }"
              @mouseenter="showNoteHover(n.id)"
              @mouseleave="hideNoteHover"
              @pointerdown.stop
            >
              <button
                type="button"
                class="h-7 w-7 rounded-full bg-yellow-100 dark:bg-yellow-900/60 text-yellow-800 dark:text-yellow-200 border border-yellow-400 shadow flex items-center justify-center hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-ring"
                :aria-label="`Note on page ${i + 1}`"
                @click.stop="openEditNote(n)"
                @focus="showNoteHover(n.id)"
                @blur="hideNoteHover"
              >
                <StickyNote class="size-3.5" />
              </button>
              <div
                v-if="hoveredNoteId === n.id"
                role="tooltip"
                class="absolute top-0 z-20 w-72 max-h-60 overflow-auto bg-popover text-popover-foreground border shadow-lg rounded-md p-3 text-sm whitespace-pre-wrap pointer-events-none"
                :class="(n.x ?? 0) > 0.7 ? 'right-full mr-2' : 'left-full ml-2'"
              >
                {{ n.body }}
              </div>
            </div>
          </div>
          <div class="reader-gutter w-10 flex flex-col gap-1 items-center pt-1 shrink-0">
            <div
              v-for="n in gutterNotesFor(i + 1)"
              :key="n.id"
              class="relative"
              @mouseenter="showNoteHover(n.id)"
              @mouseleave="hideNoteHover"
            >
              <button
                type="button"
                class="h-8 w-8 rounded-md bg-yellow-100 dark:bg-yellow-900/40 text-yellow-800 dark:text-yellow-200 shadow-sm flex items-center justify-center hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-ring"
                :aria-label="`Note on page ${i + 1}`"
                @click="openEditNote(n)"
                @focus="showNoteHover(n.id)"
                @blur="hideNoteHover"
              >
                <StickyNote class="size-4" />
              </button>
              <div
                v-if="hoveredNoteId === n.id"
                role="tooltip"
                class="absolute right-full mr-2 top-0 z-20 w-72 max-h-60 overflow-auto bg-popover text-popover-foreground border shadow-lg rounded-md p-3 text-sm whitespace-pre-wrap pointer-events-none"
              >
                {{ n.body }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      class="reader-paginator fixed bottom-4 left-1/2 -translate-x-1/2 z-30 flex items-center gap-1 bg-background/85 backdrop-blur shadow-lg rounded-full border px-2 py-1 transition-opacity duration-300"
      :class="paginatorActive ? 'opacity-100' : 'opacity-30 hover:opacity-100'"
      @pointerenter="bumpPaginator"
      @pointermove.stop="bumpPaginator"
    >
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 rounded-full"
        aria-label="First page"
        title="First page"
        :disabled="currentPage <= 1"
        @click="goFirst"
      >
        <ChevronsLeft class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 rounded-full"
        aria-label="Previous page"
        title="Previous page"
        :disabled="currentPage <= 1"
        @click="prev"
      >
        <ChevronLeft class="size-4" />
      </Button>
      <div class="hidden sm:flex items-center gap-1">
        <Input
          v-model.number="pageInput"
          type="number"
          min="1"
          :max="pdfDoc?.numPages ?? 1"
          class="h-8 text-center"
          :style="{ width: pageInputWidth }"
          aria-label="Page number"
          title="Page number"
          @change="onPageInput"
        />
        <span class="text-xs text-muted-foreground px-1 tabular-nums">
          / {{ pdfDoc?.numPages ?? '—' }}
        </span>
      </div>
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 rounded-full"
        aria-label="Next page"
        title="Next page"
        :disabled="!pdfDoc || currentPage >= pdfDoc.numPages"
        @click="next"
      >
        <ChevronRight class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        class="h-8 w-8 rounded-full"
        aria-label="Last page"
        title="Last page"
        :disabled="!pdfDoc || currentPage >= pdfDoc.numPages"
        @click="goLast"
      >
        <ChevronsRight class="size-4" />
      </Button>
    </div>

    <aside
      class="flex flex-col border-l border-border bg-card/30 shrink-0 overflow-hidden transition-all duration-200"
      :class="bookmarksOpen ? 'w-full md:w-80' : 'w-0'"
    >
      <div class="flex items-center justify-between px-3 py-3 border-b border-border min-w-[18rem]">
        <h2 class="text-sm font-semibold">Bookmarks</h2>
        <Button
          variant="ghost"
          size="icon"
          class="h-8 w-8"
          aria-label="Close bookmarks"
          title="Close bookmarks"
          @click="bookmarksOpen = false"
        >
          <X class="size-4" />
        </Button>
      </div>
      <div class="overflow-y-auto flex-1 px-3 py-3 min-w-[18rem]">
        <BookmarkPanel
          :bookmarks="bookmarks"
          :notes="notes"
          :current-page="currentPage"
          @jump="goToInstant"
          @add="(page, label) => addBookmark(page, label)"
          @remove="(id) => removeBookmark(id)"
          @add-note="(page) => openCreateNote(page)"
          @edit-note="(n) => openEditNote(n)"
          @remove-note="(id) => removeNoteFromPanel(id)"
        />
      </div>
    </aside>
  </div>

  <NoteDialog
    :open="noteDialogOpen"
    :existing="noteDialogExisting"
    :initial-page="noteDialogPage"
    :initial-x="noteDialogX"
    :initial-y="noteDialogY"
    :max-page="pdfDoc?.numPages ?? totalPages"
    @update:open="(v) => (noteDialogOpen = v)"
    @save="onNoteSave"
    @delete="onNoteDelete"
  />
</template>

<style scoped>
.reader-page {
  position: relative;
  background: white;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
  max-width: 100%;
}
.reader-canvas {
  display: block;
  width: 100%;
  height: 100%;
  background: white;
}
.page-label {
  position: absolute;
  bottom: 4px;
  right: 6px;
  font-size: 10px;
  color: rgba(0, 0, 0, 0.5);
  background: rgba(255, 255, 255, 0.6);
  padding: 0 4px;
  border-radius: 2px;
  pointer-events: none;
}
.reader-note-cursor .reader-page,
.reader-note-cursor .reader-page * {
  cursor: crosshair !important;
}
</style>
