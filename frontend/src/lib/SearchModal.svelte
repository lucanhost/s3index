<script lang="ts">
  import { searchFiles, getCategory, getCategoryColor, formatSize } from './api';
  import FileIcon from './FileIcon.svelte';
  import type { FileEntry, FolderEntry } from './types';

  export let visible: boolean = false;
  export let onNavigate: (path: string) => void;
  export let onPreview: (file: FileEntry) => void;

  let query = '';
  let inputEl: HTMLInputElement;
  // Display state
  let resultFiles: FileEntry[] = [];
  let resultFolders: FolderEntry[] = [];
  let searchState: 'idle' | 'searching' | 'done' = 'idle';
  let debounceTimer: ReturnType<typeof setTimeout>;

  function close() {
    visible = false;
    query = '';
    resultFiles = [];
    resultFolders = [];
    searchState = 'idle';
  }

  async function doSearch(q: string) {
    if (q.length === 0) {
      searchState = 'idle';
      resultFiles = [];
      resultFolders = [];
      return;
    }
    try {
      // searchState already set to 'searching' in onInput
      const res = await searchFiles(q);
      resultFiles = res.files;
      resultFolders = res.folders;
    } finally {
      searchState = 'done';  // Show results (or "No results")
    }
  }

  function onInput() {
    clearTimeout(debounceTimer);
    // Set searching state immediately to prevent "No results" flash
    if (query.length > 0) {
      searchState = 'searching';
    }
    debounceTimer = setTimeout(() => doSearch(query), 300);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  $: if (visible && inputEl) setTimeout(() => inputEl?.focus(), 50);

  $: hasResults = resultFiles.length > 0 || resultFolders.length > 0;
  $: totalResults = resultFiles.length + resultFolders.length;
</script>

{#if visible}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-start justify-center pt-16 px-4"
    onclick={close}
    onkeydown={handleKeydown}
    role="dialog"
    aria-modal="true"
    aria-label="Global search"
    tabindex="-1"
  >
    <div
      class="w-full max-w-2xl glass rounded-2xl shadow-2xl border border-white/10 overflow-hidden"
      onclick={(e) => e.stopPropagation()}
      onkeydown={handleKeydown}
      role="presentation"
    >
      <!-- Input -->
      <div class="flex items-center gap-3 px-4 py-3.5 border-b border-white/8">
        {#if searchState === 'searching'}
          <svg class="text-purple-400 flex-shrink-0 animate-spin" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
          </svg>
        {:else}
          <svg class="text-slate-400 flex-shrink-0" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
        {/if}
        <input
          bind:this={inputEl}
          bind:value={query}
          oninput={onInput}
          type="text"
          placeholder="Search all files and folders in bucket..."
          class="flex-1 bg-transparent text-white placeholder-slate-500 outline-none text-sm"
        />
        <kbd class="px-2 py-0.5 text-xs text-slate-500 border border-slate-700 rounded font-mono">Esc</kbd>
      </div>

      <!-- Results -->
      <div class="max-h-[60vh] overflow-y-auto">
        {#if query.length === 0}
          <div class="flex flex-col items-center py-10 gap-2 text-slate-500">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" class="opacity-30">
              <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <p class="text-sm">Search across all files and folders in bucket</p>
          </div>

        {:else if searchState === 'searching'}
          <div class="flex items-center justify-center py-10 text-slate-500 text-sm">Searching…</div>

        {:else if !hasResults}
          <div class="flex items-center justify-center py-10 text-slate-500 text-sm">
            No results for "<span class="text-slate-300">{query}</span>"
          </div>

        {:else}
          <div class="px-2 py-2 space-y-1">
            <p class="text-xs text-slate-500 px-2 pb-0.5">{totalResults} result{totalResults !== 1 ? 's' : ''}</p>

            <!-- Folders -->
            {#if resultFolders.length > 0}
              <p class="text-[11px] text-slate-600 uppercase tracking-widest font-semibold px-2 pt-1">Folders</p>
              {#each resultFolders as folder (folder.path)}
                <button
                  class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl hover:bg-white/5 transition-colors cursor-pointer text-left group"
                  onclick={() => { onNavigate(folder.path); close(); }}
                >
                  <div class="flex-shrink-0 text-yellow-400">
                    <FileIcon category="folder" size={16} />
                  </div>
                  <div class="flex-1 min-w-0">
                    <span class="text-sm text-slate-200 group-hover:text-white font-medium">{folder.name}/</span>
                    <span class="block text-xs text-slate-500 truncate">/{folder.path}</span>
                  </div>
                  <svg class="text-slate-600 group-hover:text-slate-400 flex-shrink-0" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </button>
              {/each}
            {/if}

            <!-- Files -->
            {#if resultFiles.length > 0}
              <p class="text-[11px] text-slate-600 uppercase tracking-widest font-semibold px-2 pt-2">Files</p>
              {#each resultFiles as file (file.path)}
                {@const cat = getCategory(file.name)}
                {@const colorClass = getCategoryColor(cat)}
                <div class="flex items-center gap-3 px-3 py-2.5 rounded-xl hover:bg-white/5 transition-colors group">
                  <div class="flex-shrink-0 {colorClass}">
                    <FileIcon category={cat} size={16} />
                  </div>
                  <div class="flex-1 min-w-0">
                    <button
                      class="block text-sm text-slate-200 hover:text-purple-300 text-left font-medium truncate w-full cursor-pointer transition-colors"
                      onclick={() => { onPreview(file); close(); }}
                    >
                      {file.name}
                    </button>
                    <!-- Click path to navigate to parent folder -->
                    <button
                      class="text-xs text-slate-500 hover:text-slate-300 text-left truncate w-full cursor-pointer transition-colors"
                      onclick={() => {
                        const parts = file.path.split('/');
                        parts.pop();
                        onNavigate(parts.length > 0 ? parts.join('/') + '/' : '');
                        close();
                      }}
                    >/{file.path}</button>
                  </div>
                  <span class="text-xs text-slate-500 font-mono flex-shrink-0">{formatSize(file.size)}</span>
                </div>
              {/each}
            {/if}
          </div>
        {/if}
      </div>

      <!-- Footer -->
      <div class="border-t border-white/5 px-4 py-2 flex items-center gap-4 text-xs text-slate-500">
        <span><kbd class="font-mono">↵</kbd> preview file</span>
        <span>click path → open folder</span>
        <span><kbd class="font-mono">Esc</kbd> close</span>
      </div>
    </div>
  </div>
{/if}
