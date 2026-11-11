import { type ClassDef, type Ent, KIND_BULLET, KIND_SHAPE, KIND_TANK } from "./types";

export const COLORS = {
  bg: "#cdcdcd",
  grid: "#c0c0c0",
  outside: "rgba(0,0,0,0.16)",
  self: "#00b2e1",
  enemy: "#f14e54",
  barrel: "#999999",
  square: "#ffe869",
  triangle: "#fc7677",
  pentagon: "#768dfc",
  alpha: "#5b6fe0",
  hpBack: "#4a4a4a",
  hp: "#86c280",
};

const OUTLINE_ALPHA = 0.5;

function darken(hex: string, amount = 0.25): string {
  const n = parseInt(hex.slice(1), 16);
  const r = Math.round(((n >> 16) & 255) * (1 - amount));
  const g = Math.round(((n >> 8) & 255) * (1 - amount));
  const b = Math.round((n & 255) * (1 - amount));
  return `rgb(${r},${g},${b})`;
}

export class Renderer {
  ctx: CanvasRenderingContext2D;
  width = 0;
  height = 0;
  dpr = 1;

  constructor(public canvas: HTMLCanvasElement) {
    this.ctx = canvas.getContext("2d")!;
    this.resize();
    window.addEventListener("resize", () => this.resize());
  }

  resize(): void {
    this.dpr = Math.min(2, window.devicePixelRatio || 1);
    this.width = window.innerWidth;
    this.height = window.innerHeight;
    this.canvas.width = Math.floor(this.width * this.dpr);
    this.canvas.height = Math.floor(this.height * this.dpr);
    this.canvas.style.width = `${this.width}px`;
    this.canvas.style.height = `${this.height}px`;
  }

  /** Zoom so that a (2300*fov x 1400*fov) world area always fits the viewport. */
  zoomFor(fov: number): number {
    return Math.max(this.width / 2300, this.height / 1400) / fov;
  }

  draw(
    ents: Ent[],
    cam: { x: number; y: number; zoom: number },
    world: { w: number; h: number },
    classes: ClassDef[],
    myId: number,
  ): void {
    const { ctx, width, height, dpr } = this;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = COLORS.bg;
    ctx.fillRect(0, 0, width, height);

    ctx.save();
    ctx.translate(width / 2, height / 2);
    ctx.scale(cam.zoom, cam.zoom);
    ctx.translate(-cam.x, -cam.y);

    // visible world rect
    const vx0 = cam.x - width / 2 / cam.zoom;
    const vy0 = cam.y - height / 2 / cam.zoom;
    const vx1 = cam.x + width / 2 / cam.zoom;
    const vy1 = cam.y + height / 2 / cam.zoom;

    // grid
    const step = 50;
    ctx.strokeStyle = COLORS.grid;
    ctx.lineWidth = 1 / cam.zoom;
    ctx.beginPath();
    for (let x = Math.floor(vx0 / step) * step; x <= vx1; x += step) {
      ctx.moveTo(x, vy0);
      ctx.lineTo(x, vy1);
    }
    for (let y = Math.floor(vy0 / step) * step; y <= vy1; y += step) {
      ctx.moveTo(vx0, y);
      ctx.lineTo(vx1, y);
    }
    ctx.stroke();

    // out-of-bounds shading
    ctx.fillStyle = COLORS.outside;
    if (vx0 < 0) ctx.fillRect(vx0, vy0, -vx0, vy1 - vy0);
    if (vx1 > world.w) ctx.fillRect(world.w, vy0, vx1 - world.w, vy1 - vy0);
    if (vy0 < 0) ctx.fillRect(Math.max(vx0, 0), vy0, Math.min(vx1, world.w) - Math.max(vx0, 0), -vy0);
    if (vy1 > world.h)
      ctx.fillRect(Math.max(vx0, 0), world.h, Math.min(vx1, world.w) - Math.max(vx0, 0), vy1 - world.h);

    // draw order: shapes, bullets, tanks
    const order = (e: Ent) => (e.k === KIND_SHAPE ? 0 : e.k === KIND_BULLET ? 1 : 2);
    const sorted = ents.slice().sort((a, b) => order(a) - order(b));

    ctx.lineJoin = "round";
    for (const e of sorted) {
      switch (e.k) {
        case KIND_SHAPE:
          this.drawShape(e);
          break;
        case KIND_BULLET:
          this.drawBullet(e, e.o === myId);
          break;
        case KIND_TANK:
          this.drawTank(e, classes[e.c], e.o === myId);
          break;
      }
    }
    // overlays (names, hp bars) on top of everything
    for (const e of sorted) this.drawOverlay(e, cam.zoom);

    ctx.restore();
  }

  private fillStroke(color: string, lw: number): void {
    const { ctx } = this;
    ctx.fillStyle = color;
    ctx.fill();
    ctx.lineWidth = lw;
    ctx.strokeStyle = darken(color, OUTLINE_ALPHA);
    ctx.stroke();
  }

