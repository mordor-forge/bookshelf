<script setup lang="ts">
import { ChevronDown, ChevronRight } from 'lucide-vue-next'
import { ref } from 'vue'

export interface OutlineNode {
  title: string
  page: number | null
  children: OutlineNode[]
}

defineProps<{ nodes: OutlineNode[] }>()
const emit = defineEmits<{ jump: [page: number] }>()

const expanded = ref<Set<string>>(new Set())

function keyFor(path: string): string {
  return path
}

function toggle(k: string): void {
  const next = new Set(expanded.value)
  if (next.has(k)) next.delete(k)
  else next.add(k)
  expanded.value = next
}

function onClick(node: OutlineNode): void {
  if (node.page !== null) emit('jump', node.page)
}
</script>

<template>
  <div class="flex flex-col gap-0.5 text-sm">
    <template v-for="(node, i) in nodes" :key="i">
      <div class="flex items-center gap-1">
        <button
          v-if="node.children.length > 0"
          type="button"
          class="h-6 w-6 inline-flex items-center justify-center rounded hover:bg-accent shrink-0"
          @click="toggle(keyFor(`${i}`))"
        >
          <ChevronDown v-if="expanded.has(keyFor(`${i}`))" class="size-3.5" />
          <ChevronRight v-else class="size-3.5" />
        </button>
        <span v-else class="w-6 shrink-0" />
        <button
          type="button"
          class="flex-1 text-left px-2 py-1 rounded hover:bg-accent disabled:opacity-50 disabled:hover:bg-transparent truncate"
          :disabled="node.page === null"
          :title="node.title"
          @click="onClick(node)"
        >
          {{ node.title }}
          <span v-if="node.page !== null" class="text-xs text-muted-foreground ml-1">p.{{ node.page }}</span>
        </button>
      </div>
      <div
        v-if="node.children.length > 0 && expanded.has(keyFor(`${i}`))"
        class="pl-5"
      >
        <PdfOutline :nodes="node.children" @jump="(p) => emit('jump', p)" />
      </div>
    </template>
  </div>
</template>
