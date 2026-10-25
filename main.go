package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

//go:embed icon.png
var iconPNG []byte

const (
	W       = 960
	H       = 400
	groundY = 318.0
	playerX = 120.0
	dt      = 1.0 / 60
	maxFlm  = 100
	gravity = 2700.0
	jumpV   = -860.0
)

const (
	stMenu = iota
	stPlay
	stPaused
	stDying
	stOver
	stWin
)

const (
	obStump = iota
	obLogs
	obPost
	obCrate
	obBranch
)

type Theme struct {
	bg, farTree, nearTree, groundLine, dirt, speck color.NRGBA
	text, sub, accent, panel, btn, btnText         color.NRGBA
	bark, barkDark, woodEnd, ring                  color.NRGBA
}

var lightTheme = Theme{
	bg: hex("#FBF3E4"), farTree: hex("#EEDFC6"), nearTree: hex("#E2CBA8"),
	groundLine: hex("#5B4636"), dirt: hex("#F3E6D0"), speck: hex("#D3BE9E"),
	text: hex("#3B2A1E"), sub: hex("#8A7360"), accent: hex("#E8590C"), panel: hex("#FFF9EF"),
	btn: hex("#F1E2CA"), btnText: hex("#3B2A1E"),
	bark: hex("#8B5A2B"), barkDark: hex("#5E3B1C"), woodEnd: hex("#D9A766"), ring: hex("#A8743E"),
}

var darkTheme = Theme{
	bg: hex("#131019"), farTree: hex("#1B1724"), nearTree: hex("#231E2F"),
	groundLine: hex("#7A6655"), dirt: hex("#19151F"), speck: hex("#2D2738"),
	text: hex("#F3E8D8"), sub: hex("#9C8E80"), accent: hex("#FF8A3D"), panel: hex("#1E1A27"),
	btn: hex("#2A2433"), btnText: hex("#F3E8D8"),
	bark: hex("#8A5A2E"), barkDark: hex("#5A3A1E"), woodEnd: hex("#C99558"), ring: hex("#9A6836"),
}

type Obstacle struct {
	kind       int
	x, y, w, h float64 // y = top
	hit        bool
}

type Pickup struct {
	x, y, phase float64
	taken       bool
}

type Particle struct {
	x, y, vx, vy, life, max, size float64
	col                           color.NRGBA
	smoke                         bool
}

type Floater struct {
	x, y, life float64
	s          string
	col        color.NRGBA
}

type Tree struct{ x, h, w float64 }

type Btn struct {
	x, y, w, h float64
	label      string
}

func (b Btn) hit(mx, my float64) bool {
	return mx >= b.x && mx <= b.x+b.w && my >= b.y && my <= b.y+b.h
}

type Settings struct {
	Dark bool `json:"dark"`
	Best int  `json:"best"`
}

type Game struct {
	state int
	dark  bool
	best  int
	t     float64

	py, vy   float64
	onGround bool
	ducking  bool
	flames   int
	invuln   float64
	speed    float64
	score    float64
	newBest  bool
	deadT    float64
	shake    float64
	shield   bool
	hist     []float64 // recent fire heights, for the followers
	mergeT   float64   // time since last 10-flame merge
	winT     float64
	fwT      float64 // next firework

	nextObs, nextPick float64
	obstacles         []*Obstacle
	pickups           []*Pickup
	particles         []*Particle
	floaters          []*Floater

	bgScroll   float64
	farTrees   []Tree
	nearTrees  []Tree
	stars      [][3]float64
	specks     [][3]float64
	menuSel    int
	overSel    int
	lastMX     int
	lastMY     int
	toggleHov  bool
	world      *ebiten.Image
	snd        *Sounds
	fBig, fMid *text.GoTextFace
	fSmall     *text.GoTextFace
	fBtn       *text.GoTextFace
	fMono      *text.GoTextFace
}

var (
	menuStart = Btn{380, 196, 200, 50, "Start"}
	menuQuit  = Btn{380, 260, 200, 50, "Quit"}
	overAgain = Btn{300, 238, 170, 46, "Relight"}
	overMenu  = Btn{490, 238, 170, 46, "Menu"}
	toggleBox = Btn{896, 14, 46, 54, ""}
	winAgain  = Btn{300, 196, 170, 46, "Play again"}
	winMenu   = Btn{490, 196, 170, 46, "Menu"}
)

const bandW = 1800.0

var debugHook func(g *Game)

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Blaze", "settings.json")
}

