import { useMemo, useState, useCallback } from "react";
import type React from "react";
import { REGION_COLORS, statusColor, PAGE_SIZE } from "../colors";
import type {
  PageDetail,
  TooltipContent,
  SelectedElement,
} from "../types";
import { DetailPanel } from "./DetailPanel";

interface PageSVGProps {
  detail: PageDetail;
  showTooltip: (evt: React.MouseEvent, content: TooltipContent) => void;
  hideTooltip: () => void;
}

// Grid layout: 32 columns × 64 rows = 2048 cells, each cell = 4 bytes
const COLS = 32;
const ROWS = 64;
const BYTES_PER_CELL = 4;
const CELL_SIZE = 16;
const CELL_GAP = 1;
const CELL_STRIDE = CELL_SIZE + CELL_GAP;

const GRID_X = 16;
const TITLE_H = 24;
const LEGEND_H = 30;
const GRID_Y = TITLE_H + LEGEND_H + 8;

const GRID_W = COLS * CELL_STRIDE;
const GRID_H = ROWS * CELL_STRIDE;
const SVG_WIDTH = GRID_X + GRID_W;
const SVG_HEIGHT = GRID_Y + GRID_H + 10;

const ITEM_ID_SIZE = 4; // each line pointer is 4 bytes
const PAGE_HEADER_SIZE = 24;

// What a cell maps to
type CellInfo = {
  regionType: string;
  label: string;
  tooltip: TooltipContent;
  select: SelectedElement;
  // For line pointers and tuples: the 1-based item index used to
  // cross-highlight between grid shapes and the items list.
  itemIndex?: number;
};

function buildCellMap(detail: PageDetail): CellInfo[] {
  const cells: CellInfo[] = new Array(COLS * ROWS);
  const regions = detail.regions ?? [];
  const linePointers = detail.line_pointers ?? [];
  const tuples = detail.tuples ?? [];

  const linpRegion = regions.find((r) => r.region_type === "linp");
  const freeRegion = regions.find((r) => r.region_type === "free");
  const headerRegion = regions.find((r) => r.region_type === "header");
  const specialRegion = regions.find((r) => r.region_type === "special");

  function fillRange(startByte: number, endByte: number, info: CellInfo) {
    const startCell = Math.floor(startByte / BYTES_PER_CELL);
    const endCell = Math.ceil(endByte / BYTES_PER_CELL);
    for (let i = startCell; i < endCell && i < cells.length; i++) {
      cells[i] = info;
    }
  }

  if (headerRegion) {
    fillRange(0, PAGE_HEADER_SIZE, {
      regionType: "header",
      label: "Page Header",
      tooltip: {
        title: "Page Header",
        rows: [
          ["Offset", `0 – ${PAGE_HEADER_SIZE - 1}`],
          ["Size", `${PAGE_HEADER_SIZE} bytes`],
          ...Object.entries(detail.header ?? {}).map(
            ([k, v]) => [k, v] as [string, string]
          ),
        ],
      },
      select: { type: "region", data: headerRegion },
    });
  }

  if (linpRegion) {
    linePointers.forEach((lp, i) => {
      const start = linpRegion.start_byte + i * ITEM_ID_SIZE;
      const end = start + ITEM_ID_SIZE;
      fillRange(start, end, {
        regionType: "linp",
        label: `LP #${lp.index} ${lp.status}`,
        tooltip: {
          title: `Line Pointer #${lp.index}`,
          rows: [
            ["Status", lp.status],
            ["Pointer at", `${start} – ${end - 1}`],
            ["Points to", `offset ${lp.offset}, ${lp.length} bytes`],
          ],
        },
        select: { type: "linp", data: lp },
        itemIndex: lp.index,
      });
    });
  }

  if (freeRegion && freeRegion.size > 0) {
    fillRange(freeRegion.start_byte, freeRegion.end_byte, {
      regionType: "free",
      label: "Free Space",
      tooltip: {
        title: "Free Space",
        rows: [
          ["Offset", `${freeRegion.start_byte} – ${freeRegion.end_byte - 1}`],
          ["Size", `${freeRegion.size} bytes`],
          ["% of page", `${((freeRegion.size / PAGE_SIZE) * 100).toFixed(1)}%`],
        ],
      },
      select: { type: "region", data: freeRegion },
    });
  }

  tuples.forEach((t) => {
    if (!t.start_byte && !t.end_byte) return;
    const tooltipRows: [string, string | number][] = [
      ["Status", t.status],
      ["Offset", `${t.start_byte} – ${t.end_byte - 1}`],
      ["Length", `${t.length} bytes`],
    ];
    if (t.properties) {
      for (const [k, v] of Object.entries(t.properties)) {
        tooltipRows.push([k, v]);
      }
    }
    fillRange(t.start_byte, t.end_byte, {
      regionType: "tuple",
      label: `Tuple #${t.index}`,
      tooltip: { title: `Tuple #${t.index}`, rows: tooltipRows },
      select: { type: "tuple", data: t },
      itemIndex: t.index,
    });
  });

  if (specialRegion && specialRegion.size > 0) {
    const tooltipRows: [string, string | number][] = [
      ["Offset", `${specialRegion.start_byte} – ${specialRegion.end_byte - 1}`],
      ["Size", `${specialRegion.size} bytes`],
    ];
    if (detail.special_info) {
      for (const [k, v] of Object.entries(detail.special_info)) {
        tooltipRows.push([k, v]);
      }
    }
    fillRange(specialRegion.start_byte, specialRegion.end_byte, {
      regionType: "special",
      label: `Special (${detail.type})`,
      tooltip: { title: `Special (${detail.type})`, rows: tooltipRows },
      select: { type: "region", data: specialRegion },
    });
  }

  return cells;
}

