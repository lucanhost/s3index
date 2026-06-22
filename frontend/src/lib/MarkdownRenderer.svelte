<script lang="ts">
  import { marked } from 'marked';

  export let content: string = '';

  let rendered = '';

  $: {
    try {
      rendered = marked.parse(content) as string;
    } catch {
      rendered = `<p>${content}</p>`;
    }
  }
</script>

<div class="markdown-body prose-sm max-w-none" style="font-size:0.9rem;">
  {@html rendered}
</div>
