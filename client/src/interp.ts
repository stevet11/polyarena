import type { Ent, StateMsg } from "./types";

interface Snapshot {
  t: number; // local receive time (ms)
  ents: Map<number, Ent>;
}

const DELAY_MS = 110;

/** Buffers server snapshots and produces interpolated entity positions. */
export class Interpolator {
  private snaps: Snapshot[] = [];

  push(msg: StateMsg): void {
    const ents = new Map<number, Ent>();
    for (const e of msg.ents) ents.set(e.i, e);
    this.snaps.push({ t: performance.now(), ents });
    if (this.snaps.length > 8) this.snaps.shift();
  }

  /** Returns entities to draw for the current frame. */
  sample(now: number): Ent[] {
    const n = this.snaps.length;
    if (n === 0) return [];
    const rt = now - DELAY_MS;
    let a = this.snaps[n - 1];
    let b: Snapshot | null = null;
    for (let i = n - 1; i > 0; i--) {
      if (this.snaps[i - 1].t <= rt) {
        a = this.snaps[i - 1];
        b = this.snaps[i];
        break;
      }
    }
    if (!b) {
      // rt is older than everything we have, or newer than the latest: use the latest
      return Array.from(this.snaps[n - 1].ents.values());
    }
    const span = b.t - a.t;
    const f = span > 0 ? Math.min(1, Math.max(0, (rt - a.t) / span)) : 1;
    const out: Ent[] = [];
    for (const [id, eb] of b.ents) {
      const ea = a.ents.get(id);
      if (!ea) {
        out.push(eb);
        continue;
      }
      out.push({
        ...eb,
        x: ea.x + (eb.x - ea.x) * f,
        y: ea.y + (eb.y - ea.y) * f,
        a: lerpAngle(ea.a, eb.a, f),
        h: ea.h + (eb.h - ea.h) * f,
      });
    }
    return out;
  }
}

function lerpAngle(a: number, b: number, f: number): number {
  let d = b - a;
  while (d > Math.PI) d -= Math.PI * 2;
  while (d < -Math.PI) d += Math.PI * 2;
  return a + d * f;
}
