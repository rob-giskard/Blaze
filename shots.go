//go:build shots

package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Test-only helpers for screenshots; not part of the release build.
func init() {
	debugHook = func(g *Game) {
		if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
			g.obstacles = nil
			xs := []float64{260, 360, 470, 560, 680}
			for i, k := range []int{obStump, obLogs, obPost, obCrate, obBranch} {
				o := &Obstacle{kind: k, x: xs[i]}
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
			}
			g.pickups = []*Pickup{{x: 850, y: groundY - 90}}
			g.flames, g.score = 27, 1234
			g.nextObs = 1e9
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
			g.flames = 0
			g.state = stDying
			g.score = 1234
			g.endRun()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF7) {
			g.flames = 99
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF6) {
			g.flames = 39
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
			// collect one flame instantly
			g.pickups = append(g.pickups, &Pickup{x: playerX, y: g.py - 10})
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF8) {
			g.speed = 0
		}
	}
}
