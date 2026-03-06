import { useCallback, useEffect, useState } from "react";
import type { FileInfo, PageDetail, TooltipContent, TooltipState, SelectedElement } from "../types";
import type { DataBackend, FileEntry } from "../backend";
import { Sidebar } from "./Sidebar";
import { PageSVG } from "./PageSVG";
import { Tooltip } from "./Tooltip";
import { DetailPanel } from "./DetailPanel";

interface AppProps {
  backend: DataBackend;
  repoUrl?: string;
}

export function App({ backend, repoUrl }: AppProps) {
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
    backend.getFileInfo(fileIdx).then((data) => {
      setFileInfo(data);
      if (data.total_pages > 0) {
        backend.getPageDetail(fileIdx, 0).then(setPageDetail);
      }
    });
  }, [backend]);

  const loadPage = useCallback((n: number) => {
    setSelectedPage(n);
    setSelectedElement(null);
    backend.getPageDetail(selectedFileIdx, n).then(setPageDetail);
  }, [backend, selectedFileIdx]);

  useEffect(() => {
    backend.getFiles().then((entries) => {
      setFiles(entries);
      if (entries.length > 0) loadFile(0);
    });
  }, [backend, loadFile]);

  const showTooltip = useCallback((evt: React.MouseEvent, content: TooltipContent) => {
    setTooltip({ x: evt.clientX + 12, y: evt.clientY + 12, content });
  }, []);

  const hideTooltip = useCallback(() => {
    setTooltip(null);
  }, []);

  const handleOpenFile = useCallback(() => {
    if (!backend.openFile) return;
    backend.openFile().then((entries) => {
      setFiles(entries);
      if (entries.length > 0 && entries.length > files.length) {
        loadFile(entries.length - 1);
      }
    });
  }, [backend, files.length, loadFile]);

  const handleCloseFile = useCallback(() => {
    if (!backend.closeFile) return;
    backend.closeFile(selectedFileIdx).then((entries) => {
      setFiles(entries);
      if (entries.length > 0) {
        loadFile(Math.min(selectedFileIdx, entries.length - 1));
      } else {
        setFileInfo(null);
        setPageDetail(null);
      }
    });
  }, [backend, selectedFileIdx, loadFile]);

  if (files.length === 0) {
    return (
      <div className="app">
        <div className="welcome">
          <div className="welcome-content">
            <img src={`${import.meta.env.BASE_URL}logo.webp`} alt="pgpageshell" className="welcome-logo" />
            <h1 className="welcome-title">pgpageshell</h1>
            <p className="welcome-subtitle">PostgreSQL Page Inspector</p>
            <p className="welcome-desc">
              Open a PostgreSQL heap or index data file to inspect its pages,
              headers, line pointers, tuples, and special regions.
            </p>
            {backend.openFile && (
              <button className="welcome-btn" onClick={handleOpenFile}>
                Open File
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

  if (!fileInfo) {
    return <div className="loading">Loading...</div>;
  }

  return (
    <div className="app">
      <div className="topbar">
        <img src={`${import.meta.env.BASE_URL}logo.webp`} alt="" className="topbar-logo" />
        <h1>pgpageshell</h1>
        <select
          className="file-select"
          value={selectedFileIdx}
          onChange={(e) => loadFile(Number(e.target.value))}
        >
          {files.map((f) => (
            <option key={f.index} value={f.index}>
              {f.filename} ({f.file_type}, {f.total_pages} pages)
            </option>
          ))}
        </select>
        {backend.openFile && (
          <button className="topbar-btn" onClick={handleOpenFile}>
            Open File
          </button>
        )}
        {backend.closeFile && files.length > 1 && (
          <button
            className="topbar-btn topbar-btn-danger"
            onClick={handleCloseFile}
          >
            Close File
          </button>
        )}
        {repoUrl && (
          <a href={repoUrl} target="_blank" rel="noopener noreferrer" className="topbar-link" title="GitHub">
            <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor">
              <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0016 8c0-4.42-3.58-8-8-8z"/>
            </svg>
          </a>
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
        {selectedElement && pageDetail && (
          <div className="mobile-detail">
            <button
              className="mobile-detail-close"
              onClick={() => setSelectedElement(null)}
              aria-label="Close"
            >
              ✕
            </button>
            <DetailPanel element={selectedElement} detail={pageDetail} />
          </div>
        )}
      </div>
      {tooltip && <Tooltip {...tooltip} />}
    </div>
  );
}
