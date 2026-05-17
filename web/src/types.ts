// mirror of internal/api/dto.go shapes. dates are RFC 3339 strings.

export type Status =
  | 'never_started'
  | 'currently_reading'
  | 'in_progress'
  | 'completed'

export interface Progress {
  currentPage: number
  totalPages: number
  percent: number
  lastReadAt: string | null
  currentlyReading?: boolean
  status?: Status
}

export interface Bookmark {
  id: number
  page: number
  label: string | null
  createdAt: string
}

export interface Note {
  id: number
  page: number
  body: string
  createdAt: string
  updatedAt: string
  x?: number | null
  y?: number | null
}

export interface BookSummary {
  path: string
  title: string
  category: string
  sizeBytes: number
  fingerprint: string
  progress: Progress | null
  bookmarkCount: number
  collectionIds: number[]
  status: Status
  hidden: boolean
}

export interface Book {
  path: string
  title: string
  category: string
  sizeBytes: number
  fingerprint: string
  addedAt: string
  progress: Progress | null
  bookmarks: Bookmark[]
  notes: Note[]
  collectionIds?: number[]
  status?: Status
  hidden: boolean
}

export interface Category {
  name: string
  books: BookSummary[]
}

export type CollectionSource = 'scan' | 'manual'

export interface Collection {
  id: number
  name: string
  parentId: number | null
  source: CollectionSource
  folderPath: string
}

export interface Library {
  categories: Category[]
  scannedAt: string
  libraryConfigured: boolean
  collections: Collection[]
}

export interface Settings {
  libraryDir: string
}

export interface ScanStatus {
  running: boolean
  startedAt: string
  finishedAt: string | null
  added: number
  updated: number
  removed: number
  error: string | null
}

export interface ApiErrorEnvelope {
  error: { code: string; message: string }
}
