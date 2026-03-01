import { useCallback, useEffect, useState } from "react";
import type { FileInfo, PageDetail, TooltipContent, TooltipState } from "../types";
import { Sidebar } from "./Sidebar";
import { PageSVG } from "./PageSVG";
import { Tooltip } from "./Tooltip";

export function App() {
  const [fileInfo, setFileInfo] = useState<FileInfo | null>(null);
  const [selectedPage, setSelectedPage] = useState(0);
  const [pageDetail, setPageDetail] = useState<PageDetail | null>(null);
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);

  const loadPage = useCallback((n: number) => {
    setSelectedPage(n);
    fetch(`/api/page/${n}`)
      .then((r) => r.json())
      .then(setPageDetail);
  }, []);

  useEffect(() => {
    fetch("/api/file")
      .then((r) => r.json())
      .then((data: FileInfo) => {
        setFileInfo(data);
        if (data.total_pages > 0) loadPage(0);
      });
  }, [loadPage]);

  const showTooltip = useCallback((evt: React.MouseEvent, content: TooltipContent) => {
    setTooltip({ x: evt.clientX + 12, y: evt.clientY + 12, content });
  }, []);

  const hideTooltip = useCallback(() => {
    setTooltip(null);
  }, []);

  if (!fileInfo) {
    return <div className="loading">Loading...</div>;
  }

  return (
    <div className="app">
      <div className="topbar">
        <h1>pgpageshell</h1>
        <span className="file-info">
          {fileInfo.filename} — {fileInfo.total_pages} pages — {fileInfo.file_type}
        </span>
      </div>
      <div className="main-content">
        <Sidebar fileInfo={fileInfo} selectedPage={selectedPage} onSelect={loadPage} />
        <div className="viewer">
          {pageDetail ? (
            <PageSVG
              detail={pageDetail}
              showTooltip={showTooltip}
              hideTooltip={hideTooltip}
            />
          ) : (
            <div className="loading">Select a page</div>
          )}
        </div>
      </div>
      {tooltip && <Tooltip {...tooltip} />}
    </div>
  );
}
