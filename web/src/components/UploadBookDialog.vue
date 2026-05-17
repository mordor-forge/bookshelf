<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from 'reka-ui'
import { X, Upload, FileText } from 'lucide-vue-next'
import type { Collection } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const props = defineProps<{
  open: boolean
  collections: Collection[]
}>()

const emit = defineEmits<{
  'update:open': [v: boolean]
  submit: [payload: { file: File; collectionIds: number[] }]
}>()

const file = ref<File | null>(null)
const selectedIds = ref<Set<number>>(new Set())
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const progress = ref(0)
const collectionSearch = ref('')

watch(
  () => props.open,
  (v) => {
    if (v) {
      file.value = null
      selectedIds.value = new Set()
      progress.value = 0
      uploading.value = false
      collectionSearch.value = ''
    }
  },
)

const sortedCollections = computed(() =>
  [...props.collections].sort((a, b) => {
    if (a.source !== b.source) return a.source === 'manual' ? -1 : 1
    return a.name.localeCompare(b.name)
  }),
)

const filteredCollections = computed(() => {
  const q = collectionSearch.value.trim().toLowerCase()
  if (q.length === 0) return sortedCollections.value
  return sortedCollections.value.filter((c) => c.name.toLowerCase().includes(q))
})

function pickFile(e: Event): void {
  const target = e.target as HTMLInputElement
  const f = target.files?.[0] ?? null
  if (f && /\.pdf$/i.test(f.name)) file.value = f
  else if (f) file.value = null
}

function onDrop(e: DragEvent): void {
  e.preventDefault()
  dragOver.value = false
  const f = e.dataTransfer?.files?.[0] ?? null
  if (f && /\.pdf$/i.test(f.name)) file.value = f
}

function onDragOver(e: DragEvent): void {
  e.preventDefault()
  dragOver.value = true
}

function onDragLeave(): void {
  dragOver.value = false
}

function toggleCollection(id: number): void {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

function canSubmit(): boolean {
  return file.value !== null && !uploading.value
}

function submit(): void {
  if (!file.value) return
  emit('submit', {
    file: file.value,
    collectionIds: Array.from(selectedIds.value),
  })
}

function setProgress(fraction: number): void {
  progress.value = Math.max(0, Math.min(1, fraction))
}

function setUploading(v: boolean): void {
  uploading.value = v
  if (!v) progress.value = 0
}

defineExpose({ setProgress, setUploading })
</script>

<template>
  <DialogRoot :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6 shadow-lg rounded-lg max-h-[90vh] overflow-y-auto"
      >
        <div class="flex items-start justify-between">
          <div>
            <DialogTitle class="text-lg font-semibold">Upload book</DialogTitle>
            <DialogDescription class="text-sm text-muted-foreground">
              Add a PDF to the library.
            </DialogDescription>
          </div>
          <DialogClose
            class="rounded-sm opacity-70 hover:opacity-100"
            aria-label="Close"
            :disabled="uploading"
          >
            <X class="size-4" />
          </DialogClose>
        </div>

        <div
          class="rounded-md border-2 border-dashed px-4 py-6 text-sm transition-colors cursor-pointer flex flex-col items-center gap-2"
          :class="dragOver ? 'border-primary bg-primary/5' : 'border-input'"
          role="button"
          tabindex="0"
          @click="fileInput?.click()"
          @keydown.enter.prevent="fileInput?.click()"
          @keydown.space.prevent="fileInput?.click()"
          @drop="onDrop"
          @dragover="onDragOver"
          @dragleave="onDragLeave"
        >
          <FileText v-if="file" class="size-6 text-primary" />
          <Upload v-else class="size-6 text-muted-foreground" />
          <p v-if="file" class="font-medium truncate max-w-full">{{ file.name }}</p>
          <p v-else class="text-muted-foreground">
            Drop a PDF here, or click to choose a file.
          </p>
          <input
            ref="fileInput"
            type="file"
            accept="application/pdf,.pdf"
            class="hidden"
            @change="pickFile"
          />
        </div>

        <div class="grid gap-1.5">
          <Label>Add to collections (optional)</Label>
          <Input
            v-model="collectionSearch"
            placeholder="Search collections…"
            :disabled="uploading"
          />
          <div class="flex flex-col gap-1 max-h-40 overflow-y-auto rounded-md border border-input p-1">
            <button
              v-for="c in filteredCollections"
              :key="c.id"
              type="button"
              class="flex items-center justify-between gap-2 text-left px-2 py-1.5 rounded hover:bg-accent text-sm"
              :class="selectedIds.has(c.id) ? 'bg-accent' : ''"
              :disabled="uploading"
              @click="toggleCollection(c.id)"
            >
              <span class="truncate">{{ c.name }}</span>
              <span
                v-if="selectedIds.has(c.id)"
                class="text-[10px] text-primary shrink-0"
              >selected</span>
            </button>
            <p
              v-if="filteredCollections.length === 0"
              class="text-sm text-muted-foreground px-2 py-1"
            >
              No collections.
            </p>
          </div>
        </div>

        <div v-if="uploading" class="grid gap-1.5">
          <div class="h-2 w-full bg-muted rounded-full overflow-hidden">
            <div
              class="h-full bg-primary transition-all"
              :style="{ width: `${Math.round(progress * 100)}%` }"
            />
          </div>
          <p class="text-xs text-muted-foreground tabular-nums">
            Uploading… {{ Math.round(progress * 100) }}%
          </p>
        </div>

        <div class="flex justify-end gap-2 pt-1">
          <DialogClose as-child>
            <Button type="button" variant="ghost" :disabled="uploading">Cancel</Button>
          </DialogClose>
          <Button type="button" :disabled="!canSubmit()" @click="submit">
            <Upload class="size-4 mr-2" />
            Upload
          </Button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