func (g *Game) load() {
	p := settingsPath()
	if p == "" {
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var s Settings
	if json.Unmarshal(b, &s) == nil {
		g.dark, g.best = s.Dark, s.Best
	}
}

func (g *Game) save() {
	p := settingsPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.Marshal(Settings{g.dark, g.best})
	_ = os.WriteFile(p, b, 0o644)
}

func face(ttf []byte, size float64) *text.GoTextFace {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
	if err != nil {
		panic(err)
	}
	return &text.GoTextFace{Source: src, Size: size}
}

func newGame() *Game {
	g := &Game{state: stMenu, world: ebiten.NewImage(W, H)}
	g.fBig = face(gobold.TTF, 96)
	g.fMid = face(gobold.TTF, 32)
	g.fBtn = face(gobold.TTF, 22)
	g.fSmall = face(goregular.TTF, 15)
	g.fMono = face(gomonobold.TTF, 20)
	for x := 0.0; x < bandW; x += 40 + rand.Float64()*90 {
		g.farTrees = append(g.farTrees, Tree{x, 70 + rand.Float64()*70, 34 + rand.Float64()*20})
	}
	for x := 0.0; x < bandW; x += 90 + rand.Float64()*170 {
		g.nearTrees = append(g.nearTrees, Tree{x, 90 + rand.Float64()*80, 44 + rand.Float64()*24})
	}
	for i := 0; i < 70; i++ {
		g.stars = append(g.stars, [3]float64{rand.Float64() * bandW, rand.Float64() * 220, rand.Float64()})
	}
	for i := 0; i < 60; i++ {
		g.specks = append(g.specks, [3]float64{rand.Float64() * bandW, groundY + 8 + rand.Float64()*(H-groundY-14), 1 + rand.Float64()*2})
	}
	g.py, g.onGround = groundY, true
	g.load()
	g.snd = newSounds()
	return g
}

func (g *Game) theme() *Theme {
	if g.dark {
		return &darkTheme
	}
	return &lightTheme
}

func (g *Game) toggleDark() {
	g.dark = !g.dark
	g.snd.play(g.snd.click)
	g.save()
}

func (g *Game) startGame() {
	g.state = stPlay
	g.py, g.vy, g.onGround, g.ducking = groundY, 0, true, false
	g.flames, g.invuln = 3, 0
	g.speed, g.score, g.newBest = 340, 0, false
	g.deadT, g.shake, g.shield, g.mergeT, g.hist, g.winT = 0, 0, false, 99, nil, 0
	g.obstacles, g.pickups, g.floaters, g.particles = nil, nil, nil, nil
	g.nextObs, g.nextPick = 700, 1100
	g.snd.play(g.snd.click)
}

func jumpPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) ||
		inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

func jumpReleased() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeySpace) || inpututil.IsKeyJustReleased(ebiten.KeyArrowUp) ||
		inpututil.IsKeyJustReleased(ebiten.KeyW) || inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
}

func confirmPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
}

func (g *Game) Update() error {
	g.t += dt
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) || (inpututil.IsKeyJustPressed(ebiten.KeyEnter) && ebiten.IsKeyPressed(ebiten.KeyAlt)) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
		return nil
	}
	if debugHook != nil {
		debugHook(g)
	}
	mx, my := ebiten.CursorPosition()
	moved := mx != g.lastMX || my != g.lastMY
	g.lastMX, g.lastMY = mx, my
	fx, fy := float64(mx), float64(my)
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	switch g.state {
	case stMenu:
		g.bgScroll += 25 * dt
		g.pushHist(groundY)
		g.toggleHov = toggleBox.hit(fx, fy)
		if moved {
			if menuStart.hit(fx, fy) {
				g.menuSel = 0
			} else if menuQuit.hit(fx, fy) {
				g.menuSel = 1
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			g.menuSel = 0
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.menuSel = 1
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyD) {
			g.toggleDark()
		}
		if click {
			switch {
			case g.toggleHov:
				g.toggleDark()
			case menuStart.hit(fx, fy):
				g.startGame()
			case menuQuit.hit(fx, fy):
				return ebiten.Termination
			}
		} else if confirmPressed() {
			if g.menuSel == 0 {
				g.startGame()
			} else {
				return ebiten.Termination
			}
		} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			return ebiten.Termination
		}
		g.updateParticles(0)
	case stPlay:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyP) || !ebiten.IsFocused() {
			g.state = stPaused
			return nil
		}
		g.updatePlay()
	case stPaused:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyM) {
			g.endRun()
			g.state = stMenu
		} else if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyP) ||
			inpututil.IsKeyJustPressed(ebiten.KeyEnter) || click {
			g.state = stPlay
		} else if inpututil.IsKeyJustPressed(ebiten.KeyD) {
			g.toggleDark()
		}
	case stDying:
		g.deadT += dt
		g.shake = math.Max(0, g.shake-dt)
		if rand.Float64() < 0.5 && g.deadT < 1.4 {
			g.addSmoke()
		}
		g.updateParticles(0)
		g.updateFloaters()
		if g.deadT > 1.3 {
			g.state = stOver
			g.overSel = 0
		}
	case stOver, stWin:
		g.updateParticles(0)
		g.updateFloaters()
		overAgain, overMenu := overAgain, overMenu
		if g.state == stWin {
			overAgain, overMenu = winAgain, winMenu
			g.winT += dt
			g.mergeT += dt
			g.updateFireworks()
		} else if rand.Float64() < 0.08 {
			g.addSmoke()
		}
		if moved {
			if overAgain.hit(fx, fy) {
				g.overSel = 0
			} else if overMenu.hit(fx, fy) {
				g.overSel = 1
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
			g.overSel = 0
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
			g.overSel = 1
		}
		if click {
			if overAgain.hit(fx, fy) {
				g.startGame()
			} else if overMenu.hit(fx, fy) {
				g.state = stMenu
				g.snd.play(g.snd.click)
			}
		} else if confirmPressed() {
			if g.overSel == 0 {
				g.startGame()
			} else {
				g.state = stMenu
				g.snd.play(g.snd.click)
			}
		} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.state = stMenu
		}
	}
	return nil
}

