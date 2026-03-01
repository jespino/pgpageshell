import type React from "react";
import { REGION_COLORS, TUPLE_COLORS, statusColor, PAGE_SIZE } from "../colors";
import type { PageDetail, TooltipContent, SelectedElement } from "../types";

interface PageSVGProps {
  detail: PageDetail;
  showTooltip: (evt: React.MouseEvent, content: TooltipContent) => void;
  hideTooltip: () => void;
  onSelect: (element: SelectedElement) => void;
}

const SVG_WIDTH = 700;
const PAGE_RECT_X = 20;
const PAGE_RECT_WIDTH = SVG_WIDTH - 40;
const REGION_HEIGHT = 40;
const TUPLE_ROW_HEIGHT = 24;
const PADDING = 4;
const HEADER_Y = 20;

export function PageSVG({ detail, showTooltip, hideTooltip, onSelect }: PageSVGProps) {
  const regions = detail.regions ?? [];
  const tuples = detail.tuples ?? [];

  // Layout: regions stacked vertically
  let y = HEADER_Y;
  const regionRects = regions.map((region) => {
    const rect = { ...region, y, height: REGION_HEIGHT };
    y += REGION_HEIGHT + PADDING;
    return rect;
  });

  // Tuples section below regions
  const tuplesStartY = y + 10;
  y = tuplesStartY;
  const tupleRects: (typeof tuples[number] & { y: number; height: number; colorIdx: number })[] = [];
  if (tuples.length > 0) {
    y += 20; // label space
    tuples.forEach((t, i) => {
      tupleRects.push({ ...t, y, height: TUPLE_ROW_HEIGHT, colorIdx: i % TUPLE_COLORS.length });
      y += TUPLE_ROW_HEIGHT + 2;
    });
  }

  const totalHeight = y + 20;
  const pageRectHeight = totalHeight - 10;

  return (
    <div className="svg-container">
      <svg width={SVG_WIDTH} height={totalHeight} xmlns="http://www.w3.org/2000/svg">
        {/* Page outer rectangle */}
        <rect
          x={PAGE_RECT_X - 10}
          y={5}
          width={PAGE_RECT_WIDTH + 20}
          height={pageRectHeight}
          rx={6}
          fill="#0d1117"
          stroke="#30363d"
          strokeWidth={2}
        />
        {/* Page title */}
        <text
          x={SVG_WIDTH / 2}
          y={HEADER_Y - 2}
          textAnchor="middle"
          fill="#8b949e"
          fontSize={11}
          fontFamily="monospace"
        >
          Page {detail.page_num} ({detail.type}) — {PAGE_SIZE} bytes
        </text>

        {/* Region rectangles */}
        {regionRects.map((r, i) => {
          const color = REGION_COLORS[r.region_type] ?? REGION_COLORS.free;
          return (
            <g key={`region-${i}`}>
              <rect
                x={PAGE_RECT_X}
                y={r.y}
                width={PAGE_RECT_WIDTH}
                height={r.height}
                rx={3}
                fill={color.fill}
                stroke={color.stroke}
                strokeWidth={1.5}
                cursor="pointer"
                onMouseMove={(evt) =>
                  showTooltip(evt, {
                    title: r.name,
                    rows: [
                      ["Offset", `${r.start_byte} – ${r.end_byte - 1}`],
                      ["Size", `${r.size} bytes`],
                      ["% of page", `${((r.size / PAGE_SIZE) * 100).toFixed(1)}%`],
                    ],
                  })
                }
                onMouseLeave={hideTooltip}
                onClick={() => onSelect({ type: "region", data: r })}
                onMouseEnter={(evt) => {
                  (evt.target as SVGRectElement).setAttribute("fill", color.hover);
                }}
                onMouseOut={(evt) => {
                  (evt.target as SVGRectElement).setAttribute("fill", color.fill);
                }}
              />
              <text
                x={PAGE_RECT_X + 10}
                y={r.y + r.height / 2 + 1}
                dominantBaseline="middle"
                fill={color.text}
                fontSize={11}
                fontFamily="monospace"
                pointerEvents="none"
              >
                {r.name}
              </text>
              <text
                x={PAGE_RECT_X + PAGE_RECT_WIDTH - 10}
                y={r.y + r.height / 2 + 1}
                dominantBaseline="middle"
                textAnchor="end"
                fill={color.text}
                fontSize={10}
                fontFamily="monospace"
                opacity={0.7}
                pointerEvents="none"
              >
                {r.size}B [{r.start_byte}-{r.end_byte - 1}]
              </text>
              {/* Proportional bar */}
              <rect
                x={PAGE_RECT_X + 2}
                y={r.y + r.height - 5}
                width={Math.max(2, ((PAGE_RECT_WIDTH - 4) * r.size) / PAGE_SIZE)}
                height={3}
                rx={1.5}
                fill={color.stroke}
                opacity={0.4}
                pointerEvents="none"
              />
            </g>
          );
        })}

        {/* Tuples section label */}
        {tuples.length > 0 && (
          <text
            x={PAGE_RECT_X + 10}
            y={tuplesStartY + 14}
            fill="#8b949e"
            fontSize={11}
            fontFamily="monospace"
          >
            Individual Items ({tuples.length})
          </text>
        )}

        {/* Tuple rows */}
        {tupleRects.map((t, i) => {
          const color = TUPLE_COLORS[t.colorIdx];
          const sc = statusColor(t.status);
          const tooltipRows: [string, string | number][] = [
            ["Status", t.status],
            ["Offset", t.offset],
            ["Length", `${t.length} bytes`],
          ];
          if (t.properties) {
            for (const [k, v] of Object.entries(t.properties)) {
              tooltipRows.push([k, v]);
            }
          }

          const label = [
            `#${t.index} ${t.status}`,
            t.length > 0 ? ` (${t.length}B @ ${t.offset})` : "",
            t.properties?.t_ctid ? ` ctid=${t.properties.t_ctid}` : "",
            t.properties?.t_tid ? ` tid=${t.properties.t_tid}` : "",
            t.properties?.printable
              ? ` "${t.properties.printable.substring(0, 30)}"`
              : "",
          ].join("");

          return (
            <g key={`tuple-${i}`}>
              <rect
                x={PAGE_RECT_X}
                y={t.y}
                width={PAGE_RECT_WIDTH}
                height={t.height}
                rx={2}
                fill={color.fill}
                stroke={color.stroke}
                strokeWidth={1}
                cursor="pointer"
                onMouseMove={(evt) =>
                  showTooltip(evt, { title: `Item ${t.index}`, rows: tooltipRows })
                }
                onMouseLeave={hideTooltip}
                onClick={() => onSelect({ type: "tuple", data: t })}
                onMouseEnter={(evt) => {
                  (evt.target as SVGRectElement).setAttribute("fill", color.hover);
                }}
                onMouseOut={(evt) => {
                  (evt.target as SVGRectElement).setAttribute("fill", color.fill);
                }}
              />
              <circle
                cx={PAGE_RECT_X + 12}
                cy={t.y + t.height / 2}
                r={4}
                fill={sc}
                pointerEvents="none"
              />
              <text
                x={PAGE_RECT_X + 24}
                y={t.y + t.height / 2 + 1}
                dominantBaseline="middle"
                fill="#c9d1d9"
                fontSize={10}
                fontFamily="monospace"
                pointerEvents="none"
              >
                {label}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}
