import type { FileInfo, PageDetail } from "./types";

export interface FileEntry {
  index: number;
  filename: string;
  total_pages: number;
  file_type: string;
}

export interface DataBackend {
  getFiles(): Promise<FileEntry[]>;
  getFileInfo(fileIdx: number): Promise<FileInfo>;
  getPageDetail(fileIdx: number, pageNum: number): Promise<PageDetail>;
  openFile?(): Promise<FileEntry[]>;
  closeFile?(fileIdx: number): Promise<FileEntry[]>;
}
