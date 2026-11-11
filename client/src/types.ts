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
  i: number; // entity id
  k: number; // kind
  x: number;
  y: number;
  r: number; // radius
  a: number; // rotation
  h: number; // hp fraction 0..1
  s?: number; // shape sides
  c?: number; // tank class
  l?: number; // tank level
  n?: string; // tank name
  p?: boolean; // tank spawn shield
  mine?: boolean; // belongs to the viewing player
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
  me: boolean;
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
  ents: Ent[];
  me: Me;
  lb: LBEntry[] | null; // null = unchanged since the last frame
}

export interface DeadMsg {
  t: "dead";
  score: number;
  level: number;
  killer: string;
  kills: number;
}

export type ServerMsg = InitMsg | StateMsg | DeadMsg;
