import { PAGE_SIZE } from "../colors";
import type { PageDetail, SelectedElement } from "../types";

interface DetailPanelProps {
  element: SelectedElement;
  detail: PageDetail;
}

export function DetailPanel({ element, detail }: DetailPanelProps) {
  let title = "";
  const rows: [string, string | number][] = [];

  if (element.type === "region") {
    const r = element.data;
    title = r.name;
    rows.push(
      ["Type", r.region_type],
      ["Offset Range", `${r.start_byte} – ${r.end_byte - 1}`],
      ["Size", `${r.size} bytes`],
      ["% of Page", `${((r.size / PAGE_SIZE) * 100).toFixed(1)}%`]
    );
    if (r.region_type === "header" && detail.header) {
      for (const [k, v] of Object.entries(detail.header)) {
        rows.push([k, v]);
      }
    }
    if (r.region_type === "special" && detail.special_info) {
      for (const [k, v] of Object.entries(detail.special_info)) {
        rows.push([k, v]);
      }
    }
  } else if (element.type === "linp") {
    const lp = element.data;
    title = `Line Pointer #${lp.index}`;
    rows.push(
      ["Status", lp.status],
      ["Points to offset", lp.offset],
      ["Points to length", `${lp.length} bytes`]
    );
  } else if (element.type === "tuple") {
    const t = element.data;
    title = `Tuple ${t.index}`;
    rows.push(["Status", t.status], ["Offset", t.offset], ["Length", `${t.length} bytes`]);
    if (t.properties) {
      for (const [k, v] of Object.entries(t.properties)) {
        rows.push([k, v]);
      }
    }
  }

  return (
    <div className="detail-panel">
      <h3>{title}</h3>
      <table>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i}>
              <td>{row[0]}</td>
              <td>{String(row[1])}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
