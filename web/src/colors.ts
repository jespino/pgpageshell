export const REGION_COLORS: Record<
  string,
  { fill: string; stroke: string; hover: string; text: string }
> = {
  header: {
    fill: "#2d333b",
    stroke: "#58a6ff",
    hover: "#3a424d",
    text: "#58a6ff",
  },
  linp: {
    fill: "#1e3a2f",
    stroke: "#3fb950",
    hover: "#264d3b",
    text: "#7ee787",
  },
  free: {
    fill: "#1c1c1c",
    stroke: "#484f58",
    hover: "#2a2a2a",
    text: "#8b949e",
  },
  tuple: {
    fill: "#2a1e3a",
    stroke: "#bc8cff",
    hover: "#3a2d4d",
    text: "#d2a8ff",
  },
  special: {
    fill: "#3a2a1e",
    stroke: "#d29922",
    hover: "#4d3a2d",
    text: "#e3b341",
  },
};

export function statusColor(status: string): string {
  switch (status) {
    case "NORMAL":
      return "#3fb950";
    case "DEAD":
      return "#f85149";
    case "REDIRECT":
      return "#d29922";
    case "UNUSED":
      return "#484f58";
    default:
      return "#8b949e";
  }
}

export const PAGE_SIZE = 8192;