func (g *Game) endRun() {
	if int(g.score) > g.best {
		g.best = int(g.score)
		g.newBest = true
		g.save()
	}
}

func (g *Game) updatePlay() {
	g.speed = math.Min(920, g.speed+7*dt)
	dx := g.speed * dt
	g.score += dx * 0.025
	g.bgScroll += dx * 0.3

	down := ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS)
	if jumpPressed() && g.onGround {
		g.vy, g.onGround = jumpV, false
		g.snd.play(g.snd.jump)
	}
	if jumpReleased() && g.vy < -380 {
		g.vy = -380
	}
	grav := gravity
	if down && !g.onGround {
		grav *= 2.6
	}
	g.vy += grav * dt
	g.py += g.vy * dt
	if g.py >= groundY {
		g.py, g.vy, g.onGround = groundY, 0, true
	}
	g.ducking = down && g.onGround
	g.pushHist(g.py)
	g.mergeT += dt

	// obstacles
	g.nextObs -= dx
	if g.nextObs <= 0 {
		g.spawnObstacle()
	}
	kept := g.obstacles[:0]
	for _, o := range g.obstacles {
		o.x -= dx
		if o.x+o.w > -60 {
			kept = append(kept, o)
		}
	}
	g.obstacles = kept

	// pickups
	g.nextPick -= dx
	if g.nextPick <= 0 {
		clear := true
		for _, o := range g.obstacles {
			if o.x > W-120 {
				clear = false
			}
		}
		if clear {
			g.spawnPickups()
		} else {
			g.nextPick = 80
		}
	}
	keptP := g.pickups[:0]
	for _, p := range g.pickups {
		p.x -= dx
		if p.x > -40 && !p.taken {
			keptP = append(keptP, p)
		}
	}
	g.pickups = keptP

	// collisions
	px0, px1, py0, py1 := g.hitbox()
	for _, p := range g.pickups {
		by := p.y + math.Sin(g.t*4+p.phase)*5
		if px1 > p.x-12 && px0 < p.x+12 && py1 > by-26 && py0 < by+2 {
			p.taken = true
			g.flames++
			g.score += 5
			g.floaters = append(g.floaters, &Floater{p.x, by - 30, 1, "+1", flameMid})
			g.snd.play(g.snd.collect)
			if g.flames%10 == 0 {
				g.mergeT = 0
				if g.flames >= maxFlm {
					g.win()
					return
				}
				g.shield = true
				g.snd.play(g.snd.shield)
				g.floaters = append(g.floaters, &Floater{playerX, g.py - 95, 1.4, "Shield!", bluePal[1]})
			}
			for i := 0; i < 14; i++ {
				a := rand.Float64() * 2 * math.Pi
				s := 60 + rand.Float64()*140
				g.particles = append(g.particles, &Particle{x: p.x, y: by - 10, vx: math.Cos(a) * s, vy: math.Sin(a) * s,
					life: 0.5, max: 0.5, size: 2 + rand.Float64()*2, col: flameInner})
			}
		}
	}
	if g.invuln <= 0 {
		for _, o := range g.obstacles {
			if o.hit {
				continue
			}
			ox0, ox1, oy0, oy1 := o.x+3, o.x+o.w-3, o.y+3, o.y+o.h
			if px1 > ox0 && px0 < ox1 && py1 > oy0 && py0 < oy1 {
				o.hit = true
				if g.shield {
					g.shield = false
					g.invuln = 0.9
					g.snd.play(g.snd.block)
					g.floaters = append(g.floaters, &Floater{playerX, g.py - 80, 1, "Blocked!", bluePal[1]})
					for i := 0; i < 26; i++ {
						a := rand.Float64() * 2 * math.Pi
						s := 100 + rand.Float64()*200
						g.particles = append(g.particles, &Particle{x: playerX, y: g.py - 28, vx: math.Cos(a) * s, vy: math.Sin(a) * s,
							life: 0.6, max: 0.6, size: 2 + rand.Float64()*2.5, col: bluePal[1]})
					}
					break
				}
				if g.flames <= 3 {
					// the last trailing flame puffs out
					fx, fy := g.followerPos(g.flames - 1)
					for k := 0; k < 6; k++ {
						g.particles = append(g.particles, &Particle{x: fx + (rand.Float64()-0.5)*10, y: fy - 10,
							vx: -g.speed * 0.4, vy: -30 - rand.Float64()*30, life: 0.9, max: 0.9, size: 4 + rand.Float64()*3, col: hex("#9A9590"), smoke: true})
					}
				}
				g.flames--
				g.invuln = 1.3
				g.shake = 0.25
				g.snd.play(g.snd.hit)
				g.floaters = append(g.floaters, &Floater{playerX, g.py - 80, 1, "-1", hex("#C0392B")})
				for i := 0; i < 20; i++ {
					a := -math.Pi/2 + (rand.Float64()-0.5)*2.4
					s := 120 + rand.Float64()*220
					g.particles = append(g.particles, &Particle{x: playerX + 10, y: g.py - 25, vx: math.Cos(a) * s, vy: math.Sin(a) * s,
						life: 0.6, max: 0.6, size: 2 + rand.Float64()*2.5, col: flameMid})
				}
				if g.flames <= 0 {
					g.flames = 0
					g.state = stDying
					g.deadT = 0
					g.snd.play(g.snd.out)
					g.endRun()
				}
				break
			}
		}
	}
	g.invuln = math.Max(0, g.invuln-dt)
	g.shake = math.Max(0, g.shake-dt)

	// embers from the fire
	if rand.Float64() < 0.35 {
		g.particles = append(g.particles, &Particle{x: playerX + (rand.Float64()-0.5)*20, y: g.py - 40,
			vx: -g.speed*0.35 + (rand.Float64()-0.5)*40, vy: -60 - rand.Float64()*80,
			life: 0.9, max: 0.9, size: 1.2 + rand.Float64()*1.8, col: flameMid})
	}
	g.updateParticles(0)
	g.updateFloaters()
}

