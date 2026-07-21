import React, { useState } from "react";

type Entry = {
  name: string;
  reference: string;
  isDir: boolean;
};

type Props = {
  files: ReadonlyArray<Entry | null | undefined>;
  symbols: ReadonlyArray<Entry | null | undefined>;
  activeRef: string;
  onSelect: (ref: string) => void;
  symbolsLoading?: boolean;
};

export function FileRail({
  files,
  symbols,
  activeRef,
  onSelect,
  symbolsLoading,
}: Props) {
  const [tab, setTab] = useState<"files" | "symbols">("files");

  return (
    <div className="p-2 flex flex-col gap-2">
      <div role="tablist" className="tabs tabs-box tabs-sm w-full">
        <button
          type="button"
          role="tab"
          className={`tab flex-1 ${tab === "files" ? "tab-active" : ""}`}
          onClick={() => setTab("files")}
        >
          Files
        </button>
        <button
          type="button"
          role="tab"
          className={`tab flex-1 ${tab === "symbols" ? "tab-active" : ""}`}
          onClick={() => setTab("symbols")}
        >
          Symbols
          {symbolsLoading ? (
            <span className="loading loading-spinner loading-xs ml-1" />
          ) : null}
        </button>
      </div>

      {tab === "files" ? (
        <ul className="menu menu-sm bg-base-200 rounded-box w-full p-0">
          {files.filter(Boolean).map((f) => (
            <li key={f!.reference + f!.name}>
              <a
                href={"/code/" + encodeURIComponent(f!.reference)}
                className={f!.reference === activeRef ? "menu-active" : ""}
                onClick={(e) => {
                  e.preventDefault();
                  onSelect(f!.reference);
                }}
              >
                {f!.isDir ? (
                  <span className="badge badge-ghost badge-xs">dir</span>
                ) : null}
                <span className="truncate">{f!.name}</span>
              </a>
            </li>
          ))}
          {!files.length ? (
            <li className="menu-disabled">
              <span className="text-base-content/50">—</span>
            </li>
          ) : null}
        </ul>
      ) : (
        <ul className="menu menu-sm bg-base-200 rounded-box w-full p-0">
          {symbols.filter(Boolean).map((s) => (
            <li key={s!.reference + s!.name}>
              <button
                type="button"
                className={s!.reference === activeRef ? "menu-active" : ""}
                onClick={() => onSelect(s!.reference)}
              >
                <span className="truncate">{s!.name}</span>
              </button>
            </li>
          ))}
          {!symbols.length ? (
            <li className="menu-disabled">
              <span className="text-base-content/50">
                {symbolsLoading ? "Loading symbols…" : "Open a file to list symbols."}
              </span>
            </li>
          ) : null}
        </ul>
      )}
    </div>
  );
}
