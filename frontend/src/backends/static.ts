import type { DataBackend, FileEntry } from "../backend";
import type { FileInfo, PageDetail } from "../types";

export interface StaticFileData {
  filename: string;
  file_type: string;
  info: FileInfo;
  pages: PageDetail[];
}

export function createStaticBackend(dataUrl: string): DataBackend {
  let data: StaticFileData[] | null = null;

  async function load(): Promise<StaticFileData[]> {
    if (data) return data;
    const resp = await fetch(dataUrl);
    data = await resp.json();
    return data!;
  }

  return {
    async getFiles(): Promise<FileEntry[]> {
      const files = await load();
      return files.map((f, i) => ({
        index: i,
        filename: f.filename,
        total_pages: f.info.total_pages,
        file_type: f.file_type,
      }));
    },

    async getFileInfo(fileIdx: number): Promise<FileInfo> {
      const files = await load();
      return files[fileIdx].info;
    },

    async getPageDetail(fileIdx: number, pageNum: number): Promise<PageDetail> {
      const files = await load();
      return files[fileIdx].pages[pageNum];
    },
  };
}
