// Shared types across frontend and could be shared with backend

export interface FileEntry {
  name: string;
  path: string;
  size: number;
  lastModified: string;
}

export interface FolderEntry {
  name: string;
  path: string;
}

export interface DirectoryListing {
  folders: FolderEntry[];
  files: FileEntry[];
}

export interface FileInfo {
  size: number;
  contentType: string;
  lastModified: string;
  eTag: string;
}

export type ViewMode = 'list' | 'grid';
export type SortKey = 'name' | 'size' | 'lastModified' | 'type';
export type SortDir = 'asc' | 'desc';
