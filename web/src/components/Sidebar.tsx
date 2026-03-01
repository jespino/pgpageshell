import type { FileInfo } from "../types";

interface SidebarProps {
  fileInfo: FileInfo;
  selectedPage: number;
  onSelect: (pageNum: number) => void;
}

export function Sidebar({ fileInfo, selectedPage, onSelect }: SidebarProps) {
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
    </div>
  );
}
