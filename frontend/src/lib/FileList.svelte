<script lang="ts">
  import { getCategory, getCategoryColor, formatSize, relativeTime, getObjectUrl } from './api';
  import FileIcon from './FileIcon.svelte';
  import type { FileEntry, FolderEntry, SortKey, SortDir } from './types';

  export let files: FileEntry[] = [];
  export let folders: FolderEntry[] = [];
  export let onNavigate: (path: string) => void;
  export let onPreview: (file: FileEntry) => void;
  export let sortKey: SortKey = 'name';
  export let sortDir: SortDir = 'asc';

  $: sortedFolders = [...folders].sort((a, b) => {
    const dir = sortDir === 'asc' ? 1 : -1;
    if (sortKey === 'name') return dir * a.name.localeCompare(b.name);
    return 0;
  });

  $: sortedFiles = [...files].sort((a, b) => {
    const dir = sortDir === 'asc' ? 1 : -1;
    if (sortKey === 'name') return dir * a.name.localeCompare(b.name);
    if (sortKey === 'size') return dir * (a.size - b.size);
    if (sortKey === 'lastModified') return dir * (new Date(a.lastModified).getTime() - new Date(b.lastModified).getTime());
    return 0;
  });
</script>

<div class="divide-y divide-white/5">
  {#each sortedFolders as folder (folder.path)}
    <button
      class="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-white/[0.04] transition-colors duration-150 group cursor-pointer text-left"
      onclick={() => onNavigate(folder.path)}
    >
      <!-- Icon -->
      <div class="flex-shrink-0 text-yellow-400/80 group-hover:text-yellow-300 transition-colors">
        <FileIcon category="folder" size={18} />
      </div>

      <!-- Name -->
      <span class="flex-1 text-sm text-slate-200 group-hover:text-white transition-colors font-medium truncate">
        {folder.name}
      </span>

      <!-- Arrow indicator -->
      <svg class="text-slate-600 group-hover:text-slate-400 transition-colors flex-shrink-0" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <polyline points="9 18 15 12 9 6" />
      </svg>
    </button>
  {/each}

  {#each sortedFiles as file (file.path)}
    {@const cat = getCategory(file.name)}
    {@const colorClass = getCategoryColor(cat)}
    <div
      class="flex items-center gap-3 px-4 py-2.5 hover:bg-white/[0.04] transition-colors duration-150 group"
    >
      <!-- Icon -->
      <div class="flex-shrink-0 {colorClass} opacity-80 group-hover:opacity-100 transition-opacity">
        <FileIcon category={cat} size={18} />
      </div>

      <!-- Name -->
      <button
        class="flex-1 text-sm text-slate-300 group-hover:text-white transition-colors text-left truncate cursor-pointer hover:text-purple-300"
        onclick={() => onPreview(file)}
        title={file.name}
      >
        {file.name}
      </button>

      <!-- Metadata -->
      <div class="flex items-center gap-6 flex-shrink-0">
        <span class="text-xs text-slate-500 hidden sm:block w-24 text-right" title={file.lastModified}>
          {relativeTime(file.lastModified)}
        </span>
        <span class="text-xs text-slate-400 font-mono w-20 text-right">
          {formatSize(file.size)}
        </span>
      </div>

      <!-- Download link -->
      <a
        href={getObjectUrl(file.path)}
        download={file.name}
        class="flex-shrink-0 text-slate-600 hover:text-purple-400 transition-colors p-1 rounded opacity-0 group-hover:opacity-100"
        title="Download"
        onclick={(e) => e.stopPropagation()}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
          <polyline points="7 10 12 15 17 10" />
          <line x1="12" y1="15" x2="12" y2="3" />
        </svg>
      </a>
    </div>
  {/each}

  {#if sortedFolders.length === 0 && sortedFiles.length === 0}
    <div class="flex flex-col items-center justify-center py-16 text-slate-500">
      <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" class="mb-3 opacity-30">
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
      </svg>
      <p class="text-sm">This folder is empty</p>
    </div>
  {/if}
</div>
