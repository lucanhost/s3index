package api

type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type FileEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
}

type DirectoryListing struct {
	Folders []FolderEntry `json:"folders"`
	Files   []FileEntry   `json:"files"`
	HasMore bool          `json:"has_more"`
	Offset  int           `json:"offset"`
}

type FileInfo struct {
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	LastModified string `json:"lastModified"`
	ETag         string `json:"eTag"`
}

type SearchResults struct {
	Files   []FileEntry   `json:"files"`
	Folders []FolderEntry `json:"folders"`
}
