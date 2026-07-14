<script lang="ts">
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';

  export let content: string = '';

  let rendered = '';

  $: {
    try {
      rendered = DOMPurify.sanitize(marked.parse(content) as string);
    } catch {
      rendered = DOMPurify.sanitize(`<p>${content}</p>`);
    }
  }
</script>

<div class="markdown-body prose-sm max-w-none" style="font-size:0.9rem;">
  {@html rendered}
</div>
