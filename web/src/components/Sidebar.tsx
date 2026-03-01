import type { FileInfo, PageDetail, SelectedElement } from "../types";
import { DetailPanel } from "./DetailPanel";

interface SidebarProps {
  fileInfo: FileInfo;
  selectedPage: number;
  onSelect: (pageNum: number) => void;
  selectedElement: SelectedElement | null;
  pageDetail: PageDetail | null;
}

export function Sidebar({ fileInfo, selectedPage, onSelect, selectedElement, pageDetail }: SidebarProps) {
  return (
    <div className="sidebar">
      <div className="sidebar-header">Pages</div>
      <ul className="page-list">
        {fileInfo.pages.map((p) => (
          <li
            key={p.page_num}
            className={p.page_num === selectedPage ? "active" : ""}
            onClick={() => onSelect(p.page_num)}
          >
            Page {p.page_num}
            <span className="page-type">{p.type}</span>
            <span className="page-items">{p.num_items} items</span>
          </li>
        ))}
      </ul>
      <div className="sidebar-detail">
        {selectedElement && pageDetail ? (
          <DetailPanel element={selectedElement} detail={pageDetail} />
        ) : (
          <div className="sidebar-placeholder">
            Select a block to see its details
          </div>
        )}
      </div>
    </div>
  );
}
