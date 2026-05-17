<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ArrowLeft, RefreshCw } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { getScan, getSettings, postScan, putSettings } from '../api'
import type { ScanStatus } from '../types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useTheme } from '@/composables/useTheme'

const { theme, setTheme } = useTheme()

function onThemeChange(v: unknown): void {
  if (v === 'system' || v === 'light' || v === 'dark') {
    setTheme(v)
  }
}

const libraryDir = ref('')
const initialLibraryDir = ref('')
const loading = ref(true)
const saving = ref(false)
const scanStatus = ref<ScanStatus | null>(null)
const scanning = ref(false)
let pollHandle: ReturnType<typeof setInterval> | null = null

async function loadSettings(): Promise<void> {
  try {
    const s = await getSettings()
    libraryDir.value = s.libraryDir
    initialLibraryDir.value = s.libraryDir
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to load settings')
  } finally {
    loading.value = false
  }
}

async function loadScan(): Promise<void> {
  try {
    scanStatus.value = await getScan()
  } catch {
    scanStatus.value = null
  }
}

async function onSave(): Promise<void> {
  saving.value = true
  try {
    const s = await putSettings(libraryDir.value.trim())
    libraryDir.value = s.libraryDir
    initialLibraryDir.value = s.libraryDir
    toast.success('Settings saved')
    await loadScan()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : 'Failed to save settings')
  } finally {
    saving.value = false
  }
}

function startPolling(): void {
  if (pollHandle !== null) return
  pollHandle = setInterval(async () => {
    try {
      const status = await getScan()
      scanStatus.value = status
      if (!status.running) {
        stopPolling()
        scanning.value = false
        toast.success('Scan complete')
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
    if ('conflict' in res) {
      toast.info('Scan already in progress')
    } else {
      toast.success('Scan started')
    }
    startPolling()
  } catch (err) {
    scanning.value = false
    toast.error(err instanceof Error ? err.message : 'Scan failed')
  }
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadScan()])
})
onUnmounted(stopPolling)
</script>

<template>
  <Teleport defer to="#header-actions-left">
    <Button as-child variant="ghost" size="sm">
      <router-link to="/">
        <ArrowLeft class="size-4 mr-1" /> Back
      </router-link>
    </Button>
  </Teleport>
  <div class="max-w-3xl mx-auto px-6 py-6">
    <h1 class="text-2xl font-semibold tracking-tight mb-6">Settings</h1>

    <Card class="mb-6">
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
      </CardHeader>
      <CardContent>
        <RadioGroup
          :model-value="theme"
          class="grid grid-cols-1 sm:grid-cols-3 gap-3"
          @update:model-value="onThemeChange"
        >
          <Label
            v-for="opt in (['system', 'light', 'dark'] as const)"
            :key="opt"
            :for="`theme-${opt}`"
            class="flex items-center gap-3 rounded-md border border-border bg-card p-3 cursor-pointer hover:bg-accent/40"
          >
            <RadioGroupItem :id="`theme-${opt}`" :value="opt" />
            <span class="capitalize">{{ opt }}</span>
          </Label>
        </RadioGroup>
        <p class="text-xs text-muted-foreground mt-3">
          Stored on this device only.
        </p>
      </CardContent>
    </Card>

    <Card class="mb-6">
      <CardHeader>
        <CardTitle>Library</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="space-y-2">
          <Label for="libraryDir">Library directory</Label>
          <Input
            id="libraryDir"
            v-model="libraryDir"
            type="text"
            class="font-mono"
            placeholder="/srv/books"
            :disabled="loading || saving"
          />
          <p class="text-xs text-muted-foreground">
            Absolute path on the server. Read-only mount recommended.
          </p>
        </div>
        <div class="flex justify-end">
          <Button
            type="button"
            :disabled="loading || saving || libraryDir === initialLibraryDir"
            @click="onSave"
          >
            {{ saving ? 'Saving…' : 'Save' }}
          </Button>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>Current state</CardTitle>
      </CardHeader>
      <CardContent class="space-y-4">
        <p class="text-sm">
          <span class="text-muted-foreground">Library:</span>
          {{ initialLibraryDir ? 'configured' : 'not configured' }}
        </p>
        <Separator />
        <div v-if="scanStatus" class="text-sm text-muted-foreground space-y-1">
          <p>Last scan finished: {{ scanStatus.finishedAt ?? 'never' }}</p>
          <p>
            Added {{ scanStatus.added }}, updated {{ scanStatus.updated }},
            removed {{ scanStatus.removed }}.
          </p>
          <p v-if="scanStatus.error" class="text-destructive">
            Error: {{ scanStatus.error }}
          </p>
        </div>
        <div class="flex justify-end">
          <Button
            type="button"
            variant="outline"
            :disabled="!initialLibraryDir || scanning"
            @click="onRescan"
          >
            <RefreshCw class="size-4 mr-2" :class="scanning ? 'animate-spin' : ''" />
            {{ scanning ? 'Scanning…' : 'Trigger rescan' }}
          </Button>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
