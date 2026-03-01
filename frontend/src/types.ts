export interface PageSummary {
  page_num: number;
  type: string;
  num_items: number;
  free_space: number;
  special_size: number;
}

export interface FileInfo {
  filename: string;
  total_pages: number;
  file_type: string;
  pages: PageSummary[];
}

export interface PageRegion {
  name: string;
  start_byte: number;
  end_byte: number;
  size: number;
  region_type: string;
}

export interface LinePointerInfo {
  index: number;
  status: string;
  offset: number;
  length: number;
}

export interface TupleInfo {
  index: number;
  status: string;
  offset: number;
  length: number;
  start_byte: number;
  end_byte: number;
  properties: Record<string, string>;
}

export interface PageDetail {
  page_num: number;
  type: string;
  header: Record<string, string>;
  regions: PageRegion[];
  line_pointers: LinePointerInfo[];
  tuples: TupleInfo[];
  special_info?: Record<string, string>;
}

export interface TooltipContent {
  title: string;
  rows: [string, string | number][];
}

export interface TooltipState {
  x: number;
  y: number;
  content: TooltipContent;
}

export type SelectedElement =
  | { type: "region"; data: PageRegion }
  | { type: "tuple"; data: TupleInfo }
  | { type: "linp"; data: LinePointerInfo };
