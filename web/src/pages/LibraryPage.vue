<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { PanelLeftClose, PanelLeftOpen, Plus, RefreshCw, Upload, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import {
  addBookToCollection,
  createCollection,
  deleteCollection,
  getScan,
  patchCollection,
  postScan,
  removeBookFromCollection,
  uploadBook,
} from '../api'
import { useLibrary } from '../composables/useLibrary'
import { useSettings } from '../composables/useSettings'
import type { Status } from '../types'
import BookCard from '../components/BookCard.vue'
import CollectionTree from '../components/CollectionTree.vue'
import CollectionDialog from '../components/CollectionDialog.vue'
import AddToCollectionDialog from '../components/AddToCollectionDialog.vue'
import UploadBookDialog from '../components/UploadBookDialog.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import type { TreeNode, Selection } from '../composables/useLibrary'

const {
  library,
  loading,
  error,
  collections,
  allBooks,
  tree,
  selection,
  setSelection,
  filters,
  filteredBooks,
  load,
  updateBookStatus,
  updateBookHidden,
  addCollectionLocal,
  updateCollectionLocal,
  removeCollectionLocal,
  addBookToCollectionLocal,
  removeBookFromCollectionLocal,
} = useLibrary()

const scanning = ref(false)
let pollHandle: ReturnType<typeof setInterval> | null = null

const settings = useSettings()
const treePanelOpen = computed({
  get: () => settings.libraryPanelOpen,
  set: (v: boolean) => { settings.libraryPanelOpen = v },
})

function toggleTreePanel(): void {
  settings.libraryPanelOpen = !settings.libraryPanelOpen
}
const collectionDialogOpen = ref(false)
const dialogMode = ref<'create' | 'rename'>('create')
const dialogInitialName = ref('')
const dialogInitialParent = ref<number | null>(null)
const dialogExcludeId = ref<number | undefined>(undefined)
const renamingId = ref<number | null>(null)

const addToCollectionOpen = ref(false)
const addToCollectionBook = ref<{ path: string; title: string } | null>(null)

const uploadDialogOpen = ref(false)
const uploadDialogRef = ref<InstanceType<typeof UploadBookDialog> | null>(null)

async function onUploadSubmit(payload: {
  file: File
  collectionIds: number[]
}): Promise<void> {
  const dlg = uploadDialogRef.value
  dlg?.setUploading(true)
  try {
    await uploadBook(
      payload.file,
      payload.collectionIds.length > 0 ? payload.collectionIds : undefined,
      (frac) => dlg?.setProgress(frac),
    )
    toast.success('Book uploaded')
    uploadDialogOpen.value = false
    await load()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Upload failed')
  } finally {
    dlg?.setUploading(false)
  }
}

const uncategorizedCount = computed(
  () => allBooks.value.filter((b) => b.collectionIds.length === 0).length,
)

const selectionTitle = computed(() => {
  const sel = selection.value
  if (sel.kind === 'all') return 'All books'
  if (sel.kind === 'uncategorized') return 'Uncategorized'
  const c = collections.value.find((x) => x.id === sel.id)
  return c?.name ?? 'Collection'
})

const currentCollectionId = computed<number | null>(() => {
  const sel = selection.value
  return sel.kind === 'collection' ? sel.id : null
})

const currentCollectionIsManual = computed(() => {
  const id = currentCollectionId.value
  if (id === null) return false
  return collections.value.find((c) => c.id === id)?.source === 'manual'
})

const statusOptions: { value: Status | 'all'; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'never_started', label: 'Never started' },
  { value: 'in_progress', label: 'In progress' },
  { value: 'currently_reading', label: 'Currently reading' },
  { value: 'completed', label: 'Completed' },
]

const visibilityOptions: { value: 'hidden'; label: string }[] = [
  { value: 'hidden', label: 'Hidden' },
]

function startPolling(): void {
  if (pollHandle !== null) return
  pollHandle = setInterval(async () => {
    try {
      const status = await getScan()
      if (!status.running) {
        stopPolling()
        scanning.value = false
        toast.success('Scan complete')
        await load()
      }
    } catch {
      stopPolling()
      scanning.value = false
    }
  }, 2000)
}

function stopPolling(): void {
  if (pollHandle !== null) {
    clearInterval(pollHandle)
    pollHandle = null
  }
}

async function onRescan(): Promise<void> {
  scanning.value = true
  try {
    const res = await postScan()
    if ('conflict' in res) toast.info('Scan already in progress')
    else toast.success('Scan started')
    startPolling()
  } catch (err) {
    scanning.value = false
    toast.error(err instanceof Error ? err.message : 'Scan failed')
  }
}