func (g *Game) spawnPickups() {
	ph := rand.Float64() * 6
	length := 0.0
	switch r := rand.Float64(); {
	case r < 0.5: // single flame
		g.pickups = append(g.pickups, &Pickup{x: W + 30, y: groundY - 34 - rand.Float64()*100, phase: ph})
	case r < 0.75: // a row of three
		y := groundY - 34 - rand.Float64()*50
		for k := 0; k < 3; k++ {
			g.pickups = append(g.pickups, &Pickup{x: W + 30 + float64(k)*38, y: y, phase: ph})
		}
		length = 76
	default: // an arc you catch by jumping through it
		for k := 0; k < 5; k++ {
			t := float64(k) / 4
			g.pickups = append(g.pickups, &Pickup{x: W + 30 + float64(k)*42, y: groundY - 40 - 380*t*(1-t), phase: ph})
		}
		length = 168
	}
	g.nextObs = math.Max(g.nextObs, length+220)
	g.nextPick = 650 + rand.Float64()*850 + length
}

func (g *Game) win() {
	g.state = stWin
	g.winT, g.fwT, g.overSel = 0, 0, 0
	g.shield = false
	g.snd.play(g.snd.win)
	g.endRun()
	for i := 0; i < 50; i++ {
		a := rand.Float64() * 2 * math.Pi
		sp := 100 + rand.Float64()*300
		g.particles = append(g.particles, &Particle{x: playerX, y: g.py - 40, vx: math.Cos(a) * sp, vy: math.Sin(a) * sp,
			life: 1, max: 1, size: 2 + rand.Float64()*3, col: flameMid})
	}
}

func (g *Game) updateFireworks() {
	g.fwT -= dt
	if g.fwT > 0 {
		return
	}
	g.fwT = 0.3 + rand.Float64()*0.35
	cols := []color.NRGBA{flameInner, flameOuter, bluePal[1], hex("#FF5FA2"), hex("#8BE36A")}
	c := cols[rand.IntN(len(cols))]
	x, y := 60+rand.Float64()*840, 40+rand.Float64()*150
	for i := 0; i < 34; i++ {
		a := float64(i) / 34 * 2 * math.Pi
		sp := 90 + rand.Float64()*90
		g.particles = append(g.particles, &Particle{x: x, y: y, vx: math.Cos(a) * sp, vy: math.Sin(a) * sp,
			life: 1.1, max: 1.1, size: 2 + rand.Float64()*1.5, col: c})
	}
	g.snd.play(g.snd.pop)
}

func (g *Game) hitbox() (x0, x1, y0, y1 float64) {
	top := 56.0
	if g.ducking {
		top = 34
	}
	return playerX - 18, playerX + 18, g.py - top + 4, g.py - 2
}

func (g *Game) spawnObstacle() {
	kinds := []int{obStump, obLogs, obPost, obCrate}
	if g.score > 120 {
		kinds = append(kinds, obBranch)
	}
	k := kinds[rand.IntN(len(kinds))]
	o := &Obstacle{kind: k, x: W + 20}
	switch k {
	case obStump:
		o.w, o.h = 34, 40
	case obLogs:
		o.w, o.h = 64, 46
	case obPost:
		o.w, o.h = 22, 64
	case obCrate:
		o.w, o.h = 44, 44
	case obBranch:
		o.w, o.h = 84, 22
	}
	o.y = groundY - o.h
	if k == obBranch {
		o.y = groundY - 70
	}
	g.obstacles = append(g.obstacles, o)
	gap := g.speed*(0.72+rand.Float64()*0.9) + 140
	// occasional close pair of small stumps
	if k == obStump && rand.Float64() < 0.25 && g.score > 60 {
		g.obstacles = append(g.obstacles, &Obstacle{kind: obStump, x: W + 20 + 40, y: groundY - 32, w: 30, h: 32})
		gap += 50
	}
	g.nextObs = gap
}

func (g *Game) addSmoke() {
	g.particles = append(g.particles, &Particle{x: playerX + (rand.Float64()-0.5)*16, y: g.py - 16,
		vx: (rand.Float64() - 0.5) * 20, vy: -40 - rand.Float64()*30, life: 1.6, max: 1.6,
		size: 6 + rand.Float64()*6, col: hex("#9A9590"), smoke: true})
}

