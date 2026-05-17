<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from 'reka-ui'
import { MapPin, X } from 'lucide-vue-next'
import type { Note } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface Props {
  open: boolean
  existing: Note | null
  initialPage: number
  initialX?: number | null
  initialY?: number | null
  maxPage?: number
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:open': [v: boolean]
  save: [payload: {
    id: number | null
    page: number
    body: string
    x: number | null
    y: number | null
    clearPosition: boolean
  }]
  delete: [id: number]
}>()

const body = ref('')
const page = ref(1)
const noteX = ref<number | null>(null)
const noteY = ref<number | null>(null)
const clearPos = ref(false)
const textareaRef = ref<HTMLTextAreaElement | null>(null)

const anchored = computed(() => !clearPos.value && noteX.value !== null && noteY.value !== null)
const anchorLabel = computed(() => {
  if (!anchored.value || noteX.value === null || noteY.value === null) return ''
  return `Anchored at (${Math.round(noteX.value * 100)}%, ${Math.round(noteY.value * 100)}%)`
})

watch(
  () => props.open,
  (v) => {
    if (v) {
      clearPos.value = false
      if (props.existing) {
        body.value = props.existing.body
        page.value = props.existing.page
        noteX.value = props.existing.x ?? null
        noteY.value = props.existing.y ?? null
      } else {
        body.value = ''
        page.value = props.initialPage
        noteX.value = props.initialX ?? null
        noteY.value = props.initialY ?? null
      }
      void nextTick(() => {
        textareaRef.value?.focus()
      })
    }
  },
  { immediate: true },
)

function moveToGutter(): void {
  noteX.value = null
  noteY.value = null
  clearPos.value = true
}

function submit(): void {
  const trimmed = body.value.trim()
  if (trimmed.length === 0) return
  emit('save', {
    id: props.existing ? props.existing.id : null,
    page: page.value,
    body: trimmed,
    x: clearPos.value ? null : noteX.value,
    y: clearPos.value ? null : noteY.value,
    clearPosition: clearPos.value,
  })
}

function onDelete(): void {
  if (!props.existing) return
  if (!window.confirm('Delete this note?')) return
  emit('delete', props.existing.id)
}
</script>

<template>
  <DialogRoot :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6 shadow-lg rounded-lg"
      >
        <div class="flex items-start justify-between">
          <div>
            <DialogTitle class="text-lg font-semibold">
              {{ existing ? 'Edit note' : 'New note' }}
            </DialogTitle>
            <DialogDescription class="text-sm text-muted-foreground">
              {{ existing ? 'Update the note text or move it to another page.' : 'Anchor a note to a page in this book.' }}
            </DialogDescription>
          </div>
          <DialogClose class="rounded-sm opacity-70 hover:opacity-100 transition-opacity" aria-label="Close">
            <X class="size-4" />
          </DialogClose>
        </div>

        <form class="flex flex-col gap-3" @submit.prevent="submit">
          <div class="flex flex-col gap-1.5">
            <Label for="note-body">Note</Label>
            <textarea
              id="note-body"
              ref="textareaRef"
              v-model="body"
              rows="8"
              maxlength="10000"
              class="rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring resize-y"
              placeholder="Your note…"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="note-page">Page</Label>
            <Input
              id="note-page"
              v-model.number="page"
              type="number"
              min="1"
              :max="maxPage"
              class="w-28"
            />
          </div>
          <div v-if="anchored || (existing && (existing.x !== null && existing.x !== undefined))" class="flex items-center justify-between gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-sm">
            <span v-if="anchored" class="flex items-center gap-1.5 text-muted-foreground">
              <MapPin class="size-3.5" />
              {{ anchorLabel }}
            </span>
            <span v-else class="text-muted-foreground italic">Will move to the gutter on save.</span>
            <Button
              v-if="anchored"
              type="button"
              variant="ghost"
              size="sm"
              @click="moveToGutter"
            >
              Move to gutter
            </Button>
          </div>
          <div class="flex justify-between gap-2 pt-2">
            <Button
              v-if="existing"
              type="button"
              variant="destructive"
              @click="onDelete"
            >
              Delete
            </Button>
            <span v-else />
            <div class="flex gap-2">
              <DialogClose as-child>
                <Button type="button" variant="ghost">Close</Button>
              </DialogClose>
              <Button type="submit" :disabled="body.trim().length === 0">
                Save
              </Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
