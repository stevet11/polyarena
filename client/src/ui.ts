import type { ClassDef, LBEntry, Me } from "./types";

const $ = <T extends HTMLElement>(id: string): T => document.getElementById(id) as T;

const STAT_KEYS = ["1", "2", "3", "4", "5", "6", "7", "8"];
const CLASS_KEYS = ["Y", "U", "I", "O"];

export class UI {
  start = $<HTMLDivElement>("start");
  nameInput = $<HTMLInputElement>("name");
  playBtn = $<HTMLButtonElement>("play");
  death = $<HTMLDivElement>("death");
  deathText = $<HTMLDivElement>("death-text");
  respawnBtn = $<HTMLButtonElement>("respawn");
  hud = $<HTMLDivElement>("hud");
  stats = $<HTMLDivElement>("stats");
  classes = $<HTMLDivElement>("classes");
  lb = $<HTMLDivElement>("leaderboard");
  scoreText = $<HTMLDivElement>("score");
  levelText = $<HTMLDivElement>("level");
  xpFill = $<HTMLDivElement>("xp-fill");
  pointsText = $<HTMLDivElement>("points");
  status = $<HTMLDivElement>("status");
  minimap = $<HTMLCanvasElement>("minimap");

  onStat: (i: number) => void = () => {};
  onClass: (c: number) => void = () => {};

  private statRows: { fill: HTMLDivElement[]; btn: HTMLButtonElement }[] = [];
  private classDefs: ClassDef[] = [];
  private lastUpg = "";

  buildStats(names: string[]): void {
    this.stats.innerHTML = "";
    this.statRows = [];
    names.forEach((name, i) => {
      const row = document.createElement("div");
      row.className = "stat";
      const label = document.createElement("span");
      label.className = "stat-name";
      label.textContent = name;
      const bar = document.createElement("div");
      bar.className = "stat-bar";
      const fill: HTMLDivElement[] = [];
      for (let j = 0; j < 7; j++) {
        const seg = document.createElement("div");
        seg.className = "seg";
        seg.style.setProperty("--c", STAT_COLORS[i]);
        bar.appendChild(seg);
        fill.push(seg);
      }
      const btn = document.createElement("button");
      btn.textContent = `+ [${STAT_KEYS[i]}]`;
      btn.onclick = (ev) => {
        ev.preventDefault();
        this.onStat(i);
      };
      row.append(label, bar, btn);
      this.stats.appendChild(row);
      this.statRows.push({ fill, btn });
    });
  }

  setClasses(defs: ClassDef[]): void {
    this.classDefs = defs;
  }

  updateMe(me: Me): void {
    this.scoreText.textContent = `Score: ${me.sc.toLocaleString()}`;
    const cls = this.classDefs[me.cls]?.name ?? "";
    this.levelText.textContent = `Lvl ${me.lvl} ${cls}`;
    this.xpFill.style.width = `${me.lvl >= 45 ? 100 : (100 * me.xp) / Math.max(1, me.xpn)}%`;
    this.pointsText.textContent = me.pts > 0 ? `x${me.pts}` : "";
    this.stats.classList.toggle("has-points", me.pts > 0);
    this.statRows.forEach((row, i) => {
      const v = me.stats[i] ?? 0;
      row.fill.forEach((seg, j) => seg.classList.toggle("on", j < v));
      row.btn.disabled = me.pts <= 0 || v >= 7;
    });

    const key = me.upg.join(",");
    if (key !== this.lastUpg) {
      this.lastUpg = key;
      this.classes.innerHTML = "";
      me.upg.forEach((id, i) => {
        const def = this.classDefs[id];
        if (!def) return;
        const btn = document.createElement("button");
        btn.className = "class-btn";
        btn.innerHTML = `<span>${def.name}</span><small>[${CLASS_KEYS[i] ?? ""}]</small>`;
        btn.onclick = (ev) => {
          ev.preventDefault();
          this.onClass(id);
        };
        this.classes.appendChild(btn);
      });
    }
    this.classes.style.display = me.upg.length ? "flex" : "none";
  }

  updateLeaderboard(lb: LBEntry[]): void {
    const max = lb.length ? lb[0].s : 1;
    this.lb.innerHTML = "<h3>Leaderboard</h3>";
    for (const e of lb) {
      const row = document.createElement("div");
      row.className = "lb-row" + (e.me ? " me" : "");
      const fill = document.createElement("div");
      fill.className = "lb-fill";
      fill.style.width = `${Math.max(4, (100 * e.s) / Math.max(1, max))}%`;
      const txt = document.createElement("span");
      txt.textContent = `${e.n} — ${e.s.toLocaleString()}`;
      row.append(fill, txt);
      this.lb.appendChild(row);
    }
  }

  showStart(): void {
    this.start.style.display = "flex";
    this.death.style.display = "none";
    this.hud.style.display = "none";
    this.nameInput.focus();
  }
  showGame(): void {
    this.start.style.display = "none";
    this.death.style.display = "none";
    this.hud.style.display = "block";
  }
  showDeath(score: number, level: number, killer: string, kills: number): void {
    this.deathText.innerHTML = `Score: <b>${score.toLocaleString()}</b> &middot; Level <b>${level}</b> &middot; Kills <b>${kills}</b><br>Destroyed by <b>${escapeHtml(killer)}</b>`;
    this.death.style.display = "flex";
    this.hud.style.display = "none";
  }
  setStatus(text: string): void {
    this.status.textContent = text;
    this.status.style.display = text ? "block" : "none";
  }
}

const STAT_COLORS = ["#f7a96b", "#e36bf7", "#ba6bf7", "#6b8df7", "#f7e86b", "#f76b6b", "#8ff76b", "#6bf7e8"];

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
}