func (g *Game) updateParticles(_ float64) {
	if g.state == stMenu && rand.Float64() < 0.25 {
		g.particles = append(g.particles, &Particle{x: playerX + (rand.Float64()-0.5)*20, y: groundY - 30,
			vx: (rand.Float64() - 0.3) * 30, vy: -50 - rand.Float64()*70,
			life: 1.1, max: 1.1, size: 1.2 + rand.Float64()*1.8, col: flameMid})
	}
	kept := g.particles[:0]
	for _, p := range g.particles {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		if p.smoke {
			p.size += 10 * dt
		} else {
			p.vy -= 20 * dt
		}
		kept = append(kept, p)
	}
	g.particles = kept
}

func (g *Game) updateFloaters() {
	kept := g.floaters[:0]
	for _, f := range g.floaters {
		f.life -= dt * 1.2
		f.y -= 40 * dt
		if f.life > 0 {
			kept = append(kept, f)
		}
	}
	g.floaters = kept
}

// ---------------------------------------------------------------- drawing

func (g *Game) Draw(screen *ebiten.Image) {
	th := g.theme()
	w := g.world
	w.Fill(th.bg)
	g.drawBackground(w, th)
	g.drawGround(w, th)

	if g.state != stMenu {
		for _, o := range g.obstacles {
			g.drawObstacle(w, th, o)
		}
		for _, p := range g.pickups {
			g.drawPickup(w, th, p)
		}
	}
	g.drawPlayer(w, th)
	g.drawParticles(w)
	for _, f := range g.floaters {
		drawText(w, f.s, g.fBtn, f.x, f.y, text.AlignCenter, withAlpha(f.col, f.life))
	}

	op := &ebiten.DrawImageOptions{}
	if g.shake > 0 {
		op.GeoM.Translate((rand.Float64()-0.5)*14*g.shake/0.25, (rand.Float64()-0.5)*10*g.shake/0.25)
	}
	screen.Fill(th.bg)
	screen.DrawImage(w, op)

	switch g.state {
	case stMenu:
		g.drawMenu(screen, th)
	case stPlay, stPaused, stDying, stOver, stWin:
		g.drawHUD(screen, th)
	}
	if g.state == stWin {
		g.drawWin(screen, th)
	}
	if g.state == stPaused {
		g.drawPause(screen, th)
	}
	if g.state == stOver {
		g.drawOver(screen, th)
	}
}

func wrap(x float64) float64 {
	x = math.Mod(x, bandW)
	if x < 0 {
		x += bandW
	}
	return x
}

func drawPine(dst *ebiten.Image, x, h, w float64, c color.NRGBA) {
	base := groundY
	rect(dst, x-w*0.07, base-h*0.18, w*0.14, h*0.18, c)
	for i := 0; i < 3; i++ {
		fi := float64(i)
		ty := base - h*0.15 - fi*h*0.24
		tw := w * (1 - fi*0.22)
		var p = triPath(x-tw/2, ty, x+tw/2, ty, x, ty-h*0.42)
		fillPath(dst, p, c)
	}
}

func (g *Game) drawBackground(dst *ebiten.Image, th *Theme) {
	if g.dark {
		for _, s := range g.stars {
			x := wrap(s[0]-g.bgScroll*0.05) * W / bandW * 2
			if x > W {
				continue
			}
			tw := 0.5 + 0.5*math.Sin(g.t*2+s[2]*20)
			circle(dst, x, s[1], 0.8+s[2]*1.0, withAlpha(hex("#FFF4E0"), 0.35+0.55*tw*s[2]))
		}
		// moon
		circle(dst, 740, 78, 22, hex("#EDE3D1"))
		circle(dst, 751, 71, 20, th.bg)
	} else {
		glow(dst, 740, 78, 60, hex("#FFD9A0"), 0.09)
		circle(dst, 740, 78, 22, hex("#FFE2B0"))
	}
	for _, t := range g.farTrees {
		x := wrap(t.x-g.bgScroll*0.35) - 100
		if x > -80 && x < W+80 {
			drawPine(dst, x, t.h, t.w, th.farTree)
		}
	}
	for _, t := range g.nearTrees {
		x := wrap(t.x-g.bgScroll*0.7) - 100
		if x > -80 && x < W+80 {
			drawPine(dst, x, t.h, t.w, th.nearTree)
		}
	}
	// warm light from the campfire
	str := 0.035
	if g.dark {
		str = 0.07
	}
	fade := 1.0
	if g.state == stDying {
		fade = math.Max(0, 1-g.deadT/0.7)
	} else if g.state == stOver {
		fade = 0
	}
	if fade > 0 {
		glow(dst, playerX, g.py-24, 150+6*math.Sin(g.t*9), hex("#FF8A3D"), str*fade)
	}
}

func triPath(x0, y0, x1, y1, x2, y2 float64) *vector.Path {
	var p vector.Path
	p.MoveTo(float32(x0), float32(y0))
	p.LineTo(float32(x1), float32(y1))
	p.LineTo(float32(x2), float32(y2))
	p.Close()
	return &p
}

func (g *Game) drawGround(dst *ebiten.Image, th *Theme) {
	rect(dst, 0, groundY, W, H-groundY, th.dirt)
	rect(dst, 0, groundY, W, 2.5, th.groundLine)
	scroll := g.bgScroll / 0.3
	for _, s := range g.specks {
		x := wrap(s[0]-scroll) * 1.0
		if x < W+10 {
			rect(dst, x, s[1], s[2]*3, 1.6, th.speck)
		}
	}
}

