<script setup lang="ts">
import { ref } from 'vue'
import { Pencil, Plus, StickyNote, Trash2 } from 'lucide-vue-next'
import type { Bookmark, Note } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

const props = defineProps<{
  bookmarks: Bookmark[]
  notes: Note[]
  currentPage: number
}>()

const emit = defineEmits<{
  jump: [page: number]
  add: [page: number, label: string]
  remove: [id: number]
  'add-note': [page: number]
  'edit-note': [note: Note]
  'remove-note': [id: number]
}>()

const newLabel = ref('')

function onAdd(): void {
  emit('add', props.currentPage, newLabel.value.trim())
  newLabel.value = ''
}

function firstLine(body: string): string {
  const idx = body.indexOf('\n')
  return idx === -1 ? body : body.slice(0, idx)
}
</script>

<template>
  <div class="flex flex-col gap-3 h-full">
    <div>
      <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        Bookmarks
      </h3>
    </div>
    <div class="flex flex-col gap-2">
      <Input
        v-model="newLabel"
        placeholder="Label (optional)"
        @keydown.enter="onAdd"
      />
      <Button type="button" size="sm" @click="onAdd">
        Add at page {{ currentPage }}
      </Button>
    </div>
    <Separator />
    <div v-if="bookmarks.length > 0" class="flex flex-col gap-1 overflow-y-auto">
      <div
        v-for="bm in bookmarks"
        :key="bm.id"
        class="flex items-center gap-2 rounded-md border border-border bg-card p-2"
      >
        <button
          type="button"
          class="flex flex-1 items-center gap-2 text-left hover:opacity-80 focus:outline-none"
          @click="emit('jump', bm.page)"
        >
          <Badge variant="secondary" class="shrink-0">p.{{ bm.page }}</Badge>
          <span class="text-sm break-words">
            {{ bm.label || '(no label)' }}
          </span>
        </button>
        <Button
          variant="ghost"
          size="icon"
          class="shrink-0 text-destructive hover:text-destructive"
          :title="'Delete bookmark'"
          @click="emit('remove', bm.id)"
        >
          <Trash2 class="size-4" />
        </Button>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">No bookmarks yet.</p>

    <Separator />

    <div class="flex items-center justify-between">
      <h3 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        Notes
      </h3>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        class="h-7 w-7"
        :aria-label="`Add note on page ${currentPage}`"
        :title="`Add note on page ${currentPage}`"
        @click="emit('add-note', currentPage)"
      >
        <Plus class="size-4" />
      </Button>
    </div>
    <div v-if="notes.length > 0" class="flex flex-col gap-1 overflow-y-auto">
      <div
        v-for="n in notes"
        :key="n.id"
        class="flex items-center gap-2 rounded-md border border-border bg-card p-2"
      >
        <button
          type="button"
          class="flex flex-1 items-center gap-2 text-left hover:opacity-80 focus:outline-none min-w-0"
          @click="emit('jump', n.page)"
        >
          <Badge variant="secondary" class="shrink-0">p.{{ n.page }}</Badge>
          <StickyNote class="size-4 text-yellow-600 dark:text-yellow-400 shrink-0" />
          <span class="text-sm truncate">
            {{ firstLine(n.body) || '(empty)' }}
          </span>
        </button>
        <Button
          variant="ghost"
          size="icon"
          class="shrink-0"
          title="Edit note"
          @click="emit('edit-note', n)"
        >
          <Pencil class="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          class="shrink-0 text-destructive hover:text-destructive"
          title="Delete note"
          @click="emit('remove-note', n.id)"
        >
          <Trash2 class="size-4" />
        </Button>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">No notes yet.</p>
  </div>
</template>
