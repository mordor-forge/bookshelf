// cover thumbnail service: renders page 1 of a PDF via pdfjs into a JPEG blob
// cached in IndexedDB. keyed by book path; invalidated when fingerprint differs.
import * as pdfjsLib from 'pdfjs-dist'
import PdfWorker from 'pdfjs-dist/build/pdf.worker.mjs?worker'
import { bookFileUrl } from '../api'

const DB_NAME = 'bookshelf-covers'
const DB_VERSION = 1
const STORE = 'covers'
const MAX_ENTRIES = 200
const EVICT_TO = 180
const TARGET_WIDTH = 320
const JPEG_QUALITY = 0.78
const MAX_CONCURRENCY = 3

interface CoverRecord {
  path: string
  fingerprint: string
  blob: Blob
  createdAt: number
}

let workerInitialised = false
function ensureWorker(): void {
  if (workerInitialised) return
  // do not stomp on an existing worker port (e.g. ReaderPage already set one).
  if (!pdfjsLib.GlobalWorkerOptions.workerPort) {
    pdfjsLib.GlobalWorkerOptions.workerPort = new PdfWorker()
  }
  workerInitialised = true
}

let dbPromise: Promise<IDBDatabase> | null = null
function openDb(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) {
        db.createObjectStore(STORE, { keyPath: 'path' })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error ?? new Error('failed to open cover db'))
  })
  return dbPromise
}

function tx(db: IDBDatabase, mode: IDBTransactionMode): IDBObjectStore {
  return db.transaction(STORE, mode).objectStore(STORE)
}

async function dbGet(path: string): Promise<CoverRecord | null> {
  const db = await openDb()
  return new Promise((resolve, reject) => {
    const req = tx(db, 'readonly').get(path)
    req.onsuccess = () => resolve((req.result as CoverRecord | undefined) ?? null)
    req.onerror = () => reject(req.error)
  })
}

async function dbPut(record: CoverRecord): Promise<void> {
  const db = await openDb()
  await new Promise<void>((resolve, reject) => {
    const req = tx(db, 'readwrite').put(record)
    req.onsuccess = () => resolve()
    req.onerror = () => reject(req.error)
  })
  void evictIfNeeded()
}

async function evictIfNeeded(): Promise<void> {
  try {
    const db = await openDb()
    const count = await new Promise<number>((resolve, reject) => {
      const req = tx(db, 'readonly').count()
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => reject(req.error)
    })
    if (count <= MAX_ENTRIES) return
    const records: { path: string; createdAt: number }[] = await new Promise(
      (resolve, reject) => {
        const out: { path: string; createdAt: number }[] = []
        const req = tx(db, 'readonly').openCursor()
        req.onsuccess = () => {
          const cur = req.result
          if (cur) {
            const v = cur.value as CoverRecord
            out.push({ path: v.path, createdAt: v.createdAt })
            cur.continue()
          } else {
            resolve(out)
          }
        }
        req.onerror = () => reject(req.error)
      },
    )
    records.sort((a, b) => a.createdAt - b.createdAt)
    const toDelete = records.slice(0, records.length - EVICT_TO)
    await new Promise<void>((resolve, reject) => {
      const store = tx(db, 'readwrite')
      for (const r of toDelete) store.delete(r.path)
      store.transaction.oncomplete = () => resolve()
      store.transaction.onerror = () => reject(store.transaction.error)
    })
  } catch {
    // best-effort eviction; ignore failures
  }
}

export async function clearAllCovers(): Promise<void> {
  const db = await openDb()
  await new Promise<void>((resolve, reject) => {
    const req = tx(db, 'readwrite').clear()
    req.onsuccess = () => resolve()
    req.onerror = () => reject(req.error)
  })
}

// concurrency-limited render queue keyed by path.
const inFlight = new Map<string, Promise<string | null>>()
let active = 0
const waiters: Array<() => void> = []

async function acquire(): Promise<void> {
  if (active < MAX_CONCURRENCY) {
    active++
    return
  }
  await new Promise<void>((resolve) => waiters.push(resolve))
  active++
}

function release(): void {
  active--
  const next = waiters.shift()
  if (next) next()
}

async function renderCover(path: string, fingerprint: string): Promise<Blob | null> {
  ensureWorker()
  // disableStream:false + disableAutoFetch:true lets pdfjs use range requests
  // against the byte-range-capable /books endpoint and only fetch what page 1
  // needs to render.
  const task = pdfjsLib.getDocument({
    url: bookFileUrl(path),
    disableStream: false,
    disableAutoFetch: true,
  })
  try {
    const doc = await task.promise
    try {
      const page = await doc.getPage(1)
      const baseVp = page.getViewport({ scale: 1 })
      const scale = TARGET_WIDTH / baseVp.width
      const viewport = page.getViewport({ scale })
      const canvas = document.createElement('canvas')
      canvas.width = Math.ceil(viewport.width)
      canvas.height = Math.ceil(viewport.height)
      const ctx = canvas.getContext('2d')
      if (!ctx) return null
      ctx.fillStyle = '#ffffff'
      ctx.fillRect(0, 0, canvas.width, canvas.height)
      await page.render({ canvasContext: ctx, viewport }).promise
      const blob: Blob | null = await new Promise((resolve) =>
        canvas.toBlob((b) => resolve(b), 'image/jpeg', JPEG_QUALITY),
      )
      if (!blob) return null
      if (fingerprint) {
        await dbPut({ path, fingerprint, blob, createdAt: Date.now() })
      }
      return blob
    } finally {
      await doc.destroy().catch(() => {})
    }
  } catch {
    return null
  }
}

async function resolveCover(path: string, fingerprint: string): Promise<string | null> {
  try {
    const existing = await dbGet(path)
    if (existing && existing.fingerprint === fingerprint && fingerprint !== '') {
      return URL.createObjectURL(existing.blob)
    }
  } catch {
    // db failure — fall through to render
  }
  await acquire()
  let blob: Blob | null = null
  try {
    blob = await renderCover(path, fingerprint)
  } finally {
    release()
  }
  if (!blob) return null
  return URL.createObjectURL(blob)
}

export function getCover(book: { path: string; fingerprint: string }): Promise<string | null> {
  const existing = inFlight.get(book.path)
  if (existing) return existing
  const p = resolveCover(book.path, book.fingerprint).finally(() => {
    inFlight.delete(book.path)
  })
  inFlight.set(book.path, p)
  return p
}

export function revokeCover(url: string | null | undefined): void {
  if (url && url.startsWith('blob:')) {
    try {
      URL.revokeObjectURL(url)
    } catch {
      // ignore
    }
  }
}
