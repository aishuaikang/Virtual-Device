import type { SVGProps } from "react";

const paths = {
  radio:
    "M5 16a10 10 0 0 1 0-8m14 0a10 10 0 0 1 0 8M8 14a5 5 0 0 1 0-4m8 0a5 5 0 0 1 0 4M12 12v8m-3 0h6",
  play: "m9 5 10 7-10 7Z",
  stop: "M6 6h12v12H6Z",
  plus: "M12 5v14M5 12h14",
  close: "m6 6 12 12M6 18 18 6",
  chevron: "m9 5 7 7-7 7",
  more: "M5 12h.01M12 12h.01M19 12h.01",
  trash: "M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7m4-7v7",
  save: "M5 3h12l4 4v14H3V3h2Zm2 0v6h10V3M7 21v-8h10v8",
  map: "m3 6 6-3 6 3 6-3v15l-6 3-6-3-6 3V6Zm6-3v15m6-12v15",
  search: "M21 21l-5-5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0Z",
  download: "M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5",
  log: "m4 6 4 4-4 4m8 2h8",
} as const;

export type IconName = keyof typeof paths;

export default function Icon({
  name,
  ...props
}: SVGProps<SVGSVGElement> & { name: IconName }) {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d={paths[name]} />
    </svg>
  );
}
