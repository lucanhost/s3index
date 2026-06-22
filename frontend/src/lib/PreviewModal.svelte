<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getObjectUrl, getFileInfo, formatSize, formatDate, getCategory } from './api';
  import FileIcon from './FileIcon.svelte';
  import MarkdownRenderer from './MarkdownRenderer.svelte';
  import type { FileEntry, FileInfo } from './types';

  export let file: FileEntry | null = null;
  export let onClose: () => void;

  let info: FileInfo | null = null;
  let loadingInfo = false;
  let textContent: string | null = null;
  let loadingText = false;
  let copied = false;
  let copiedDownload = false;

  function copyToClipboard(text: string) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).catch(err => {
        console.error("Clipboard API failed, using fallback:", err);
        fallbackCopyToClipboard(text);
      });
    } else {
      fallbackCopyToClipboard(text);
    }
  }

  // Fallback for insecure contexts (HTTP) or unsupported browsers
  function fallbackCopyToClipboard(text: string) {
    const textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.top = "0";
    textArea.style.left = "0";
    textArea.style.position = "fixed";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
      document.execCommand("copy");
    } catch (err) {
      console.error("Fallback copy failed:", err);
    }
    document.body.removeChild(textArea);
  }

  function copyShareLink() {
    if (!file) return;
    const shareUrl = window.location.origin + '/' + file.path;
    copyToClipboard(shareUrl);
    copied = true;
    setTimeout(() => { copied = false; }, 2000);
  }

  function copyDownloadLink() {
    if (!file) return;
    const downloadUrl = window.location.origin + getObjectUrl(file.path);
    copyToClipboard(downloadUrl);
    copiedDownload = true;
    setTimeout(() => { copiedDownload = false; }, 2000);
  }

  $: category = file ? getCategory(file.name) : '';
  $: objectUrl = file ? getObjectUrl(file.path) : '';

  $: if (file) {
    loadInfo(file);
  } else {
    info = null;
    textContent = null;
  }

  async function loadInfo(f: FileEntry) {
    loadingInfo = true;
    info = null;
    textContent = null;
    try {
      info = await getFileInfo(f.path);
      // For text/code/markdown, load content
      if (['markdown', 'code', 'text'].includes(getCategory(f.name, info.contentType))) {
        loadingText = true;
        try {
          const res = await fetch(getObjectUrl(f.path));
          textContent = await res.text();
        } finally {
          loadingText = false;
        }
      }
    } finally {
      loadingInfo = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }

  onMount(() => window.addEventListener('keydown', handleKeydown));
  onDestroy(() => window.removeEventListener('keydown', handleKeydown));
</script>

{#if file}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-40 bg-black/70 backdrop-blur-md flex items-center justify-center p-4"
    onclick={onClose}
    onkeydown={(e) => e.key === 'Escape' && onClose()}
    role="dialog"
    aria-modal="true"
    aria-label="File preview"
    tabindex="-1"
  >
    <!-- Panel -->
    <div
      class="glass rounded-2xl border border-white/10 shadow-2xl w-full max-w-4xl max-h-[90vh] flex flex-col overflow-hidden"
      onclick={(e) => e.stopPropagation()}
      role="presentation"
    >
      <!-- Header -->
      <div class="flex items-center gap-3 px-5 py-3.5 border-b border-white/8 flex-shrink-0">
        <div class="text-purple-400">
          <FileIcon category={category} size={20} />
        </div>
        <div class="flex-1 min-w-0">
          <h2 class="text-white font-semibold text-sm truncate">{file.name}</h2>
          <p class="text-xs text-slate-500 truncate font-mono">{file.path}</p>
        </div>
        <div class="flex items-center gap-2 flex-shrink-0">
          <!-- Download button -->
          <a
            href={objectUrl}
            download={file.name}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-purple-600/20 text-purple-300 hover:bg-purple-600/40 border border-purple-500/20 hover:border-purple-400/40 transition-all text-xs font-medium"
          >
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7 10 12 15 17 10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
            Download
          </a>
          <!-- Copy download URL button -->
          <button
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white/5 text-slate-300 hover:bg-white/10 border border-white/8 transition-all text-xs font-medium"
            onclick={copyDownloadLink}
            title="Copy direct download URL of this file"
          >
            {#if copiedDownload}
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#86efac" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              <span class="text-green-300">Copied URL!</span>
            {:else}
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
              Copy URL
            {/if}
          </button>
          <!-- Share link -->
          <button
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-white/5 text-slate-300 hover:bg-white/10 border border-white/8 transition-all text-xs font-medium"
            onclick={copyShareLink}
            title="Copy shareable link"
          >
            {#if copied}
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#86efac" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              <span class="text-green-300">Copied!</span>
            {:else}
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
              </svg>
              Share
            {/if}
          </button>
          <!-- Close -->
          <button
            class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/10 transition-all"
            onclick={onClose}
            aria-label="Close preview"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Content area -->
      <div class="flex-1 overflow-auto min-h-0">
        <!-- Image preview -->
        {#if category === 'image'}
          <div class="flex items-center justify-center p-6 bg-black/30 min-h-64">
            <img
              src={objectUrl}
              alt={file.name}
              class="max-w-full max-h-[60vh] object-contain rounded-lg shadow-xl"
            />
          </div>

        <!-- Video preview -->
        {:else if category === 'video'}
          <div class="bg-black p-4">
            <!-- svelte-ignore a11y-media-has-caption -->
            <video
              src={objectUrl}
              controls
              class="w-full max-h-[55vh] rounded-lg"
              preload="metadata"
            ></video>
          </div>

        <!-- Audio preview -->
        {:else if category === 'audio'}
          <div class="flex flex-col items-center justify-center p-12 gap-6">
            <div class="w-20 h-20 rounded-full bg-orange-400/10 border border-orange-400/20 flex items-center justify-center">
              <FileIcon category="audio" size={36} className="text-orange-400" />
            </div>
            <audio src={objectUrl} controls class="w-full max-w-md" preload="metadata"></audio>
          </div>

        <!-- PDF preview -->
        {:else if category === 'pdf'}
          <iframe
            src={objectUrl}
            title={file.name}
            class="w-full h-full min-h-[60vh] border-0"
          ></iframe>

        <!-- Markdown preview -->
        {:else if category === 'markdown'}
          <div class="p-6">
            {#if loadingText}
              <div class="flex items-center gap-2 text-slate-400 text-sm py-8 justify-center">
                <span class="animate-spin">◌</span> Loading...
              </div>
            {:else if textContent !== null}
              <MarkdownRenderer content={textContent} />
            {/if}
          </div>

        <!-- Code preview -->
        {:else if category === 'code' || category === 'text'}
          <div class="p-4">
            {#if loadingText}
              <div class="flex items-center gap-2 text-slate-400 text-sm py-8 justify-center">
                <span class="animate-spin">◌</span> Loading...
              </div>
            {:else if textContent !== null}
              <pre class="bg-black/40 rounded-lg p-4 overflow-x-auto border border-white/8 text-sm"><code class="text-slate-200 font-mono text-xs leading-relaxed">{textContent}</code></pre>
            {/if}
          </div>

        <!-- Generic file -->
        {:else}
          <div class="flex flex-col items-center justify-center py-16 gap-3">
            <div class="text-slate-500 opacity-50">
              <FileIcon category={category} size={56} />
            </div>
            <p class="text-slate-400 text-sm">No preview available</p>
          </div>
        {/if}
      </div>

      <!-- Footer: file metadata -->
      <div class="border-t border-white/8 px-5 py-3 flex-shrink-0 flex flex-wrap gap-4 text-xs text-slate-500">
        {#if loadingInfo}
          <span class="animate-pulse text-slate-600">Loading metadata...</span>
        {:else if info}
          <span><span class="text-slate-400 font-medium">Size:</span> {formatSize(info.size)}</span>
          <span><span class="text-slate-400 font-medium">Type:</span> {info.contentType}</span>
          <span><span class="text-slate-400 font-medium">Modified:</span> {formatDate(info.lastModified)}</span>
          {#if info.eTag}
            <span class="hidden sm:inline"><span class="text-slate-400 font-medium">ETag:</span> <span class="font-mono">{info.eTag.replace(/"/g,'')}</span></span>
          {/if}
        {/if}
      </div>
    </div>
  </div>
{/if}
