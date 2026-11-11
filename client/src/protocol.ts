// Binary wire protocol. Keep in sync with server/protocol.go, which documents
// the exact layout of every message.

import type { ClassDef, DeadMsg, Ent, InitMsg, Leaderboard, Me, ServerMsg, StateMsg } from "./types";
import { KIND_BULLET, KIND_SHAPE, KIND_TANK } from "./types";

const MSG_JOIN = 1;
const MSG_INPUT = 2;
const MSG_STAT = 3;
const MSG_UPGRADE = 4;

const MSG_INIT = 1;
const MSG_STATE = 2;
const MSG_DEAD = 3;

const FLAG_MINE = 1 << 0;
const FLAG_SHIELD = 1 << 1;
const FLAG_NAME = 1 << 2;

const INPUT_FIRE = 1 << 0;

const POS_OFFSET = 200;
const POS_SCALE = 10;
const TAU = Math.PI * 2;

const utf8Dec = new TextDecoder();
const utf8Enc = new TextEncoder();

class Reader {
  private i = 0;
  private dv: DataView;
  constructor(private buf: ArrayBuffer) {
    this.dv = new DataView(buf);
  }
  u8(): number {
    return this.dv.getUint8(this.i++);
  }
  u16(): number {
    const v = this.dv.getUint16(this.i, true);
    this.i += 2;
    return v;
  }
  u32(): number {
    const v = this.dv.getUint32(this.i, true);
    this.i += 4;
    return v;
  }
  f32(): number {
    const v = this.dv.getFloat32(this.i, true);
    this.i += 4;
    return v;
  }
  str(): string {
    const n = this.u8();
    const s = utf8Dec.decode(new Uint8Array(this.buf, this.i, n));
    this.i += n;
    return s;
  }
  pos(): number {
    return this.u16() / POS_SCALE - POS_OFFSET;
  }
  angle(): number {
    return (this.u16() / 65535) * TAU;
  }
  frac(): number {
    return this.u8() / 255;
  }
}

/** Decodes server frames. Holds the per-connection tank name cache. */
export class Decoder {
  private names = new Map<number, string>();

  reset(): void {
    this.names.clear();
  }

  decode(buf: ArrayBuffer): ServerMsg {
    const r = new Reader(buf);
    const type = r.u8();
    switch (type) {
      case MSG_INIT:
        return this.init(r);
      case MSG_STATE:
        return this.state(r);
      case MSG_DEAD:
        return this.dead(r);
      default:
        throw new Error(`unknown message type ${type}`);
    }
  }

  private init(r: Reader): InitMsg {
    const id = r.u32();
    const w = r.f32();
    const h = r.f32();
    const mode = r.u8();
    const teams: string[] = [];
    for (let n = r.u8(); n > 0; n--) teams.push(r.str());
    const stats: string[] = [];
    for (let n = r.u8(); n > 0; n--) stats.push(r.str());
    const classes: ClassDef[] = [];
    const nClasses = r.u8();
    for (let id = 0; id < nClasses; id++) {
      const name = r.str();
      const level = r.u8();
      const fov = r.f32();
      const barrels = [];
      for (let n = r.u8(); n > 0; n--) {
        barrels.push({ a: r.f32(), x: r.f32(), l: r.f32(), w: r.f32(), we: r.f32() });
      }
      const upgrades: number[] = [];
      for (let n = r.u8(); n > 0; n--) upgrades.push(r.u8());
      classes.push({ id, name, level, fov, barrels, upgrades });
    }
    return { t: "init", id, w, h, classes, stats, mode, teams };
  }

  private state(r: Reader): StateMsg {
    const me: Me = {
      ent: r.u32(),
      x: r.f32(),
      y: r.f32(),
      hp: r.f32(),
      mhp: r.f32(),
      lvl: r.u8(),
      xp: r.u32(),
      xpn: r.u32(),
      sc: r.u32(),
      pts: r.u8(),
      stats: [r.u8(), r.u8(), r.u8(), r.u8(), r.u8(), r.u8(), r.u8(), r.u8()],
      cls: r.u8(),
      fov: r.f32(),
      alive: r.u8() !== 0,
      team: r.u8(),
      upg: [],
    };
    for (let n = r.u8(); n > 0; n--) me.upg.push(r.u8());

    const nEnts = r.u16();
    const ents: Ent[] = new Array(nEnts);
    for (let k = 0; k < nEnts; k++) {
      const i = r.u32();
      const kind = r.u8();
      const x = r.pos();
      const y = r.pos();
      const rad = r.u16() / 10;
      const e: Ent = { i, k: kind, x, y, r: rad, a: 0, h: 1 };
      if (kind === KIND_TANK) {
        const flags = r.u8();
        e.a = r.angle();
        e.h = r.frac();
        e.c = r.u8();
        e.l = r.u8();
        e.t = r.u8();
        if (flags & FLAG_NAME) this.names.set(i, r.str());
        e.n = this.names.get(i) ?? "";
        e.mine = (flags & FLAG_MINE) !== 0;
        e.p = (flags & FLAG_SHIELD) !== 0;
      } else if (kind === KIND_BULLET) {
        e.mine = (r.u8() & FLAG_MINE) !== 0;
        e.t = r.u8();
      } else if (kind === KIND_SHAPE) {
        e.s = r.u8();
        e.a = r.angle();
        e.h = r.frac();
      }
      ents[k] = e;
    }

    let lb: Leaderboard | null = null;
    if (r.u8()) {
      lb = { rows: [], teams: [] };
      for (let n = r.u8(); n > 0; n--) {
        lb.rows.push({ n: r.str(), s: r.u32(), c: r.u8(), t: r.u8(), me: r.u8() !== 0 });
      }
      for (let n = r.u8(); n > 0; n--) lb.teams.push(r.u32());
    }
    return { t: "state", ents, me, lb };
  }

  private dead(r: Reader): DeadMsg {
    return { t: "dead", score: r.u32(), level: r.u8(), killer: r.str(), kills: r.u16() };
  }
}

// ---------- client -> server ----------

export function encodeJoin(name: string): ArrayBuffer {
  // the server keeps 16 runes; cap bytes so the u8 length can't overflow
  let bytes = utf8Enc.encode(name);
  if (bytes.length > 255) bytes = bytes.subarray(0, 255);
  const out = new Uint8Array(2 + bytes.length);
  out[0] = MSG_JOIN;
  out[1] = bytes.length;
  out.set(bytes, 2);
  return out.buffer;
}

export function encodeInput(dx: number, dy: number, aim: number, fire: boolean): ArrayBuffer {
  const dv = new DataView(new ArrayBuffer(6));
  dv.setUint8(0, MSG_INPUT);
  dv.setInt8(1, Math.round(clampN(dx, -1, 1) * 127));
  dv.setInt8(2, Math.round(clampN(dy, -1, 1) * 127));
  dv.setUint16(3, encodeAngle(aim), true);
  dv.setUint8(5, fire ? INPUT_FIRE : 0);
  return dv.buffer;
}

export function encodeStat(i: number): ArrayBuffer {
  return new Uint8Array([MSG_STAT, i & 255]).buffer;
}

export function encodeUpgrade(c: number): ArrayBuffer {
  return new Uint8Array([MSG_UPGRADE, c & 255]).buffer;
}

export function encodeAngle(a: number): number {
  a %= TAU;
  if (a < 0) a += TAU;
  return Math.round((a / TAU) * 65535);
}

function clampN(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v;
}
