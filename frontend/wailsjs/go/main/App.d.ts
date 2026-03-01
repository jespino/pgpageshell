import { main } from "../models";

export function GetFiles(): Promise<main.FileEntry[]>;
export function GetFileInfo(fileIdx: number): Promise<main.FileInfo>;
export function GetPageDetail(fileIdx: number, pageNum: number): Promise<main.PageDetail>;
export function OpenFile(): Promise<main.FileEntry[]>;
export function CloseFile(fileIdx: number): Promise<main.FileEntry[]>;