function onSelect(sel: Selection): void {
  setSelection(sel)
  if (window.innerWidth < 768) treePanelOpen.value = false
}

function openCreate(parent: TreeNode | null): void {
  dialogMode.value = 'create'
  dialogInitialName.value = ''
  dialogInitialParent.value = parent?.id ?? null
  dialogExcludeId.value = undefined
  renamingId.value = null
  collectionDialogOpen.value = true
}

function openRename(node: TreeNode): void {
  dialogMode.value = 'rename'
  dialogInitialName.value = node.name
  dialogInitialParent.value = node.parentId
  dialogExcludeId.value = node.id
  renamingId.value = node.id
  collectionDialogOpen.value = true
}

async function onDialogSubmit(payload: { name: string; parentId: number | null }): Promise<void> {
  try {
    if (dialogMode.value === 'create') {
      const c = await createCollection(payload.name, payload.parentId)
      addCollectionLocal(c)
      toast.success(`Created "${c.name}"`)
    } else if (renamingId.value !== null) {
      const c = await patchCollection(renamingId.value, {
        name: payload.name,
        changeParent: true,
        parentId: payload.parentId,
      })
      updateCollectionLocal(c)
      toast.success(`Saved`)
    }
    collectionDialogOpen.value = false
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Operation failed')
  }
}

async function onDelete(node: TreeNode): Promise<void> {
  if (!window.confirm(`Delete collection "${node.name}"?`)) return
  try {
    await deleteCollection(node.id)
    removeCollectionLocal(node.id)
    if (selection.value.kind === 'collection' && selection.value.id === node.id) {
      setSelection({ kind: 'all' })
    }
    toast.success('Deleted')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Delete failed')
  }
}

function onBookStatus(path: string, status: Status, currentlyReading: boolean): void {
  updateBookStatus(path, status, currentlyReading)
}

function onBookHidden(path: string, hidden: boolean): void {
  updateBookHidden(path, hidden)
}

function openAddToCollection(book: { path: string; title: string }): void {
  addToCollectionBook.value = { path: book.path, title: book.title }
  addToCollectionOpen.value = true
}

async function onPickCollection(id: number): Promise<void> {
  const b = addToCollectionBook.value
  if (!b) return
  try {
    await addBookToCollection(id, b.path)
    addBookToCollectionLocal(id, b.path)
    addToCollectionOpen.value = false
    toast.success('Added to collection')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Add failed')
  }
}

async function onRemoveFromCollection(book: { path: string }, collectionId: number): Promise<void> {
  try {
    await removeBookFromCollection(collectionId, book.path)
    removeBookFromCollectionLocal(collectionId, book.path)
    toast.success('Removed from collection')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Remove failed')
  }
}

onMounted(load)
onUnmounted(stopPolling)
</script>

