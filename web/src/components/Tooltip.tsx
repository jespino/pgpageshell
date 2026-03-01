import type { TooltipState } from "../types";

export function Tooltip({ x, y, content }: TooltipState) {
  const style: React.CSSProperties = {
    left: Math.min(x, window.innerWidth - 420),
    top: Math.min(y, window.innerHeight - 200),
  };

  return (
    <div className="tooltip" style={style}>
      <div className="tt-title">{content.title}</div>
      {content.rows.map((row, i) => (
        <div className="tt-row" key={i}>
          <span className="tt-key">{row[0]}:</span>
          <span className="tt-val">{String(row[1])}</span>
        </div>
      ))}
    </div>
  );
}
