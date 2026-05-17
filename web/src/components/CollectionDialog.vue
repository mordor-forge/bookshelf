<script setup lang="ts">
import { ref, watch } from 'vue'
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
import { Label } from '@/components/ui/label'

interface Props {
  open: boolean
  mode: 'create' | 'rename'
  initialName?: string
  parents: Collection[]
  initialParentId?: number | null
  /** in rename mode, the collection being renamed (to filter parent choices) */
  excludeId?: number
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:open': [v: boolean]
  submit: [payload: { name: string; parentId: number | null }]
}>()

const name = ref('')
const parentId = ref<number | null>(null)

watch(
  () => props.open,
  (v) => {
    if (v) {
      name.value = props.initialName ?? ''
      parentId.value = props.initialParentId ?? null
    }
  },
  { immediate: true },
)

const eligibleParents = computedParents()
function computedParents(): typeof props.parents {
  // for create, all manual collections can be parents. for rename we still
  // allow reparent to any manual collection (excluding self).
  return props.parents
}

function submit(): void {
  const trimmed = name.value.trim()
  if (trimmed.length === 0) return
  emit('submit', { name: trimmed, parentId: parentId.value })
}
</script>

<template>
  <DialogRoot :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 grid w-full max-w-md -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6 shadow-lg rounded-lg"
      >
        <div class="flex items-start justify-between">
          <div>
            <DialogTitle class="text-lg font-semibold">
              {{ mode === 'create' ? 'New collection' : 'Rename collection' }}
            </DialogTitle>
            <DialogDescription class="text-sm text-muted-foreground">
              {{ mode === 'create' ? 'Create a manual collection of books.' : 'Update the collection name and parent.' }}
            </DialogDescription>
          </div>
          <DialogClose class="rounded-sm opacity-70 hover:opacity-100 transition-opacity" aria-label="Close">
            <X class="size-4" />
          </DialogClose>
        </div>

        <form class="flex flex-col gap-3" @submit.prevent="submit">
          <div class="flex flex-col gap-1.5">
            <Label for="col-name">Name</Label>
            <Input id="col-name" v-model="name" autofocus placeholder="My collection" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="col-parent">Parent</Label>
            <select
              id="col-parent"
              v-model="parentId"
              class="h-9 rounded-md border border-input bg-transparent px-3 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
            >
              <option :value="null">(top level)</option>
              <option
                v-for="p in eligibleParents"
                :key="p.id"
                :value="p.id"
                :disabled="excludeId === p.id"
              >
                {{ p.name }}
              </option>
            </select>
          </div>
          <div class="flex justify-end gap-2 pt-2">
            <DialogClose as-child>
              <Button type="button" variant="ghost">Cancel</Button>
            </DialogClose>
            <Button type="submit" :disabled="name.trim().length === 0">
              {{ mode === 'create' ? 'Create' : 'Save' }}
            </Button>
          </div>
        </form>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