func (g *Game) followerPos(i int) (float64, float64) {
	delay := 8 * (i + 1)
	y := groundY
	if len(g.hist) > delay {
		y = g.hist[len(g.hist)-1-delay]
	}
	x := playerX - 34 - 22*float64(i)
	if y >= groundY-0.5 && g.state != stOver {
		// little running hop
		y -= math.Abs(math.Sin(g.t*9+float64(i)*1.3)) * 4
	}
	return x, y - 1
}

func (g *Game) pushHist(y float64) {
	g.hist = append(g.hist, y)
	if len(g.hist) > 120 {
		g.hist = append(g.hist[:0], g.hist[len(g.hist)-60:]...)
	}
}

func (g *Game) drawPlayer(dst *ebiten.Image, th *Theme) {
	scale := 1.0
	switch g.state {
	case stDying:
		scale = math.Max(0, 1-g.deadT/0.6)
	case stOver:
		scale = 0
	}
	// up to three spare flames trail behind the fire
	n := min(g.flames, 3)
	if g.state == stMenu {
		n = 3
	}
	for i := n - 1; i >= 0; i-- {
		x, y := g.followerPos(i)
		k := 1 - 0.07*float64(i)
		drawFlame(dst, x, y, 17*scale*k, 25*scale*k, g.t, 2+float64(i)*1.9, 1)
	}
	if g.invuln > 0 && g.state == stPlay && int(g.invuln*12)%2 == 0 {
		return
	}
	base := g.py - 1
	fw, fh := 40.0, 54.0
	if g.ducking {
		fw, fh = 52, 30
	}
	// the fire grows a little with every big flame
	bonus := 1 + 0.035*float64(g.flames/10)
	switch g.state {
	case stMenu:
		bonus = 1
	case stWin:
		bonus = 1.45
	}
	if g.shield && scale > 0 {
		pulse := 0.5 + 0.15*math.Sin(g.t*6)
		glow(dst, playerX, base-fh*0.45, 70, bluePal[1], 0.05)
		drawFlamePal(dst, playerX, base+2, fw*bonus*1.5, fh*bonus*1.3, g.t*1.3, 5, pulse, bluePal)
	}
	if scale > 0 {
		drawFlame(dst, playerX, base, fw*scale*bonus, fh*scale*bonus, g.t, 0, 1)
	}
	if scale < 1 {
		// glowing embers left on the ground
		a := 0.5 + 0.4*math.Sin(g.t*5)
		circle(dst, playerX-6, base-3, 3, withAlpha(flameOuter, a))
		circle(dst, playerX+7, base-2, 2.5, withAlpha(flameMid, a*0.8))
	}
}

func (g *Game) drawObstacle(dst *ebiten.Image, th *Theme, o *Obstacle) {
	x, y, w, h := o.x, o.y, o.w, o.h
	switch o.kind {
	case obStump:
		roundRect(dst, x, y+4, w, h-4, 4, th.bark)
		for i := 1; i < 4; i++ {
			lx := x + w*float64(i)/4
			line(dst, lx, y+10, lx+(float64(i)-2)*1.5, y+h-2, 1.5, th.barkDark)
		}
		// roots
		fillPath(dst, triPath(x-6, y+h, x+6, y+h-12, x+8, y+h), th.bark)
		fillPath(dst, triPath(x+w+6, y+h, x+w-6, y+h-12, x+w-8, y+h), th.bark)
		roundRect(dst, x-1, y, w+2, 9, 4.5, th.woodEnd)
		var p vector.Path
		p.Arc(float32(x+w/2), float32(y+4.5), 6, 0, 2*math.Pi, vector.Clockwise)
		strokePath(dst, &p, 1.4, th.ring)
	case obLogs:
		r := 13.0
		cx := []float64{x + r, x + w/2, x + w - r, x + w/2 - r + 1, x + w/2 + r - 1}
		cy := []float64{y + h - r, y + h - r, y + h - r, y + h - 3*r + 4, y + h - 3*r + 4}
		for i := range cx {
			circle(dst, cx[i], cy[i], r+1.5, th.barkDark)
			circle(dst, cx[i], cy[i], r-1, th.woodEnd)
			var p vector.Path
			p.Arc(float32(cx[i]), float32(cy[i]), float32(r*0.55), 0, 2*math.Pi, vector.Clockwise)
			strokePath(dst, &p, 1.4, th.ring)
			circle(dst, cx[i], cy[i], 1.8, th.ring)
		}
	case obPost:
		rect(dst, x, y+10, w, h-10, th.bark)
		fillPath(dst, triPath(x, y+10, x+w, y+10, x+w/2, y), th.bark)
		line(dst, x+w*0.35, y+16, x+w*0.35, y+h-4, 1.3, th.barkDark)
		line(dst, x+w*0.7, y+24, x+w*0.7, y+h-10, 1.3, th.barkDark)
		circle(dst, x+w/2, y+22, 1.8, th.barkDark)
		circle(dst, x+w/2, y+h-14, 1.8, th.barkDark)
	case obCrate:
		rect(dst, x, y, w, h, th.barkDark)
		rect(dst, x+3, y+3, w-6, h-6, th.bark)
		line(dst, x+5, y+5, x+w-5, y+h-5, 4, th.barkDark)
		line(dst, x+w-5, y+5, x+5, y+h-5, 4, th.barkDark)
		line(dst, x+3, y+h/2, x+w-3, y+h/2, 2, withAlpha(th.barkDark, 0.6))
		for _, c := range [][2]float64{{x + 5, y + 5}, {x + w - 5, y + 5}, {x + 5, y + h - 5}, {x + w - 5, y + h - 5}} {
			circle(dst, c[0], c[1], 1.7, th.woodEnd)
		}
	case obBranch:
		cy := y + h/2
		bob := math.Sin(g.t*3+x*0.01) * 2
		cy += bob
		drawLog(dst, x+w/2, cy, w-h, h, 0, th.bark, th.woodEnd, th.ring)
		line(dst, x+w*0.3, cy-h/2+2, x+w*0.18, cy-h/2-12, 3, th.bark)
		line(dst, x+w*0.65, cy+h/2-2, x+w*0.75, cy+h/2+9, 3, th.bark)
		leaf := hex("#6E8B3D")
		if g.dark {
			leaf = hex("#4F6A2E")
		}
		circle(dst, x+w*0.18, cy-h/2-14, 4.5, leaf)
		circle(dst, x+w*0.13, cy-h/2-10, 3.5, leaf)
	}
}