  private drawShape(e: Ent): void {
    const { ctx } = this;
    const sides = e.s ?? 4;
    const color =
      sides === 3 ? COLORS.triangle : sides === 4 ? COLORS.square : e.r > 80 ? COLORS.alpha : COLORS.pentagon;
    ctx.save();
    ctx.translate(e.x, e.y);
    ctx.rotate(e.a);
    ctx.beginPath();
    for (let i = 0; i < sides; i++) {
      const t = (i / sides) * Math.PI * 2;
      const px = Math.cos(t) * e.r;
      const py = Math.sin(t) * e.r;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.closePath();
    this.fillStroke(color, Math.max(2, e.r * 0.12));
    ctx.restore();
  }

  private drawBullet(e: Ent, mine: boolean): void {
    const { ctx } = this;
    ctx.beginPath();
    ctx.arc(e.x, e.y, e.r, 0, Math.PI * 2);
    this.fillStroke(mine ? COLORS.self : COLORS.enemy, Math.max(2, e.r * 0.25));
  }

  private drawTank(e: Ent, cls: ClassDef | undefined, mine: boolean): void {
    const { ctx } = this;
    const R = e.r;
    ctx.save();
    ctx.translate(e.x, e.y);
    ctx.rotate(e.a);
    if (cls) {
      for (const b of cls.barrels) {
        ctx.save();
        ctx.rotate(b.a);
        ctx.translate(0, b.x * R);
        const w0 = b.w * R;
        const w1 = b.we * R;
        const len = b.l * R;
        ctx.beginPath();
        ctx.moveTo(0, -w0 / 2);
        ctx.lineTo(len, -w1 / 2);
        ctx.lineTo(len, w1 / 2);
        ctx.lineTo(0, w0 / 2);
        ctx.closePath();
        this.fillStroke(COLORS.barrel, 3);
        ctx.restore();
      }
    }
    ctx.beginPath();
    ctx.arc(0, 0, R, 0, Math.PI * 2);
    this.fillStroke(mine ? COLORS.self : COLORS.enemy, 3);
    if (e.p) {
      ctx.beginPath();
      ctx.arc(0, 0, R + 6, 0, Math.PI * 2);
      ctx.strokeStyle = "rgba(255,255,255,0.7)";
      ctx.lineWidth = 3;
      ctx.setLineDash([6, 6]);
      ctx.stroke();
    }
    ctx.restore();
  }

  private drawOverlay(e: Ent, zoom: number): void {
    const { ctx } = this;
    if (e.k === KIND_BULLET) return;
    const damaged = e.h < e.m - 0.5;
    if (e.k === KIND_TANK) {
      ctx.save();
      ctx.font = `bold ${Math.max(14, 16 / Math.sqrt(zoom))}px Ubuntu, Arial, sans-serif`;
      ctx.textAlign = "center";
      ctx.textBaseline = "bottom";
      ctx.lineWidth = 4;
      ctx.strokeStyle = "rgba(0,0,0,0.6)";
      ctx.fillStyle = "#fff";
      const label = e.n ?? "";
      ctx.strokeText(label, e.x, e.y - e.r - 12);
      ctx.fillText(label, e.x, e.y - e.r - 12);
      ctx.restore();
    }
    if (damaged) {
      const w = e.r * 2;
      const h = Math.max(4, e.r * 0.2);
      const x = e.x - e.r;
      const y = e.y + e.r + 8;
      ctx.fillStyle = COLORS.hpBack;
      roundRect(ctx, x - 1, y - 1, w + 2, h + 2, h / 2 + 1);
      ctx.fill();
      ctx.fillStyle = COLORS.hp;
      roundRect(ctx, x, y, Math.max(0, (w * e.h) / e.m), h, h / 2);
      ctx.fill();
    }
  }
}

function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number): void {
  r = Math.min(r, w / 2, h / 2);
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + w - r, y);
  ctx.arc(x + w - r, y + r, r, -Math.PI / 2, 0);
  ctx.lineTo(x + w, y + h - r);
  ctx.arc(x + w - r, y + h - r, r, 0, Math.PI / 2);
  ctx.lineTo(x + r, y + h);
  ctx.arc(x + r, y + h - r, r, Math.PI / 2, Math.PI);
  ctx.lineTo(x, y + r);
  ctx.arc(x + r, y + r, r, Math.PI, Math.PI * 1.5);
  ctx.closePath();
}

/** Minimap: world bounds with own position. */
export function drawMinimap(
  canvas: HTMLCanvasElement,
  world: { w: number; h: number },
  me: { x: number; y: number } | null,
): void {
  const ctx = canvas.getContext("2d")!;
  const size = canvas.width;
  ctx.clearRect(0, 0, size, size);
  ctx.fillStyle = "rgba(205,205,205,0.95)";
  ctx.fillRect(0, 0, size, size);
  ctx.strokeStyle = "rgba(0,0,0,0.4)";
  ctx.lineWidth = 3;
  ctx.strokeRect(1.5, 1.5, size - 3, size - 3);
  if (me) {
    ctx.fillStyle = "#222";
    ctx.beginPath();
    ctx.arc((me.x / world.w) * size, (me.y / world.h) * size, 4, 0, Math.PI * 2);
    ctx.fill();
  }
}
