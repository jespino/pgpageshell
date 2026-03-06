import { useCallback, useEffect, useState } from "react";
import type { FileInfo, PageDetail, TooltipContent, TooltipState, SelectedElement } from "../types";
import type { DataBackend, FileEntry } from "../backend";
import { Sidebar } from "./Sidebar";
import { PageSVG } from "./PageSVG";
import { Tooltip } from "./Tooltip";
import { DetailPanel } from "./DetailPanel";

interface AppProps {
  backend: DataBackend;
}

export function App({ backend }: AppProps) {
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
            <DetailPanel element={selectedElement} detail={pageDetail} />
          </div>
        )}
      </div>
      {tooltip && <Tooltip {...tooltip} />}
    </div>
  );
}
