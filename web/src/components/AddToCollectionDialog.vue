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
import { X } from 'lucide-vue-next'
import type { Collection } from '../types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps<{
  open: boolean
  collections: Collection[]
  bookPath: string
  bookTitle: string
}>()

const emit = defineEmits<{
  'update:open': [v: boolean]
  pick: [collectionId: number]
}>()

const search = ref('')

watch(
  () => props.open,
  (v) => {
    if (v) search.value = ''
  },
)

const sorted = computed(() =>
  [...props.collections]
    .filter((c) => c.source === 'manual')
    .sort((a, b) => {
    return a.name.localeCompare(b.name)
  }),
)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (q.length === 0) return sorted.value
  return sorted.value.filter((c) => c.name.toLowerCase().includes(q))
})
</script>

<template>
  <DialogRoot :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 grid w-full max-w-md -translate-x-1/2 -translate-y-1/2 gap-3 border bg-background p-6 shadow-lg rounded-lg"
      >
        <div class="flex items-start justify-between">
          <div>
            <DialogTitle class="text-lg font-semibold">Add to collection</DialogTitle>
            <DialogDescription class="text-sm text-muted-foreground truncate max-w-[20rem]">
              {{ bookTitle }}
            </DialogDescription>
          </div>
          <DialogClose class="rounded-sm opacity-70 hover:opacity-100" aria-label="Close">
            <X class="size-4" />
          </DialogClose>
        </div>

        <Input v-model="search" placeholder="Search collections…" />

        <div class="flex flex-col gap-1 max-h-72 overflow-y-auto">
          <button
            v-for="c in filtered"
            :key="c.id"
            type="button"
            class="flex items-center justify-between gap-2 text-left px-3 py-2 rounded-md hover:bg-accent text-sm"
            @click="emit('pick', c.id)"
          >
            <span class="truncate">{{ c.name }}</span>
          </button>
          <p
            v-if="filtered.length === 0"
            class="text-sm text-muted-foreground px-3 py-2"
          >
            No collections.
          </p>
        </div>

        <div class="flex justify-end pt-1">
          <DialogClose as-child>
            <Button type="button" variant="ghost">Close</Button>
          </DialogClose>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
