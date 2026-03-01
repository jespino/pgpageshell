import { useCallback, useEffect, useState } from "react";
import type { FileEntry, FilesResponse, FileInfo, PageDetail, TooltipContent, TooltipState, SelectedElement } from "../types";
import { Sidebar } from "./Sidebar";
import { PageSVG } from "./PageSVG";
import { Tooltip } from "./Tooltip";

export function App() {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [selectedFileIdx, setSelectedFileIdx] = useState(0);
  const [fileInfo, setFileInfo] = useState<FileInfo | null>(null);
  const [selectedPage, setSelectedPage] = useState(0);
  const [pageDetail, setPageDetail] = useState<PageDetail | null>(null);
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const [selectedElement, setSelectedElement] = useState<SelectedElement | null>(null);

  const loadFile = useCallback((fileIdx: number) => {
    setSelectedFileIdx(fileIdx);
    setPageDetail(null);
    setSelectedPage(0);
    setSelectedElement(null);
    fetch(`/api/file/${fileIdx}`)
      .then((r) => r.json())
      .then((data: FileInfo) => {
        setFileInfo(data);
        if (data.total_pages > 0) {
          fetch(`/api/file/${fileIdx}/page/0`)
            .then((r) => r.json())
            .then(setPageDetail);
        }
      });
  }, []);

  const loadPage = useCallback((n: number) => {
    setSelectedPage(n);
    setSelectedElement(null);
    fetch(`/api/file/${selectedFileIdx}/page/${n}`)
      .then((r) => r.json())
      .then(setPageDetail);
  }, [selectedFileIdx]);

  useEffect(() => {
    fetch("/api/files")
      .then((r) => r.json())
      .then((data: FilesResponse) => {
        setFiles(data.files);
        if (data.files.length > 0) loadFile(0);
      });
  }, [loadFile]);

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
        {files.length > 1 ? (
          <select
            className="file-select"
            value={selectedFileIdx}
            onChange={(e) => loadFile(Number(e.target.value))}
          >
            {files.map((f) => (
              <option key={f.index} value={f.index}>
                {f.filename} ({f.total_pages} pages)
              </option>
            ))}
          </select>
        ) : (
          <span className="file-info">
            {fileInfo.filename} — {fileInfo.total_pages} pages — {fileInfo.file_type}
          </span>
        )}
      </div>
      <div className="main-content">
        <Sidebar
          fileInfo={fileInfo}
          selectedPage={selectedPage}
          onSelect={loadPage}
          selectedElement={selectedElement}
          pageDetail={pageDetail}
        />
        <div className="viewer">
          {pageDetail ? (
            <PageSVG
              detail={pageDetail}
              showTooltip={showTooltip}
              hideTooltip={hideTooltip}
              onSelect={setSelectedElement}
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
