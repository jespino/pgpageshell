import type { DataBackend, FileEntry } from "../backend";
import type { FileInfo, PageDetail } from "../types";
import {
  GetFiles as WailsGetFiles,
  GetFileInfo as WailsGetFileInfo,
  GetPageDetail as WailsGetPageDetail,
  OpenFile as WailsOpenFile,
  CloseFile as WailsCloseFile,
} from "../../wailsjs/go/main/App";

export const wailsBackend: DataBackend = {
  getFiles(): Promise<FileEntry[]> {
    return WailsGetFiles();
  },
  getFileInfo(fileIdx: number): Promise<FileInfo> {
    return WailsGetFileInfo(fileIdx);
  },
  getPageDetail(fileIdx: number, pageNum: number): Promise<PageDetail> {
    return WailsGetPageDetail(fileIdx, pageNum);
  },
  openFile(): Promise<FileEntry[]> {
    return WailsOpenFile();
  },
  closeFile(fileIdx: number): Promise<FileEntry[]> {
    return WailsCloseFile(fileIdx);
  },
};