export function PageSVG({
  detail,
  showTooltip,
  hideTooltip,
}: PageSVGProps) {
  const cellMap = useMemo(() => buildCellMap(detail), [detail]);
  const tuples = detail.tuples ?? [];
  const linePointers = detail.line_pointers ?? [];

  const [selectedElement, setSelectedElement] = useState<SelectedElement | null>(null);
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  const [hoveredRegion, setHoveredRegion] = useState<string | null>(null);
  const onHover = useCallback((cell: CellInfo) => {
    if (cell.itemIndex != null) {
      setHoveredIndex(cell.itemIndex);
      setHoveredRegion(null);
    } else if (cell.regionType !== "free") {
      setHoveredIndex(null);
      setHoveredRegion(cell.regionType);
    }
  }, []);
  const onUnhover = useCallback(() => {
    setHoveredIndex(null);
    setHoveredRegion(null);
  }, []);

  return (
    <div className="page-view">
      {/* Left: grid */}
      <div className="grid-panel">
        <svg
          width={SVG_WIDTH}
          height={SVG_HEIGHT}
          xmlns="http://www.w3.org/2000/svg"
        >
          {/* Title */}
          <text
            x={SVG_WIDTH / 2}
            y={16}
            textAnchor="middle"
            fill="#8b949e"
            fontSize={12}
            fontFamily="monospace"
          >
            Page {detail.page_num} ({detail.type}) — {PAGE_SIZE} bytes
            — {BYTES_PER_CELL}B per cell
          </text>

          {/* Legend */}
          <Legend />

          {/* Grid background */}
          <rect
            x={GRID_X}
            y={GRID_Y}
            width={COLS * CELL_STRIDE - CELL_GAP}
            height={ROWS * CELL_STRIDE - CELL_GAP}
            rx={2}
            fill="#0d1117"
          />

          {/* Individual cells */}
          {cellMap.map((cell, i) => {
            if (!cell) return null;
            const col = i % COLS;
            const row = Math.floor(i / COLS);
            const x = GRID_X + col * CELL_STRIDE;
            const y = GRID_Y + row * CELL_STRIDE;
            const color = REGION_COLORS[cell.regionType] ?? REGION_COLORS.free;
            const isHighlighted =
              (hoveredIndex != null &&
                cell.itemIndex != null &&
                cell.itemIndex === hoveredIndex) ||
              (hoveredRegion != null &&
                cell.itemIndex == null &&
                cell.regionType === hoveredRegion);
            const fill = isHighlighted ? color.hover : color.fill;

            return (
              <rect
                key={i}
                x={x}
                y={y}
                width={CELL_SIZE}
                height={CELL_SIZE}
                rx={2}
                fill={fill}
                stroke={isHighlighted ? color.text : color.stroke}
                strokeWidth={isHighlighted ? 1.5 : 0.5}
                cursor="pointer"
                onMouseMove={(evt) => showTooltip(evt, cell.tooltip)}
                onMouseLeave={() => {
                  hideTooltip();
                  onUnhover();
                }}
                onClick={() => setSelectedElement(cell.select)}
                onMouseEnter={() => onHover(cell)}
              />
            );
          })}
        </svg>
      </div>

      {/* Right: items list */}
      <div className="items-panel">
        {linePointers.length > 0 && (
          <div className="items-section">
            <div className="items-section-title">Line Pointers ({linePointers.length})</div>
            {linePointers.map((lp) => {
              const sc = statusColor(lp.status);
              const highlighted = hoveredIndex === lp.index;
              return (
                <div
                  key={`lp-${lp.index}`}
                  className={`item-row${highlighted ? " item-row-highlight" : ""}`}
                  onClick={() => setSelectedElement({ type: "linp", data: lp })}
                  onMouseMove={(evt) =>
                    showTooltip(evt, {
                      title: `Line Pointer #${lp.index}`,
                      rows: [
                        ["Status", lp.status],
                        ["Points to", `offset ${lp.offset}, ${lp.length} bytes`],
                      ],
                    })
                  }
                  onMouseEnter={() => { setHoveredIndex(lp.index); setHoveredRegion(null); }}
                  onMouseLeave={() => {
                    hideTooltip();
                    onUnhover();
                  }}
                >
                  <span className="item-dot" style={{ background: sc }} />
                  <span className="item-label">
                    #{lp.index} {lp.status}
                  </span>
                  <span className="item-meta">
                    → {lp.offset} ({lp.length}B)
                  </span>
                </div>
              );
            })}
          </div>
        )}

        {tuples.length > 0 && (
          <div className="items-section">
            <div className="items-section-title">Tuples ({tuples.length})</div>
            {tuples.map((t) => {
              const sc = statusColor(t.status);
              const highlighted = hoveredIndex === t.index;
              const extra = t.properties?.printable
                ? ` "${t.properties.printable.substring(0, 24)}"`
                : t.properties?.t_ctid
                  ? ` ctid=${t.properties.t_ctid}`
                  : "";
              return (
                <div
                  key={`t-${t.index}`}
                  className={`item-row${highlighted ? " item-row-highlight" : ""}`}
                  onClick={() => setSelectedElement({ type: "tuple", data: t })}
                  onMouseMove={(evt) => {
                    const rows: [string, string | number][] = [
                      ["Status", t.status],
                      ["Offset", `${t.start_byte} – ${t.end_byte - 1}`],
                      ["Length", `${t.length} bytes`],
                    ];
                    if (t.properties) {
                      for (const [k, v] of Object.entries(t.properties)) {
                        rows.push([k, v]);
                      }
                    }
                    showTooltip(evt, { title: `Tuple #${t.index}`, rows });
                  }}
                  onMouseEnter={() => { setHoveredIndex(t.index); setHoveredRegion(null); }}
                  onMouseLeave={() => {
                    hideTooltip();
                    onUnhover();
                  }}
                >
                  <span className="item-dot" style={{ background: sc }} />
                  <span className="item-label">
                    #{t.index} {t.status}
                  </span>
                  <span className="item-meta">
                    {t.length}B @ {t.offset}{extra}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Right: detail panel */}
      {selectedElement && (
        <DetailPanel element={selectedElement} detail={detail} />
      )}
    </div>
  );
}

/* ---- Legend ---- */

function Legend() {
  const items: { label: string; type: string }[] = [
    { label: "Header", type: "header" },
    { label: "Line Ptr", type: "linp" },
    { label: "Free", type: "free" },
    { label: "Tuple", type: "tuple" },
    { label: "Special", type: "special" },
  ];
  const boxSize = 10;
  const gap = 14;
  const totalW =
    items.reduce((s, it) => s + boxSize + 4 + it.label.length * 7 + gap, 0) - gap;
  let x = (SVG_WIDTH - totalW) / 2;
  const y = TITLE_H + 2;

  return (
    <g>
      {items.map((it) => {
        const color = REGION_COLORS[it.type];
        const thisX = x;
        x += boxSize + 4 + it.label.length * 7 + gap;
        return (
          <g key={it.type}>
            <rect
              x={thisX}
              y={y + 4}
              width={boxSize}
              height={boxSize}
              rx={2}
              fill={color.fill}
              stroke={color.stroke}
              strokeWidth={1}
            />
            <text
              x={thisX + boxSize + 4}
              y={y + 13}
              fill={color.text}
              fontSize={10}
              fontFamily="monospace"
            >
              {it.label}
            </text>
          </g>
        );
      })}
    </g>
  );
}
