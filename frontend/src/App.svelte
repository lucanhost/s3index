<script lang="ts">
  import { onMount } from 'svelte';
  import { listDirectory, getReadme, formatSize } from './lib/api';
  import Breadcrumb from './lib/Breadcrumb.svelte';
  import FileList from './lib/FileList.svelte';
  import SearchModal from './lib/SearchModal.svelte';
  import PreviewModal from './lib/PreviewModal.svelte';
  import MarkdownRenderer from './lib/MarkdownRenderer.svelte';
  import type { FileEntry, FolderEntry, SortKey, SortDir } from './lib/types';

  // ─── State ───────────────────────────────────────────────────────────────────
  let prefix = $state('');
  let loading = $state(false);
  let error = $state<string | null>(null);

  let folders = $state<FolderEntry[]>([]);
  let files = $state<FileEntry[]>([]);
  let readmeContent = $state<string | null>(null);
  let loadingReadme = $state(false);

  let sortKey = $state<SortKey>((localStorage.getItem('s3_sortKey') as SortKey) || 'name');
  let sortDir = $state<SortDir>((localStorage.getItem('s3_sortDir') as SortDir) || 'asc');

  let searchVisible = $state(false);
  let previewFile = $state<FileEntry | null>(null);


  // ── URL helpers ─────────────────────────────────────────────────────────────
  // Preview URL:   /path/to/file?preview  (bare flag, no value)
  // Directory URL: /some/prefix/          (no query)

  function parseCurrentUrl(): { prefix: string; previewKey: string | undefined } {
    const url = new URL(window.location.href);
    const isPreview = url.searchParams.has('preview');
    if (isPreview) {
      const fileKey = url.pathname.replace(/^\//, '');
      const parts = fileKey.split('/');
      parts.pop();
      const dirPrefix = parts.length > 0 ? parts.join('/') + '/' : '';
      return { prefix: dirPrefix, previewKey: fileKey };
    }
    return { prefix: url.pathname.replace(/^\//, ''), previewKey: undefined };
  }

  async function loadDirectory(newPrefix: string, openPreviewKey: string | undefined = undefined) {
    prefix = newPrefix;
    loading = true;
    error = null;
    readmeContent = null;
    files = [];
    folders = [];

    try {
      const data = await listDirectory(newPrefix);
      folders = data.folders;
      files = data.files;

      // Auto-open preview if a key was requested (e.g. from a shared link)
      if (openPreviewKey) {
        const found = data.files.find(f => f.path === openPreviewKey);
        if (found) previewFile = found;
      }

      // Check for README.md
      const hasReadme = data.files.some(f => f.name.toLowerCase() === 'readme.md');
      if (hasReadme) {
        loadingReadme = true;
        readmeContent = await getReadme(newPrefix);
        loadingReadme = false;
      }
    } catch (e: any) {
      error = e.message || 'Failed to load directory';
    } finally {
      loading = false;
    }
  }

  function navigate(path: string) {
    window.history.pushState({}, '', '/' + path);
    previewFile = null;
    loadDirectory(path);
  }

  function openPreview(file: FileEntry) {
    previewFile = file;
    window.history.pushState({}, '', '/' + file.path + '?preview');
  }

  function closePreview() {
    previewFile = null;
    window.history.pushState({}, '', '/' + prefix);
  }


  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = 'asc';
    }
    localStorage.setItem('s3_sortKey', sortKey);
    localStorage.setItem('s3_sortDir', sortDir);
  }



  // Global keyboard shortcut for search
  function handleGlobalKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      searchVisible = true;
    }
  }

  onMount(() => {
    const { prefix: initPrefix, previewKey } = parseCurrentUrl();
    loadDirectory(initPrefix, previewKey);

    window.addEventListener('keydown', handleGlobalKeydown);
    window.addEventListener('popstate', () => {
      const { prefix: p, previewKey: pk } = parseCurrentUrl();
      previewFile = null;
      loadDirectory(p, pk);
    });
    return () => window.removeEventListener('keydown', handleGlobalKeydown);
  });

  // Computed stats
  const totalSize = $derived(files.reduce((s, f) => s + f.size, 0));
  const fileCount = $derived(files.length);
  const folderCount = $derived(folders.length);
</script>

<!-- Search modal -->
<SearchModal
  bind:visible={searchVisible}
  onNavigate={navigate}
  onPreview={openPreview}
/>

<!-- Preview modal -->
<PreviewModal file={previewFile} onClose={closePreview} />

