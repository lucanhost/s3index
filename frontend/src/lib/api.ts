import type { DirectoryListing, FileInfo } from './types';

const BASE = '/api';

export async function listDirectory(prefix: string, signal?: AbortSignal, offset?: number, limit?: number): Promise<DirectoryListing> {
  let url = `${BASE}/list?prefix=${encodeURIComponent(prefix)}`;
  if (offset !== undefined) url += `&offset=${offset}`;
  if (limit !== undefined) url += `&limit=${limit}`;
  const res = await fetch(url, { signal });
  if (!res.ok) throw new Error(`List failed: ${res.status}`);
  return res.json();
}

export async function getFileInfo(key: string, signal?: AbortSignal): Promise<FileInfo> {
  const url = `${BASE}/info?key=${encodeURIComponent(key)}`;
  const res = await fetch(url, { signal });
  if (!res.ok) throw new Error(`Info failed: ${res.status}`);
  return res.json();
}

export function getObjectUrl(key: string): string {
  return `${BASE}/object/${encodeURIComponent(key)}`;
}

export async function getReadme(prefix: string, signal?: AbortSignal): Promise<string | null> {
  const readmeKey = prefix ? `${prefix.replace(/\/$/, '')}/README.md` : 'README.md';
  try {
    const res = await fetch(getObjectUrl(readmeKey), { signal });
    if (!res.ok) return null;
    const ct = res.headers.get('content-type') || '';
    if (!ct.includes('text') && !ct.includes('markdown') && !ct.includes('octet')) return null;
    return res.text();
  } catch {
    return null;
  }
}

export async function searchFiles(query: string, signal?: AbortSignal): Promise<{ files: import('./types').FileEntry[], folders: import('./types').FolderEntry[] }> {
  if (!query) return { files: [], folders: [] };
  const res = await fetch(`${BASE}/search?q=${encodeURIComponent(query)}`, { signal });
  if (!res.ok) return { files: [], folders: [] };
  return res.json();
}



/** Format bytes to human-readable string */
export function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

/** Format ISO date to friendly string */
export function formatDate(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });
}

/** Returns a relative time string like "3 days ago" */
export function relativeTime(iso: string): string {
  if (!iso) return '';
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;
  const s = Math.floor(diff / 1000);
  if (s < 60) return 'just now';
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 86400 * 30) return `${Math.floor(s / 86400)}d ago`;
  return formatDate(iso);
}

/** Get MIME category from content type or filename */
export function getCategory(name: string, contentType?: string): string {
  const ext = name.split('.').pop()?.toLowerCase() || '';
  const ct = contentType?.toLowerCase() || '';
  
  if (ct.startsWith('image/') || ['jpg','jpeg','png','gif','webp','svg','bmp','ico','avif'].includes(ext)) return 'image';
  if (ct.startsWith('video/') || ['mp4','webm','mov','mkv','avi','m4v'].includes(ext)) return 'video';
  if (ct.startsWith('audio/') || ['mp3','wav','ogg','flac','aac','m4a','opus'].includes(ext)) return 'audio';
  if (ext === 'pdf' || ct === 'application/pdf') return 'pdf';
  if (['md','markdown'].includes(ext) || ct.includes('markdown')) return 'markdown';
  if (['js','ts','tsx','jsx','py','go','rs','java','c','cpp','h','cs','rb','php','swift','kt','sh','bash','zsh','fish','ps1','lua','r','dart','ex','exs','zig','nim'].includes(ext)) return 'code';
  if (['html','htm','xml','yaml','yml','json','toml','ini','env','conf','config'].includes(ext)) return 'code';
  if (['txt','log','csv','tsv'].includes(ext) || ct.startsWith('text/')) return 'text';
  if (['zip','tar','gz','bz2','xz','7z','rar'].includes(ext)) return 'archive';
  return 'file';
}

const CATEGORY_COLORS: Record<string, string> = {
  image: 'text-green-400',
  video: 'text-blue-400',
  audio: 'text-orange-400',
  pdf: 'text-red-400',
  markdown: 'text-purple-400',
  code: 'text-cyan-400',
  text: 'text-slate-300',
  archive: 'text-yellow-400',
  file: 'text-slate-400',
};

/** Get a color class for a file category */
export function getCategoryColor(category: string): string {
  return CATEGORY_COLORS[category] || 'text-slate-400';
}
