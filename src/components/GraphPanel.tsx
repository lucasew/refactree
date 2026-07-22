import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph2D from "react-force-graph-2d";
import {
  getGraphSession,
  isExternalId,
  mergeNeighborhood,
  snapshotGraphData,
  viewFocusId,
  type IncomingNeighborhood,
} from "../graphSession";
import {
  ensureGraphSession,
  setSessionCrawl,
  sessionVisit,
  streamExpandExternal,
  getGraphSessionClient,
} from "../graphStream";
import { nodeFill, nodeStroke, inferLanguageFromId } from "../graphColors";
import {
  formatGraphLabel,
  normalizeRef,
  type GraphViewMode,
} from "../routes";
import {
  computeUsage,
  forceUsageGravity,
  forceUsageRadial,
  forceNodeDensity,
  forceEdgeCrossing,
  degreeRadiusBoost,
} from "../graphForces";

type Props = {
  focusId: string;
  neighborhood?: IncomingNeighborhood;
  /** Visit this ref in the shared graph session (deltas). */
  streamRef?: string | null;
  /** Start with "crawl repo" on (project graph page). */
  streamProject?: boolean;
  loading?: boolean;
  onFocus: (ref: string) => void;
  onExpandExternal?: (ref: string) => void;
};

const BG = "#1a1814";

/** Tooltip: crawl uses ingest skip list (node_modules, .venv, …). */
const CRAWL_TITLE =
  "When on: crawls the repo in parallel with click-to-expand. Skips node_modules, .venv, vendor, dist, .git, …";