<!-- ── App shell ─────────────────────────────────────────────────────────── -->
<div class="min-h-screen flex flex-col">

  <!-- Navbar -->
  <header class="sticky top-0 z-30 border-b border-white/8 bg-surface-900/80 backdrop-blur-xl">
    <div class="max-w-6xl mx-auto px-4 flex items-center justify-between h-14">
      <!-- Logo -->
      <button
        class="flex items-center gap-2 flex-shrink-0 cursor-pointer hover:opacity-90 transition-opacity text-left"
        onclick={() => navigate('')}
      >
        <div class="w-7 h-7 rounded-lg overflow-hidden border border-purple-500/35 flex items-center justify-center shadow-lg shadow-purple-950/50 bg-black/40">
          <img src="/logo.jpg" alt="Logo" class="w-full h-full object-cover" />
        </div>
        <span class="font-semibold text-white text-sm hidden sm:block">S3 Index</span>
      </button>

      <!-- Actions -->
      <div class="flex items-center gap-2 flex-shrink-0">
        <!-- Search button -->
        <button
          id="search-button"
          class="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/5 border border-white/8 text-slate-400 hover:text-white hover:bg-white/10 transition-all text-xs"
          onclick={() => searchVisible = true}
        >
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <span class="hidden sm:inline">Search</span>
          <kbd class="hidden md:inline px-1.5 py-0.5 text-[10px] font-mono bg-white/10 rounded">⌘K</kbd>
        </button>

      </div>
    </div>
  </header>

  <!-- Main content -->
  <main class="flex-1 max-w-6xl mx-auto w-full px-4 py-5">

    <!-- Breadcrumb -->
    <div class="mb-4">
      <Breadcrumb {prefix} onNavigate={navigate} />
    </div>

    <!-- Error state -->
    {#if error}
      <div class="glass rounded-xl border border-red-500/20 p-6 text-center">
        <p class="text-red-400 font-semibold mb-1">Error loading directory</p>
        <p class="text-slate-400 text-sm mb-4">{error}</p>
        <button
          class="px-4 py-2 rounded-lg bg-red-500/10 text-red-400 hover:bg-red-500/20 border border-red-500/20 transition-all text-sm"
          onclick={() => loadDirectory(prefix)}
        >
          Retry
        </button>
      </div>

    <!-- Loading state -->
    {:else if loading}
      <div class="glass rounded-xl border border-white/8 overflow-hidden">
        <!-- Skeleton header -->
        <div class="px-4 py-3 border-b border-white/5 flex items-center gap-4">
          {#each [120, 80, 100] as w}
            <div class="h-3 rounded bg-white/5 animate-pulse" style="width:{w}px"></div>
          {/each}
        </div>
        {#each [1,2,3,4,5,6,7,8] as _}
          <div class="flex items-center gap-3 px-4 py-3 border-b border-white/[0.03]">
            <div class="w-5 h-5 rounded bg-white/5 animate-pulse flex-shrink-0"></div>
            <div class="h-3 rounded bg-white/5 animate-pulse flex-1" style="max-width:{60 + Math.random() * 200}px"></div>
            <div class="h-3 rounded bg-white/5 animate-pulse w-16 hidden sm:block"></div>
            <div class="h-3 rounded bg-white/5 animate-pulse w-14"></div>
          </div>
        {/each}
      </div>

    <!-- Directory listing -->
    {:else}
      <!-- Stats bar -->
      <div class="flex items-center gap-3 mb-4 flex-wrap text-xs text-slate-500">
        <span>{folderCount} folder{folderCount !== 1 ? 's' : ''}</span>
        <span class="text-slate-700">•</span>
        <span>{fileCount} file{fileCount !== 1 ? 's' : ''}</span>
        {#if fileCount > 0}
          <span class="text-slate-700">•</span>
          <span>{formatSize(totalSize)} total</span>
        {/if}
      </div>

      <!-- File browser card -->
      <div class="glass rounded-xl border border-white/8 overflow-hidden">
        <!-- Sort header -->
        <div class="flex items-center px-4 py-2 border-b border-white/8 text-xs text-slate-500 bg-white/[0.02]">
          <div class="w-5 mr-3 flex-shrink-0"></div>
          <button
            class="flex-1 flex items-center gap-1 hover:text-slate-300 transition-colors cursor-pointer text-left font-medium"
            onclick={() => toggleSort('name')}
          >
            Name
            {#if sortKey === 'name'}
              <span class="text-purple-400">{sortDir === 'asc' ? '↑' : '↓'}</span>
            {/if}
          </button>
          <button
            class="w-24 text-right hover:text-slate-300 transition-colors cursor-pointer hidden sm:block font-medium"
            onclick={() => toggleSort('lastModified')}
          >
            Modified
            {#if sortKey === 'lastModified'}<span class="text-purple-400 ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>{/if}
          </button>
          <button
            class="w-20 text-right hover:text-slate-300 transition-colors cursor-pointer ml-6 font-medium"
            onclick={() => toggleSort('size')}
          >
            Size
            {#if sortKey === 'size'}<span class="text-purple-400 ml-1">{sortDir === 'asc' ? '↑' : '↓'}</span>{/if}
          </button>
          <div class="w-6 ml-2 flex-shrink-0"></div>
        </div>

        <!-- List view -->
        <FileList
          {files}
          {folders}
          onNavigate={navigate}
          onPreview={openPreview}
          {sortKey}
          {sortDir}
        />
      </div>

      <!-- README.md section -->
      {#if loadingReadme}
        <div class="mt-6 glass rounded-xl border border-white/8 p-6">
          <div class="flex items-center gap-2 text-slate-400 text-sm">
            <span class="animate-spin">◌</span> Loading README...
          </div>
        </div>
      {:else if readmeContent}
        <div class="mt-6 glass rounded-xl border border-white/8 overflow-hidden">
          <!-- README header -->
          <div class="px-5 py-3 border-b border-white/8 flex items-center gap-2 bg-white/[0.02]">
            <svg class="text-purple-400" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" />
              <line x1="16" y1="13" x2="8" y2="13" /><line x1="16" y1="17" x2="8" y2="17" />
            </svg>
            <span class="text-sm font-semibold text-slate-300">README.md</span>
          </div>
          <div class="p-6">
            <MarkdownRenderer content={readmeContent} />
          </div>
        </div>
      {/if}
    {/if}

  </main>

  <!-- Footer -->
  <footer class="border-t border-white/5 py-4 text-center text-xs text-slate-600">
    S3 Index &mdash; Powered by Go
  </footer>
</div>
