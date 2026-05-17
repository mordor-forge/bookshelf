<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  BookOpen,
  BookOpenCheck,
  BookText,
  Eye,
  EyeOff,
  MoreHorizontal,
  FolderMinus,
  FolderPlus,
} from 'lucide-vue-next'
import {
  DropdownMenuRoot,
  DropdownMenuTrigger,
  DropdownMenuPortal,
  DropdownMenuContent,
  DropdownMenuItem,
} from 'reka-ui'
import type { BookSummary, Status } from '../types'
import { putBookHidden, putBookStatus } from '../api'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getCover, revokeCover } from '../composables/useCover'

const props = defineProps<{
  book: BookSummary
  /** when viewing a specific manual collection, allow removal */
  currentCollectionId?: number | null
  currentCollectionIsManual?: boolean
}>()

const emit = defineEmits<{
  statusChanged: [path: string, status: Status, currentlyReading: boolean]
  hiddenChanged: [path: string, hidden: boolean]
  addToCollection: [book: BookSummary]
  removeFromCollection: [book: BookSummary, collectionId: number]
}>()

const percent = computed(() =>
  props.book.progress ? Math.round(props.book.progress.percent) : 0,
)

const lastRead = computed(() => formatRelative(props.book.progress?.lastReadAt ?? null))

