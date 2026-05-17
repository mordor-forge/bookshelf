<script setup lang="ts">
import { Settings } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import { useTheme } from '@/composables/useTheme'

// initialize theme app-wide (applies .dark class + watches system pref).
useTheme()
</script>

<template>
  <div class="h-screen overflow-hidden bg-background text-foreground flex flex-col">
    <header
      class="border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-40 h-14"
    >
      <div class="h-full w-full flex items-center gap-2 px-4">
        <router-link to="/" class="text-lg font-semibold tracking-tight shrink-0 mr-2">
          Bookshelf
        </router-link>
        <div id="header-actions-left" class="flex items-center gap-1 min-w-0" />
        <div class="flex-1" />
        <div id="header-actions-right" class="flex items-center gap-1 min-w-0 justify-end" />
        <Button as-child variant="ghost" size="icon" aria-label="Settings" title="Settings">
          <router-link to="/settings">
            <Settings class="size-4" />
          </router-link>
        </Button>
      </div>
    </header>

    <main class="flex-1 min-h-0 overflow-hidden">
      <router-view />
    </main>

    <Toaster position="bottom-right" rich-colors />
  </div>
</template>