export function GraphPanel({
  focusId,
  neighborhood,
  streamRef,
  streamProject,
  loading: loadingProp,
  onFocus,
  onExpandExternal,
}: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<any>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [tick, setTick] = useState(0);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  /**
   * module = one node per module (path:./cmd/rft once)
   * atom = full atom refs (path:./cmd/rft::Main distinct)
   */
  const [viewMode, setViewMode] = useState<GraphViewMode>("atom");
  /** Full-tree crawl of the project (DiscoverDir recursive; skip list in ingest). */
  const [crawlRepo, setCrawlRepo] = useState(!!streamProject);
  const lastVisit = useRef<string>("");
  const crawlGen = useRef(0);
  const bumpRaf = useRef(0);
  const lastPaintAt = useRef(0);
  const lastReheatAt = useRef(0);
  const trailingBump = useRef<ReturnType<typeof setTimeout> | null>(null);
  const graphDataRef = useRef({ nodes: [] as ReturnType<typeof snapshotGraphData>["nodes"], links: [] as ReturnType<typeof snapshotGraphData>["links"] });
  const usageRef = useRef(new Map<string, number>());

  // Coalesce stream floods: ~10 paints/s, rare simulation reheats (was every edge → flicker).
  const bump = useCallback((opts?: { reheat?: boolean }) => {
    const paint = () => {
      lastPaintAt.current = performance.now();
      setTick((t) => t + 1);
      if (opts?.reheat) {
        const now = performance.now();
        if (now - lastReheatAt.current > 450) {
          lastReheatAt.current = now;
          try {
            fgRef.current?.d3ReheatSimulation?.();
          } catch {
            /* ignore */
          }
        }
      }
    };
    const now = performance.now();
    const minGap = 100; // ms between React graphData snapshots
    if (now - lastPaintAt.current >= minGap) {
      if (bumpRaf.current) cancelAnimationFrame(bumpRaf.current);
      bumpRaf.current = requestAnimationFrame(() => {
        bumpRaf.current = 0;
        paint();
      });
      return;
    }
    if (trailingBump.current) return;
    trailingBump.current = setTimeout(() => {
      trailingBump.current = null;
      paint();
    }, minGap - (now - lastPaintAt.current));
  }, []);

  useEffect(() => {
    return () => {
      if (bumpRaf.current) cancelAnimationFrame(bumpRaf.current);
      if (trailingBump.current) clearTimeout(trailingBump.current);
    };
  }, []);

  useEffect(() => {
    ensureGraphSession();
  }, []);

  useEffect(() => {
    const el = hostRef.current;
    if (!el) return;
    const measure = () => {
      const r = el.getBoundingClientRect();
      const w = Math.max(0, Math.floor(r.width));
      const h = Math.max(0, Math.floor(r.height));
      setSize((prev) => (prev.w === w && prev.h === h ? prev : { w, h }));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Sticky crawl: worker auto-runs project when free (only while toggle is on).
  useEffect(() => {
    const gen = ++crawlGen.current;
    let cancelled = false;
    if (!crawlRepo) {
      void setSessionCrawl(false);
      return () => {
        cancelled = true;
      };
    }
    setBusy(true);
    setErr(null);
    void setSessionCrawl(true, {
      onEvent: () => {
        if (!cancelled) bump();
      },
      onError: (e) => {
        if (!cancelled) setErr(e.message);
      },
      onDone: () => {
        if (!cancelled) bump({ reheat: true });
      },
    }).finally(() => {
      if (!cancelled && gen === crawlGen.current) setBusy(false);
    });
    const unsub = getGraphSessionClient().subscribe((msg) => {
      if (cancelled) return;
      // Paint on stream traffic; reheat only on batch/done so the sim can cool.
      if (msg.type === "edge" || msg.type === "edges" || msg.type === "focus") {
        bump(msg.type === "edges" ? { reheat: true } : undefined);
      }
      if (msg.type === "done") bump({ reheat: true });
    });
    return () => {
      cancelled = true;
      unsub();
      void setSessionCrawl(false);
    };
  }, [crawlRepo, bump]);

  // Visit focus: same WS session, only new edges arrive (works with crawl on).
  useEffect(() => {
    if (streamRef == null || streamRef === "") return;
    const visitRef = normalizeRef(streamRef);
    lastVisit.current = visitRef;
    let cancelled = false;
    setBusy(true);
    setErr(null);
    sessionVisit(visitRef, {
      onEvent: () => {
        // Visit is hop-scoped (cheap); paint without continuous reheat spam.
        if (!cancelled) bump();
      },
      onError: (e) => {
        if (!cancelled) setErr(e.message);
      },
      onDone: () => {
        if (!cancelled) bump({ reheat: true });
      },
    }).finally(() => {
      if (!cancelled) setBusy(false);
    });
    return () => {
      cancelled = true;
    };
  }, [streamRef, bump]);

  useEffect(() => {
    if (!neighborhood || streamRef != null || crawlRepo) return;
    mergeNeighborhood(neighborhood, focusId);
    bump({ reheat: true });
  }, [neighborhood, focusId, streamRef, crawlRepo, bump]);

  useEffect(() => {
    getGraphSession().focusId = normalizeRef(focusId);
    bump();
  }, [focusId, bump]);

  // Projections are incremental; snapshot is O(mode size), not a full session rescan.
  const graphData = useMemo(() => {
    void tick;
    return snapshotGraphData(viewMode);
  }, [tick, viewMode]);

  graphDataRef.current = graphData;

  const focusInView = useMemo(
    () => viewFocusId(getGraphSession().focusId || focusId, viewMode),
    [tick, focusId, viewMode]
  );

  // Indegree usage: imported/used-by-many → center; unused → rim.
  const usage = useMemo(() => computeUsage(graphData.links), [graphData.links]);
  usageRef.current = usage;

  // Install forces when size / emptiness changes — NOT on every stream tick (that was flickery).
  const hasNodes = graphData.nodes.length > 0;
  useEffect(() => {
    const fg = fgRef.current;
    if (!fg || !hasNodes) return;

    const getUse = (id: string) => usageRef.current.get(id) ?? 0;

    try {
      const center = fg.d3Force("center");
      if (center && typeof center.strength === "function") {
        center.strength(0.015);
      }
    } catch {
      /* ignore */
    }

    try {
      const charge = fg.d3Force("charge");
      if (charge && typeof charge.strength === "function") {
        charge.strength((node: { id?: string }) => {
          const u = getUse(node.id ?? "");
          // Stronger global repulsion so pairs keep more distance.
          return -40 - Math.max(0, 10 - u) * 6;
        });
      }
    } catch {
      /* ignore */
    }

    try {
      const link = fg.d3Force("link");
      if (link && typeof link.distance === "function") {
        link.distance((l: { source?: { id?: string } | string; target?: { id?: string } | string }) => {
          const idOf = (x: unknown) =>
            typeof x === "string" ? x : (x as { id?: string })?.id ?? "";
          const a = getUse(idOf(l.source));
          const b = getUse(idOf(l.target));
          const avg = (a + b) / 2;
          return 58 + Math.max(0, 14 - avg) * 4;
        });
      }
    } catch {
      /* ignore */
    }

    // Wider neighborhood + stronger pairwise push = denser clusters open up.
    const densR = Math.max(56, Math.min(size.w, size.h) * 0.12 || 72);
    fg.d3Force(
      "usageGravity",
      forceUsageGravity(getUse, {
        densityRadius: densR,
        densityWeight: 0.85,
      })
    );
    fg.d3Force(
      "usageRadial",
      forceUsageRadial(getUse, {
        outer: Math.min(size.w, size.h) * 0.42 || 220,
        inner: 18,
        strength: 0.14,
        densityRadius: densR,
        densitySpread: 56,
      })
    );
    fg.d3Force(
      "nodeDensity",
      forceNodeDensity({
        radius: densR,
        strength: 0.75,
      })
    );
    fg.d3Force(
      "edgeCrossing",
      forceEdgeCrossing(() => graphDataRef.current.links, {
        strength: 0.14,
        maxLinks: 96,
      })
    );

    try {
      fg.d3ReheatSimulation?.();
      lastReheatAt.current = performance.now();
    } catch {
      /* ignore */
    }
  }, [hasNodes, size.w, size.h, viewMode]);

  const onNodeClick = useCallback(
    (node: { id: string; expandable?: boolean; external?: boolean }) => {
      const id = normalizeRef(node.id);
      const external = node.external || isExternalId(id);
      // Always open definition in the code pane (path + external) — same as source hyperlinks.
      onFocus(id);
      // Grow graph neighborhood for unexpanded external stubs (do not block navigation).
      if (external && node.expandable !== false) {
        if (onExpandExternal) {
          onExpandExternal(id);
          return;
        }
        setBusy(true);
        streamExpandExternal(id, { onEvent: bump })
          .catch((e) => setErr(String(e)))
          .finally(() => setBusy(false));
      }
    },
    [onFocus, onExpandExternal, bump]
  );

  const loading = loadingProp || busy;
  const sess = getGraphSession();

  // Always mount the force canvas (empty graph is fine) — no placeholder copy.
  return (
    <div ref={hostRef} className="graph-canvas-host relative h-full min-h-64 overflow-hidden">
      <div className="absolute z-10 m-2 flex flex-wrap items-center gap-1">
        <span className="badge badge-ghost badge-sm">
          {graphData.nodes.length}n · {graphData.links.length}e
        </span>
        {sess.incomplete ? (
          <span className="badge badge-ghost badge-sm">incomplete</span>
        ) : null}
        {loading ? (
          <span className="badge badge-primary badge-outline badge-sm gap-1">
            <span className="loading loading-spinner loading-xs" />
            session
          </span>
        ) : (
          <span className="badge badge-ghost badge-sm opacity-70">session live</span>
        )}
        {err ? <span className="badge badge-error badge-sm">{err}</span> : null}
        <div className="join" role="group" aria-label="Graph view mode">
          <button
            type="button"
            className={`btn btn-xs join-item ${viewMode === "module" ? "btn-active" : ""}`}
            title="Modules only (import graph)"
            onClick={() => setViewMode("module")}
          >
            module
          </button>
          <button
            type="button"
            className={`btn btn-xs join-item ${viewMode === "atom" ? "btn-active" : ""}`}
            title="Atoms only (use graph)"
            onClick={() => setViewMode("atom")}
          >
            atom
          </button>
        </div>
        <label
          className="label cursor-pointer gap-1.5 py-0 px-1"
          title={CRAWL_TITLE}
        >
          <span className="label-text text-xs text-base-content/80">crawl repo</span>
          <input
            type="checkbox"
            className="toggle toggle-xs"
            checked={crawlRepo}
            onChange={(e) => setCrawlRepo(e.target.checked)}
            aria-label="Crawl current repository"
          />
        </label>
      </div>
      {size.w > 0 && size.h > 0 ? (
        <ForceGraph2D
          ref={fgRef}
          width={size.w}
          height={size.h}
          graphData={graphData}
          nodeId="id"
          nodeLabel={(n: any) => {
            const lang = n.language || inferLanguageFromId(n.id) || "?";
            const scope = n.external ? "external" : "path";
            const tip = " — click to open source";
            const pretty = formatGraphLabel(n.id, viewMode).replace(/\n/g, " · ");
            const u = usage.get(n.id) ?? 0;
            return `${pretty} [${lang} · ${scope} · used by ${u}]${tip}`;
          }}
          backgroundColor={BG}
          linkWidth={viewMode === "module" ? 1.5 : 1.25}
          linkColor={(l: any) => {
            const kind = l.kind as string;
            if (kind === "IMPORTS") return "#7a8aaa";
            if (kind === "USED_BY") return "#aa8a7a";
            return "#8aaa7a";
          }}
          // Package view: clear directed imports (importer → importee).
          linkDirectionalArrowLength={viewMode === "module" ? 7 : 4}
          linkDirectionalArrowRelPos={1}
          linkDirectionalArrowColor={() =>
            viewMode === "module" ? "#a8b8d8" : "#8aaa7a"
          }
          linkCurvature={viewMode === "module" ? 0.15 : 0.05}
          linkDirectionalParticles={viewMode === "module" ? 1 : 0}
          linkDirectionalParticleWidth={viewMode === "module" ? 2 : 0}
          linkDirectionalParticleSpeed={0.004}
          nodeCanvasObjectMode={() => "replace"}
          nodeCanvasObject={(node: any, ctx, globalScale) => {
            const label = formatGraphLabel(node.id, viewMode);
            const fontSize = Math.max(10 / globalScale, 2);
            const baseR =
              viewMode === "module"
                ? 7
                : node.kind === "ATOM"
                  ? 4
                  : node.kind === "MODULE"
                    ? 7
                    : 5;
            const r = degreeRadiusBoost(usage.get(node.id) ?? 0, baseR);
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            const lang = node.language || inferLanguageFromId(node.id);
            const focused = node.id === focusInView;
            const external = !!node.external;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, 2 * Math.PI, false);
            ctx.fillStyle = nodeFill({
              language: lang,
              external,
              focused,
            });
            ctx.fill();
            const stroke = nodeStroke({
              external,
              focused,
              expandable: node.expandable,
            });
            if (stroke) {
              ctx.strokeStyle = stroke.color;
              ctx.lineWidth = stroke.width / globalScale;
              if (external && !focused) {
                ctx.setLineDash([3 / globalScale, 2 / globalScale]);
              } else {
                ctx.setLineDash([]);
              }
              ctx.stroke();
              ctx.setLineDash([]);
            }
            if (globalScale > 0.55) {
              const lines = String(label).split("\n").filter(Boolean);
              ctx.textAlign = "center";
              ctx.textBaseline = "top";
              let yy = y + r + 2;
              lines.forEach((line: string, i: number) => {
                // module line muted when atom view shows two lines
                if (lines.length > 1 && i === 0) {
                  ctx.font = `${Math.max(fontSize * 0.85, 1.5)}px sans-serif`;
                  ctx.fillStyle = "rgba(232,224,208,0.7)";
                } else {
                  ctx.font = `${fontSize}px sans-serif`;
                  ctx.fillStyle = "#e8e0d0";
                }
                ctx.fillText(line, x, yy);
                yy += fontSize * 1.15;
              });
            }
          }}
          nodePointerAreaPaint={(node: any, color, ctx) => {
            ctx.beginPath();
            ctx.arc(node.x ?? 0, node.y ?? 0, 10, 0, 2 * Math.PI, false);
            ctx.fillStyle = color;
            ctx.fill();
          }}
          onNodeClick={onNodeClick}
          cooldownTicks={80}
          // Pause canvas when the sim cools — continuous redraw pegs CPU after crawl.
          autoPauseRedraw={true}
          enableNodeDrag={true}
        />
      ) : null}
    </div>
  );
}