function formatRelative(iso: string | null): string {
  if (!iso) return 'never opened'
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return iso
  const diff = Date.now() - then
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} day${days === 1 ? '' : 's'} ago`
  return new Date(iso).toLocaleDateString()
}

const readTo = computed(
  () => `/read/${props.book.path.split('/').map(encodeURIComponent).join('/')}`,
)

const isReading = computed(() => props.book.status === 'currently_reading')
const isCompleted = computed(() => props.book.status === 'completed')
const toggling = ref(false)

const coverUrl = ref<string | null>(null)
const coverLoading = ref(false)
const coverFailed = ref(false)
const coverRoot = shallowRef<HTMLElement | null>(null)
let coverObserver: IntersectionObserver | null = null
let coverRequested = false

async function loadCover(): Promise<void> {
  if (coverRequested) return
  coverRequested = true
  coverLoading.value = true
  try {
    const url = await getCover({
      path: props.book.path,
      fingerprint: props.book.fingerprint ?? '',
    })
    if (url) {
      coverUrl.value = url
    } else {
      coverFailed.value = true
    }
  } catch {
    coverFailed.value = true
  } finally {
    coverLoading.value = false
  }
}

function setCoverRoot(el: Element | null): void {
  if (el instanceof HTMLElement) {
    coverRoot.value = el
    coverObserver?.disconnect()
    coverObserver = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) {
            void loadCover()
            coverObserver?.disconnect()
            coverObserver = null
            return
          }
        }
      },
      { rootMargin: '600px 0px 600px 0px' },
    )
    coverObserver.observe(el)
  }
}

// if path changes (e.g. card reused), reset.
watch(
  () => props.book.path,
  () => {
    if (coverUrl.value) revokeCover(coverUrl.value)
    coverUrl.value = null
    coverFailed.value = false
    coverRequested = false
    if (coverRoot.value) {
      coverObserver?.disconnect()
      coverObserver = new IntersectionObserver(
        (entries) => {
          for (const e of entries) {
            if (e.isIntersecting) {
              void loadCover()
              coverObserver?.disconnect()
              coverObserver = null
              return
            }
          }
        },
        { rootMargin: '600px 0px 600px 0px' },
      )
      coverObserver.observe(coverRoot.value)
    }
  },
)

onBeforeUnmount(() => {
  coverObserver?.disconnect()
  coverObserver = null
  if (coverUrl.value) revokeCover(coverUrl.value)
  coverUrl.value = null
})

const hiding = ref(false)

async function toggleHidden(): Promise<void> {
  if (hiding.value) return
  hiding.value = true
  const target = !props.book.hidden
  // optimistic update.
  emit('hiddenChanged', props.book.path, target)
  try {
    await putBookHidden(props.book.path, target)
  } catch (err) {
    emit('hiddenChanged', props.book.path, !target)
    toast.error(err instanceof Error ? err.message : 'Failed to update visibility')
  } finally {
    hiding.value = false
  }
}

async function toggleReading(e: Event): Promise<void> {
  e.preventDefault()
  e.stopPropagation()
  if (toggling.value) return
  toggling.value = true
  const target = !isReading.value
  try {
    const progress = await putBookStatus(props.book.path, target)
    const newStatus = progress.status ?? props.book.status
    emit(
      'statusChanged',
      props.book.path,
      newStatus,
      progress.currentlyReading ?? target,
    )
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to update status')
  } finally {
    toggling.value = false
  }
}
</script>

<template>
  <div class="relative group h-full">
    <router-link :to="readTo" class="block h-full focus:outline-none">
      <Card
        class="flex h-full flex-col overflow-hidden transition-colors group-hover:border-foreground/40 group-focus-visible:ring-2 group-focus-visible:ring-ring"
      >
        <div
          :ref="(el) => setCoverRoot(el as Element | null)"
          class="aspect-[3/4] w-full bg-muted overflow-hidden border-b border-border"
        >
          <img
            v-if="coverUrl"
            :src="coverUrl"
            alt=""
            loading="lazy"
            class="object-cover w-full h-full"
          />
          <div
            v-else-if="coverFailed"
            class="flex h-full w-full items-center justify-center text-muted-foreground"
          >
            <BookText class="size-10" />
          </div>
          <Skeleton v-else class="h-full w-full rounded-none" />
        </div>
        <div class="flex flex-1 flex-col gap-2 p-3">
          <div
            class="text-sm font-medium leading-snug break-words line-clamp-2 min-h-[2.4rem]"
            :title="book.title"
          >
            {{ book.title }}
          </div>
          <div v-if="book.bookmarkCount > 0" class="flex min-h-[1.25rem] flex-wrap items-center gap-1.5">
            <Badge variant="outline" class="text-[10px] px-1.5 py-0">
              {{ book.bookmarkCount }} bookmark{{ book.bookmarkCount === 1 ? '' : 's' }}
            </Badge>
          </div>
          <div class="mt-auto flex flex-col gap-1">
            <div class="flex items-center gap-2">
              <Progress
                :model-value="percent"
                :class="['h-1 flex-1', isCompleted && '[&>div]:bg-emerald-500']"
              />
              <span
                class="text-[10px] tabular-nums w-8 text-right"
                :class="isCompleted ? 'text-emerald-600 dark:text-emerald-400 font-medium' : 'text-muted-foreground'"
              >
                {{ percent }}%
              </span>
            </div>
            <div class="text-[10px] text-muted-foreground truncate">{{ lastRead }}</div>
          </div>
        </div>
      </Card>
    </router-link>

    <div class="absolute top-1.5 right-1.5 flex items-center gap-0.5 rounded-md bg-background/70 backdrop-blur-sm shadow-sm">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        class="h-7 w-7"
        :aria-label="isReading ? 'Mark not currently reading' : 'Mark currently reading'"
        :title="isReading ? 'Mark not currently reading' : 'Mark currently reading'"
        :disabled="toggling"
        @click="toggleReading"
      >
        <BookOpenCheck v-if="isReading" class="size-4 text-primary" />
        <BookOpen v-else class="size-4 text-muted-foreground" />
      </Button>
      <DropdownMenuRoot>
        <DropdownMenuTrigger as-child>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            aria-label="Book actions"
            title="Book actions"
            @click.prevent.stop
          >
            <MoreHorizontal class="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuPortal>
          <DropdownMenuContent
            class="z-50 min-w-[12rem] rounded-md border border-border bg-popover p-1 text-popover-foreground shadow-md"
            :side-offset="4"
            align="end"
          >
            <DropdownMenuItem
              class="relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-accent"
              @select="emit('addToCollection', book)"
            >
              <FolderPlus class="size-4" /> Add to collection…
            </DropdownMenuItem>
            <DropdownMenuItem
              class="relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-accent"
              :disabled="hiding"
              @select="toggleHidden"
            >
              <Eye v-if="book.hidden" class="size-4" />
              <EyeOff v-else class="size-4" />
              {{ book.hidden ? 'Unhide' : 'Hide' }}
            </DropdownMenuItem>
            <DropdownMenuItem
              v-if="currentCollectionId !== null && currentCollectionId !== undefined && currentCollectionIsManual"
              class="relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive outline-none data-[highlighted]:bg-accent"
              @select="emit('removeFromCollection', book, currentCollectionId)"
            >
              <FolderMinus class="size-4" /> Remove from this collection
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenuPortal>
      </DropdownMenuRoot>
    </div>
  </div>
</template>
