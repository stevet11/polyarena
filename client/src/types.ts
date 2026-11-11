export interface Barrel {
  a: number; // angle (rad) relative to aim
  x: number; // lateral offset, in tank radii
  l: number; // length, in tank radii
  w: number; // base width, in tank radii
  we: number; // muzzle width
}

export interface ClassDef {
  id: number;
  name: string;
  level: number;
  fov: number;
  barrels: Barrel[];
  upgrades: number[];
}

export const KIND_TANK = 0;
export const KIND_BULLET = 1;
export const KIND_SHAPE = 2;

export interface Ent {
  i: number;
  k: number;
  x: number;
  y: number;
  r: number;
  a: number;
  h: number;
  m: number;
  s?: number;
  c: number;
  o?: number;
  n?: string;
  l?: number;
  p?: boolean;
}

export interface Me {
  ent: number;
  x: number;
  y: number;
  hp: number;
  mhp: number;
  lvl: number;
  xp: number;
  xpn: number;
  sc: number;
  pts: number;
  stats: number[];
  cls: number;
  fov: number;
  upg: number[];
  alive: boolean;
}

export interface LBEntry {
  n: string;
  s: number;
  c: number;
  me?: boolean;
}

export interface InitMsg {
  t: "init";
  id: number;
  w: number;
  h: number;
  classes: ClassDef[];
  stats: string[];
}

export interface StateMsg {
  t: "state";
  ts: number;
  ents: Ent[];
  me: Me;
  lb: LBEntry[];
}

export interface DeadMsg {
  t: "dead";
  score: number;
  level: number;
  killer: string;
  kills: number;
}

export type ServerMsg = InitMsg | StateMsg | DeadMsg;
