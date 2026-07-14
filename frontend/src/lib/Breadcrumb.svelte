<script lang="ts">
  // Breadcrumb navigation - parses a prefix into path segments
  export let prefix: string = '';
  export let onNavigate: (path: string) => void;

  function buildCrumbs(p: string) {
    const clean = p.replace(/^\/|\/$/g, '');
    if (!clean) return [];
    const parts = clean.split('/');
    return parts.map((name, i) => ({
      name,
      path: parts.slice(0, i + 1).join('/') + '/',
    }));
  }

  $: crumbs = buildCrumbs(prefix);
</script>

<nav aria-label="Breadcrumb" class="flex items-center gap-1 text-sm flex-wrap">
  <!-- Home / root -->
  <button
    class="flex items-center gap-1.5 px-2 py-1 rounded-md text-slate-400 hover:text-purple-300 hover:bg-white/5 transition-all duration-150 cursor-pointer"
    onclick={() => onNavigate('')}
    aria-label="Root"
  >
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <polyline points="9 22 9 12 15 12 15 22" />
    </svg>
  </button>

  {#each crumbs as crumb, i}
    <!-- Separator -->
    <svg class="text-slate-600 flex-shrink-0" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
      <polyline points="9 18 15 12 9 6" />
    </svg>

    {#if i === crumbs.length - 1}
      <!-- Current directory (not clickable) -->
      <span class="px-2 py-1 text-purple-300 font-semibold truncate max-w-[200px]" aria-current="page">{crumb.name}</span>
    {:else}
      <button
        class="px-2 py-1 rounded-md text-slate-300 hover:text-purple-300 hover:bg-white/5 transition-all duration-150 truncate max-w-[160px] cursor-pointer"
        onclick={() => onNavigate(crumb.path)}
      >
        {crumb.name}
      </button>
    {/if}
  {/each}
</nav>