func (g *Game) drawPickup(dst *ebiten.Image, th *Theme, p *Pickup) {
	by := p.y + math.Sin(g.t*4+p.phase)*5
	glow(dst, p.x, by-13, 26, hex("#FFB347"), 0.05)
	drawFlame(dst, p.x, by, 21, 30, g.t, p.phase, 1)
}

func (g *Game) drawParticles(dst *ebiten.Image) {
	for _, p := range g.particles {
		a := p.life / p.max
		if p.smoke {
			circle(dst, p.x, p.y, p.size, withAlpha(p.col, a*0.35))
		} else if p.col == flameMid || p.col == flameInner {
			c := lerpCol(flameInner, flameOuter, 1-a)
			circle(dst, p.x, p.y, p.size, withAlpha(c, a))
		} else {
			circle(dst, p.x, p.y, p.size, withAlpha(p.col, a))
		}
	}
}

func (g *Game) drawHUD(dst *ebiten.Image, th *Theme) {
	cy := groundY + 58
	if g.state != stWin {
		big, small := g.flames/10, g.flames%10
		const bs, ss, gap = 32.0, 22.0, 14.0
		width := float64(big)*bs + float64(small)*ss
		if big > 0 && small > 0 {
			width += gap
		}
		x0 := W/2 - width/2
		if g.flames == 0 {
			drawText(dst, "no flames left", g.fSmall, W/2, cy-10, text.AlignCenter, th.sub)
		}
		for i := 0; i < big; i++ {
			cx := x0 + bs/2 + float64(i)*bs
			sc := 1.0
			if i == big-1 && g.mergeT < 0.6 {
				sc = 1 + 0.6*(1-g.mergeT/0.6)
				glow(dst, cx, cy-14, 40, bluePal[1], 0.08*(1-g.mergeT/0.6))
			}
			drawFlame(dst, cx, cy+2, 24*sc, 34*sc, g.t, float64(i)*2.3, 1)
		}
		sx := x0 + float64(big)*bs
		if big > 0 {
			sx += gap
		}
		for i := 0; i < small; i++ {
			drawFlame(dst, sx+ss/2+float64(i)*ss, cy, 15, 22, g.t, float64(i)*1.7, 1)
		}
		// ten small flames flying together into the new big one
		if big > 0 && g.mergeT < 0.45 {
			tx := x0 + bs/2 + float64(big-1)*bs
			p := g.mergeT / 0.45
			e := p * p * (3 - 2*p)
			for k := 0; k < 10; k++ {
				fx := tx + 50 + float64(k)*ss
				x := fx + (tx-fx)*e
				y := cy - math.Sin(p*math.Pi)*18
				drawFlame(dst, x, y, 15, 22, g.t, float64(k), 1-p*0.7)
			}
		}
		if g.flames > 0 {
			drawText(dst, fmt.Sprintf("%d/100", g.flames), g.fSmall, x0+width+14, cy-10, text.AlignStart, th.sub)
		}
		if g.shield {
			glow(dst, x0-30, cy-11, 22, bluePal[1], 0.06)
			drawFlamePal(dst, x0-30, cy, 16, 24, g.t, 7, 1, bluePal)
		}
	}
	hi := fmt.Sprintf("HI %05d", g.best)
	drawText(dst, hi, g.fMono, W-150, 30, text.AlignEnd, th.sub)
	drawText(dst, fmt.Sprintf("%05d", int(g.score)), g.fMono, W-22, 30, text.AlignEnd, th.text)
}

func (g *Game) drawButton(dst *ebiten.Image, th *Theme, b Btn, selected bool) {
	if selected {
		roundRect(dst, b.x, b.y+3, b.w, b.h, 14, withAlpha(th.barkDark, 0.35))
		roundRect(dst, b.x, b.y, b.w, b.h, 14, th.accent)
		drawText(dst, b.label, g.fBtn, b.x+b.w/2, b.y+b.h/2, text.AlignCenter, hex("#FFFFFF"))
	} else {
		roundRect(dst, b.x, b.y, b.w, b.h, 14, th.btn)
		drawText(dst, b.label, g.fBtn, b.x+b.w/2, b.y+b.h/2, text.AlignCenter, th.btnText)
	}
}

