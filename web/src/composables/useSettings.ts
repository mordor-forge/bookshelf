import { reactive, watch } from 'vue'

interface Settings {
  libraryPanelOpen: boolean
  readerTocOpen: boolean
  readerBookmarksOpen: boolean
  readerZoom: number
  readerTwoPage: boolean
  readerPdfInvert: boolean
  treeExpanded: Set<number>
}

function readBool(key: string, def: boolean): boolean {
  try {
    const v = localStorage.getItem(key)
    return v === null ? def : v === 'true'
  } catch { return def }
}

function readNum(key: string, def: number): number {
  try {
    const v = localStorage.getItem(key)
    if (v === null) return def
    const n = Number.parseFloat(v)
    return Number.isFinite(n) ? n : def
  } catch { return def }
}

function readSet(key: string): Set<number> {
  try {
    const v = localStorage.getItem(key)
    if (!v) return new Set()
    const arr = JSON.parse(v) as unknown
    if (!Array.isArray(arr)) return new Set()
    return new Set(arr.filter((x): x is number => typeof x === 'number'))
  } catch { return new Set() }
}

function persist(key: string, value: unknown): void {
  try {
    localStorage.setItem(key, value instanceof Set ? JSON.stringify([...value]) : String(value))
  } catch { /* ignore */ }
}

const settings = reactive<Settings>({
  libraryPanelOpen:   readBool('bookshelf:library-panel-open', window.innerWidth >= 1024),
  readerTocOpen:      readBool('bookshelf:reader-toc-open', false),
  readerBookmarksOpen: readBool('bookshelf:reader-bookmarks-open', false),
  readerZoom:         readNum('bookshelf:zoom', 1),
  readerTwoPage:      readBool('bookshelf:two-page', false),
  readerPdfInvert:    readBool('bookshelf:pdf-invert', false),
  treeExpanded:       readSet('bookshelf:tree-expanded'),
})

watch(() => settings.libraryPanelOpen,   (v) => persist('bookshelf:library-panel-open', v))
watch(() => settings.readerTocOpen,      (v) => persist('bookshelf:reader-toc-open', v))
watch(() => settings.readerBookmarksOpen,(v) => persist('bookshelf:reader-bookmarks-open', v))
watch(() => settings.readerZoom,         (v) => persist('bookshelf:zoom', v))
watch(() => settings.readerTwoPage,      (v) => persist('bookshelf:two-page', v))
watch(() => settings.readerPdfInvert,    (v) => persist('bookshelf:pdf-invert', v))
watch(() => settings.treeExpanded,       (v) => persist('bookshelf:tree-expanded', v), { deep: true })

export function useSettings(): typeof settings {
  return settings
}