<template>
  <Teleport defer to="#header-actions-left">
    <Button
      type="button"
      variant="ghost"
      size="sm"
      class="h-9 gap-2 px-2 md:px-3"
      :aria-label="treePanelOpen ? 'Hide collections panel' : 'Show collections panel'"
      :title="treePanelOpen ? 'Hide collections panel' : 'Show collections panel'"
      @click="toggleTreePanel"
    >
      <PanelLeftClose v-if="treePanelOpen" class="size-4" />
      <PanelLeftOpen v-else class="size-4" />
      <span class="hidden md:inline">Collections</span>
    </Button>
  </Teleport>

  <Teleport defer to="#header-actions-right">
    <select
      v-model="filters.status"
      class="hidden md:inline-flex h-9 rounded-md border border-input bg-transparent px-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
      aria-label="Filter by status"
      title="Filter by status"
    >
      <optgroup label="Status">
        <option v-for="opt in statusOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </optgroup>
      <optgroup label="Visibility">
        <option v-for="opt in visibilityOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </optgroup>
    </select>
    <Input
      v-model="filters.query"
      placeholder="Filter titles…"
      class="hidden md:inline-flex h-9 w-40 lg:w-56"
      aria-label="Filter titles"
      title="Filter titles"
    />

    <Button
      v-if="library?.libraryConfigured"
      type="button"
      variant="outline"
      size="sm"
      class="h-9 gap-2 px-2 md:px-3"
      aria-label="Upload book"
      title="Upload book"
      @click="uploadDialogOpen = true"
    >
      <Upload class="size-4" />
      <span class="hidden md:inline">Upload</span>
    </Button>

    <Button
      v-if="library?.libraryConfigured"
      type="button"
      size="sm"
      class="h-9 gap-2 px-2 md:px-3"
      :disabled="scanning"
      :aria-label="scanning ? 'Scanning library' : 'Rescan library'"
      :title="scanning ? 'Scanning library' : 'Rescan library'"
      @click="onRescan"
    >
      <RefreshCw class="size-4" :class="scanning ? 'animate-spin' : ''" />
      <span class="hidden md:inline">{{ scanning ? 'Scanning…' : 'Rescan' }}</span>
    </Button>
  </Teleport>

  <div class="flex h-[calc(100vh-3.5rem)] min-h-0">
    <aside
      class="flex flex-col border-r border-border bg-card/30 shrink-0 overflow-hidden transition-all duration-200"
      :class="treePanelOpen ? 'w-full md:w-72' : 'w-0'"
    >
      <div class="flex items-center justify-between px-3 py-3 border-b border-border min-w-[16rem]">
        <h2 class="text-sm font-semibold">Collections</h2>
        <div class="flex items-center gap-1">
          <Button size="sm" variant="outline" @click="openCreate(null)">
            <Plus class="size-3.5 mr-1" /> New
          </Button>
          <Button
            variant="ghost"
            size="icon"
            class="h-8 w-8"
            aria-label="Close collections"
            title="Close collections"
            @click="treePanelOpen = false"
          >
            <X class="size-4" />
          </Button>
        </div>
      </div>
      <div class="overflow-y-auto px-2 py-2 flex-1 min-w-[16rem]">
        <CollectionTree
          :nodes="tree"
          :selection="selection"
          :total-books="allBooks.length"
          :uncategorized-count="uncategorizedCount"
          @select="onSelect"
          @add-child="openCreate"
          @rename="openRename"
          @remove="onDelete"
        />
      </div>
    </aside>

    <section class="flex-1 min-w-0 flex flex-col">
      <div class="overflow-y-auto px-4 md:px-6 py-4 flex-1">
        <div class="flex items-baseline gap-2 mb-3">
          <h2 class="text-base font-semibold tracking-tight truncate">
            {{ selectionTitle }}
          </h2>
          <span class="text-xs text-muted-foreground tabular-nums">
            {{ filteredBooks.length }} book{{ filteredBooks.length === 1 ? '' : 's' }}
          </span>
        </div>
        <div
          v-if="loading"
          class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] auto-rows-fr gap-4"
        >
          <div v-for="i in 6" :key="i" class="rounded-lg border border-border overflow-hidden">
            <Skeleton class="aspect-[3/4] w-full rounded-none" />
            <div class="p-4 space-y-3">
              <Skeleton class="h-4 w-3/4" />
              <Skeleton class="h-3 w-1/2" />
            </div>
          </div>
        </div>
        <p v-else-if="error" class="text-destructive">Failed to load library: {{ error }}</p>
        <Card v-else-if="library && !library.libraryConfigured">
          <CardContent class="py-10 flex flex-col items-center text-center gap-3">
            <p class="text-base font-medium">No library configured</p>
            <p class="text-sm text-muted-foreground">
              Set the library directory in settings to start scanning books.
            </p>
            <Button as-child>
              <router-link to="/settings">Open settings</router-link>
            </Button>
          </CardContent>
        </Card>
        <p
          v-else-if="library && allBooks.length === 0"
          class="text-muted-foreground"
        >
          No books yet. Upload a PDF or click Rescan.
        </p>
        <p
          v-else-if="filteredBooks.length === 0"
          class="text-muted-foreground"
        >
          No books match the current filters.
        </p>
        <div
          v-else
          class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] auto-rows-fr gap-4"
        >
          <BookCard
            v-for="b in filteredBooks"
            :key="b.path"
            :book="b"
            :current-collection-id="currentCollectionId"
            :current-collection-is-manual="currentCollectionIsManual"
            @status-changed="onBookStatus"
            @hidden-changed="onBookHidden"
            @add-to-collection="(book) => openAddToCollection({ path: book.path, title: book.title })"
            @remove-from-collection="(book, id) => onRemoveFromCollection(book, id)"
          />
        </div>
      </div>
    </section>
  </div>

  <CollectionDialog
    v-model:open="collectionDialogOpen"
    :mode="dialogMode"
    :initial-name="dialogInitialName"
    :initial-parent-id="dialogInitialParent"
    :exclude-id="dialogExcludeId"
    :parents="collections.filter((c) => c.source === 'manual')"
    @submit="onDialogSubmit"
  />

  <UploadBookDialog
    ref="uploadDialogRef"
    v-model:open="uploadDialogOpen"
    :collections="collections"
    @submit="onUploadSubmit"
  />

  <AddToCollectionDialog
    v-if="addToCollectionBook"
    v-model:open="addToCollectionOpen"
    :collections="collections"
    :book-path="addToCollectionBook.path"
    :book-title="addToCollectionBook.title"
    @pick="onPickCollection"
  />

</template>