func (g *Game) drawMenu(dst *ebiten.Image, th *Theme) {
	// title with soft glow
	tc := th.text
	drawText(dst, "Blaze", g.fBig, W/2+3, 112+4, text.AlignCenter, withAlpha(th.accent, 0.25))
	drawText(dst, "Blaze", g.fBig, W/2, 112, text.AlignCenter, tc)
	g.drawButton(dst, th, menuStart, g.menuSel == 0)
	g.drawButton(dst, th, menuQuit, g.menuSel == 1)
	drawText(dst, "Space / ↑ jump   ·   ↓ duck   ·   catch 100 flames   ·   every 10 gives a blue shield", g.fSmall, W/2, groundY+48, text.AlignCenter, th.sub)

	// flame-shaped dark mode toggle
	b := toggleBox
	cx, base := b.x+b.w/2, b.y+b.h-6
	s := 1.0
	if g.toggleHov {
		s = 1.12
	}
	fw, fh := 30*s, 40*s
	if g.dark {
		glow(dst, cx, base-fh*0.4, 38, hex("#FF8A3D"), 0.06)
		drawFlame(dst, cx, base, fw, fh, g.t, 3, 1)
	} else {
		outer := flameShape(cx, base, fw, fh, fw*0.22)
		fillPath(dst, outer, withAlpha(th.accent, 0.10))
		strokePath(dst, outer, 2.4, th.text)
		strokePath(dst, flameShape(cx, base-2, fw*0.42, fh*0.5, -fw*0.08), 2.0, th.text)
	}
	if g.toggleHov {
		lbl := "Dark mode: off"
		if g.dark {
			lbl = "Dark mode: on"
		}
		drawText(dst, lbl, g.fSmall, b.x-8, b.y+b.h/2, text.AlignEnd, th.sub)
	}
}

func (g *Game) panel(dst *ebiten.Image, th *Theme, x, y, w, h float64) {
	rect(dst, 0, 0, W, H, withAlpha(th.bg, 0.45))
	roundRect(dst, x, y+5, w, h, 20, withAlpha(hex("#000000"), 0.12))
	roundRect(dst, x, y, w, h, 20, th.panel)
}

func (g *Game) drawPause(dst *ebiten.Image, th *Theme) {
	g.panel(dst, th, 330, 110, 300, 150)
	drawText(dst, "Paused", g.fMid, W/2, 158, text.AlignCenter, th.text)
	drawText(dst, "Space to resume  ·  Esc for menu", g.fSmall, W/2, 214, text.AlignCenter, th.sub)
}

func (g *Game) drawOver(dst *ebiten.Image, th *Theme) {
	g.panel(dst, th, 270, 80, 420, 228)
	drawText(dst, "The fire went out", g.fMid, W/2, 126, text.AlignCenter, th.text)
	line := fmt.Sprintf("Score %d   ·   Best %d", int(g.score), g.best)
	drawText(dst, line, g.fBtn, W/2, 176, text.AlignCenter, th.sub)
	if g.newBest {
		drawText(dst, "New best!", g.fSmall, W/2, 206, text.AlignCenter, th.accent)
	}
	g.drawButton(dst, th, overAgain, g.overSel == 0)
	g.drawButton(dst, th, overMenu, g.overSel == 1)
}

func (g *Game) drawWin(dst *ebiten.Image, th *Theme) {
	rect(dst, 0, 0, W, H, withAlpha(th.bg, 0.2))
	roundRect(dst, 270, 41, 420, 226, 20, withAlpha(hex("#000000"), 0.12))
	roundRect(dst, 270, 36, 420, 226, 20, th.panel)
	// the huge flame trophy
	pop := 1 + 0.3*math.Max(0, 1-g.winT/0.5)
	glow(dst, W/2, 338, 90, hex("#FF8A3D"), 0.08)
	drawFlame(dst, W/2, 388, 60*pop, 86*pop, g.t, 4, 1)
	drawText(dst, "100", g.fBtn, W/2+50, 362, text.AlignStart, th.accent)
	drawText(dst, "You built a Blaze!", g.fMid, W/2, 80, text.AlignCenter, th.text)
	drawText(dst, fmt.Sprintf("Score %d   ·   Best %d", int(g.score), g.best), g.fBtn, W/2, 128, text.AlignCenter, th.sub)
	if g.newBest {
		drawText(dst, "New best!", g.fSmall, W/2, 160, text.AlignCenter, th.accent)
	}
	g.drawButton(dst, th, winAgain, g.overSel == 0)
	g.drawButton(dst, th, winMenu, g.overSel == 1)
}

func (g *Game) Layout(_, _ int) (int, int) { return W, H }

func main() {
	ebiten.SetWindowTitle("Blaze")
	ebiten.SetWindowSize(W*5/4, H*5/4)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	if img, _, err := image.Decode(bytes.NewReader(iconPNG)); err == nil {
		ebiten.SetWindowIcon([]image.Image{img})
	}
	if err := ebiten.RunGame(newGame()); err != nil && err != ebiten.Termination {
		panic(err)
	}
}
