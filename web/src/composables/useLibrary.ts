import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getLibrary } from '../api'
import type { BookSummary, Collection, Library, Status } from '../types'

export interface TreeNode {
  id: number
  name: string
  parentId: number | null
  source: 'scan' | 'manual'
  children: TreeNode[]
  bookCount: number
}

export type Selection =
  | { kind: 'all' }
  | { kind: 'uncategorized' }
  | { kind: 'collection'; id: number }

export interface Filters {
  status: Status | 'all' | 'hidden'
  query: string
}

// selection lives in the URL query: ?c=42 or ?c=uncategorized (omitted = all).
function parseSelection(q: unknown): Selection {
  if (typeof q !== 'string' || q === '') return { kind: 'all' }
  if (q === 'uncategorized') return { kind: 'uncategorized' }
  const id = Number(q)
  if (Number.isFinite(id) && id > 0) return { kind: 'collection', id }
  return { kind: 'all' }
}

function selectionToQuery(sel: Selection): string | undefined {
  if (sel.kind === 'all') return undefined
  if (sel.kind === 'uncategorized') return 'uncategorized'
  return String(sel.id)
}

export function useLibrary() {
  const route = useRoute()
  const router = useRouter()

  const library = ref<Library | null>(null)
  const loading = ref(true)
  const error = ref<string | null>(null)
  const selection = computed<Selection>(() => parseSelection(route.query.c))
  const filters = ref<Filters>({ status: 'all', query: '' })

  const collections = computed<Collection[]>(
    () => library.value?.collections ?? [],
  )

  const allBooks = computed<BookSummary[]>(() => {
    const cats = library.value?.categories ?? []
    return cats.flatMap((c) => c.books)
  })

  // map collection id -> recursive descendant book count.
  const tree = computed<TreeNode[]>(() => {
    const cols = collections.value
    const books = allBooks.value
    const byParent = new Map<number | null, Collection[]>()
    for (const c of cols) {
      const arr = byParent.get(c.parentId) ?? []
      arr.push(c)
      byParent.set(c.parentId, arr)
    }
    // direct book count per collection.
    const directCount = new Map<number, number>()
    for (const b of books) {
      for (const id of b.collectionIds) {
        directCount.set(id, (directCount.get(id) ?? 0) + 1)
      }
    }
    function build(parentId: number | null): TreeNode[] {
      const list = byParent.get(parentId) ?? []
      return list
        .slice()
        .sort((a, b) => a.name.localeCompare(b.name))
        .map((c) => {
          const children = build(c.id)
          const count =
            (directCount.get(c.id) ?? 0) +
            children.reduce((acc, ch) => acc + ch.bookCount, 0)
          return {
            id: c.id,
            name: c.name,
            parentId: c.parentId,
            source: c.source,
            children,
            bookCount: count,
          }
        })
    }
    return build(null)
  })

  // ids in subtree of a given collection id (inclusive).
  function collectIds(id: number): Set<number> {
    const out = new Set<number>()
    function walk(nodes: TreeNode[]): void {
      for (const n of nodes) {
        if (n.id === id) {
          collectAll(n, out)
          return
        }
        walk(n.children)
      }
    }
    function collectAll(n: TreeNode, target: Set<number>): void {
      target.add(n.id)
      for (const c of n.children) collectAll(c, target)
    }
    walk(tree.value)
    return out
  }

  const selectedBooks = computed<BookSummary[]>(() => {
    const sel = selection.value
    const books = allBooks.value
    if (sel.kind === 'all') return books
    if (sel.kind === 'uncategorized') {
      return books.filter((b) => b.collectionIds.length === 0)
    }
    const ids = collectIds(sel.id)
    return books.filter((b) => b.collectionIds.some((cid) => ids.has(cid)))
  })

  const filteredBooks = computed<BookSummary[]>(() => {
    const f = filters.value
    let list = selectedBooks.value
    if (f.status === 'hidden') {
      list = list.filter((b) => b.hidden)
    } else {
      list = list.filter((b) => !b.hidden)
      if (f.status !== 'all') {
        list = list.filter((b) => b.status === f.status)
      }
    }
    const q = f.query.trim().toLowerCase()
    if (q.length > 0) {
      list = list.filter((b) => b.title.toLowerCase().includes(q))
    }
    // pin currently-reading books to the top.
    return [...list].sort((a, b) => {
      const ar = a.status === 'currently_reading' ? 0 : 1
      const br = b.status === 'currently_reading' ? 0 : 1
      if (ar !== br) return ar - br
      return a.title.localeCompare(b.title)
    })
  })

  function setSelection(sel: Selection): void {
    const c = selectionToQuery(sel)
    const query = { ...route.query }
    if (c === undefined) delete query.c
    else query.c = c
    void router.replace({ query })
  }

  async function load(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      library.value = await getLibrary()
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    } finally {
      loading.value = false
    }
  }

  function updateBookStatus(
    path: string,
    status: Status,
    currentlyReading: boolean,
  ): void {
    const lib = library.value
    if (!lib) return
    for (const cat of lib.categories) {
      for (const b of cat.books) {
        if (b.path === path) {
          b.status = status
          if (b.progress) {
            b.progress.status = status
            b.progress.currentlyReading = currentlyReading
          }
        }
      }
    }
  }

  function updateBookHidden(path: string, hidden: boolean): void {
    const lib = library.value
    if (!lib) return
    for (const cat of lib.categories) {
      for (const b of cat.books) {
        if (b.path === path) {
          b.hidden = hidden
        }
      }
    }
  }

  function addCollectionLocal(c: Collection): void {
    const lib = library.value
    if (!lib) return
    lib.collections = [...lib.collections, c]
  }

  function updateCollectionLocal(c: Collection): void {
    const lib = library.value
    if (!lib) return
    lib.collections = lib.collections.map((x) => (x.id === c.id ? c : x))
  }

  function removeCollectionLocal(id: number): void {
    const lib = library.value
    if (!lib) return
    lib.collections = lib.collections.filter((c) => c.id !== id)
    for (const cat of lib.categories) {
      for (const b of cat.books) {
        b.collectionIds = b.collectionIds.filter((cid) => cid !== id)
      }
    }
  }

  function addBookToCollectionLocal(id: number, path: string): void {
    const lib = library.value
    if (!lib) return
    for (const cat of lib.categories) {
      for (const b of cat.books) {
        if (b.path === path && !b.collectionIds.includes(id)) {
          b.collectionIds = [...b.collectionIds, id]
        }
      }
    }
  }

  function removeBookFromCollectionLocal(id: number, path: string): void {
    const lib = library.value
    if (!lib) return
    for (const cat of lib.categories) {
      for (const b of cat.books) {
        if (b.path === path) {
          b.collectionIds = b.collectionIds.filter((cid) => cid !== id)
        }
      }
    }
  }

  return {
    library,
    loading,
    error,
    collections,
    allBooks,
    tree,
    selection,
    setSelection,
    filters,
    selectedBooks,
    filteredBooks,
    load,
    updateBookStatus,
    updateBookHidden,
    addCollectionLocal,
    updateCollectionLocal,
    removeCollectionLocal,
    addBookToCollectionLocal,
    removeBookFromCollectionLocal,
  }
}
