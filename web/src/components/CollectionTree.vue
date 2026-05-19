<script setup lang="ts">
import { computed } from 'vue'
import {
  ChevronDown,
  ChevronRight,
  Folder,
  FolderGit2,
  Library as LibraryIcon,
  MoreHorizontal,
  Inbox,
} from 'lucide-vue-next'
import {
  DropdownMenuRoot,
  DropdownMenuTrigger,
  DropdownMenuPortal,
  DropdownMenuContent,
  DropdownMenuItem,
} from 'reka-ui'
import type { TreeNode, Selection } from '../composables/useLibrary'
import { useSettings } from '../composables/useSettings'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  nodes: TreeNode[]
  selection: Selection
  totalBooks: number
  uncategorizedCount: number
}>()

const emit = defineEmits<{
  select: [sel: Selection]
  rename: [node: TreeNode]
  remove: [node: TreeNode]
  addChild: [node: TreeNode | null]
}>()

const settings = useSettings()

function toggle(id: number): void {
  if (settings.treeExpanded.has(id)) settings.treeExpanded.delete(id)
  else settings.treeExpanded.add(id)
}

function isSelected(sel: Selection): boolean {
  const cur = props.selection
  if (cur.kind === sel.kind) {
    if (cur.kind === 'collection' && sel.kind === 'collection') {
      return cur.id === sel.id
    }
    return true
  }
  return false
}

function selectNode(node: TreeNode): void {
  emit('select', { kind: 'collection', id: node.id })
}

interface FlatRow {
  node: TreeNode
  depth: number
}
const flat = computed<FlatRow[]>(() => {
  const out: FlatRow[] = []
  function walk(list: TreeNode[], depth: number): void {
    for (const n of list) {
      out.push({ node: n, depth })
      if (settings.treeExpanded.has(n.id)) walk(n.children, depth + 1)
    }
  }
  walk(props.nodes, 0)
  return out
})
</script>

<template>
  <div class="flex flex-col gap-0.5 text-sm">
    <button
      type="button"
      class="flex items-center gap-2 px-2 h-9 rounded-md hover:bg-accent text-left"
      :class="isSelected({ kind: 'all' }) ? 'bg-accent text-accent-foreground' : ''"
      @click="emit('select', { kind: 'all' })"
    >
      <LibraryIcon class="size-4 shrink-0" />
      <span class="flex-1 truncate">All books</span>
      <span class="text-xs text-muted-foreground tabular-nums">{{ totalBooks }}</span>
    </button>
    <button
      type="button"
      class="flex items-center gap-2 px-2 h-9 rounded-md hover:bg-accent text-left"
      :class="isSelected({ kind: 'uncategorized' }) ? 'bg-accent text-accent-foreground' : ''"
      @click="emit('select', { kind: 'uncategorized' })"
    >
      <Inbox class="size-4 shrink-0" />
      <span class="flex-1 truncate">Uncategorized</span>
      <span class="text-xs text-muted-foreground tabular-nums">{{ uncategorizedCount }}</span>
    </button>

    <div class="h-px bg-border my-1" />

    <div
      v-for="row in flat"
      :key="row.node.id"
      class="group flex items-center gap-1 rounded-md hover:bg-accent h-9 pr-1"
      :class="isSelected({ kind: 'collection', id: row.node.id }) ? 'bg-accent text-accent-foreground' : ''"
      :style="{ paddingLeft: `${row.depth * 12 + 4}px` }"
    >
      <button
        v-if="row.node.children.length > 0"
        type="button"
        class="h-6 w-6 inline-flex items-center justify-center rounded hover:bg-background/60 shrink-0"
        :aria-label="settings.treeExpanded.has(row.node.id) ? 'Collapse' : 'Expand'"
        @click="toggle(row.node.id)"
      >
        <ChevronDown v-if="settings.treeExpanded.has(row.node.id)" class="size-3.5" />
        <ChevronRight v-else class="size-3.5" />
      </button>
      <span v-else class="w-6 shrink-0" />

      <button
        type="button"
        class="flex items-center gap-2 flex-1 min-w-0 text-left h-full"
        @click="selectNode(row.node)"
      >
        <FolderGit2 v-if="row.node.source === 'scan'" class="size-4 shrink-0 text-muted-foreground" />
        <Folder v-else class="size-4 shrink-0 text-primary" />
        <span class="flex-1 truncate">{{ row.node.name }}</span>
        <span class="text-xs text-muted-foreground tabular-nums">{{ row.node.bookCount }}</span>
      </button>

      <DropdownMenuRoot>
        <DropdownMenuTrigger as-child>
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7 opacity-0 group-hover:opacity-100 data-[state=open]:opacity-100"
            :aria-label="`Actions for ${row.node.name}`"
          >
            <MoreHorizontal class="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuPortal>
          <DropdownMenuContent
            class="z-50 min-w-[10rem] rounded-md border border-border bg-popover p-1 text-popover-foreground shadow-md"
            :side-offset="4"
            align="end"
          >
            <DropdownMenuItem
              class="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-accent data-[disabled]:opacity-50 data-[disabled]:pointer-events-none"
              @select="emit('addChild', row.node)"
            >
              New sub-collection
            </DropdownMenuItem>
            <DropdownMenuItem
              class="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-accent"
              @select="emit('rename', row.node)"
            >
              Rename
            </DropdownMenuItem>
            <DropdownMenuItem
              class="relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm text-destructive outline-none data-[highlighted]:bg-accent"
              @select="emit('remove', row.node)"
            >
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </div>
  </div>
</template>
