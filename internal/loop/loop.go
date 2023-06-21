// Package gameloop implements a very simple game loop.
// This code is based on: github.com/kutase/go-gameloop
package loop

import (
	"runtime"
	"time"
)

// GameLoop implements a simple game loop.
type GameLoop struct {
	onUpdate func(float64) int // update function called by loop
	onStop   func()
	tickRate time.Duration // tick interval
	Quit     chan bool     // channel used for exiting the loop
}

// Create new game loop
func New(tickRate time.Duration, onUpdate func(float64) int, onStop func()) *GameLoop {
	return &GameLoop{
		onUpdate: onUpdate,
		onStop:   onStop,
		tickRate: tickRate,
		Quit:     make(chan bool),
	}
}

// startLoop sets up and runs the loop until we exit.
func (g *GameLoop) startLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Set up ticker.
	t := time.NewTicker(g.tickRate)

	var now int64
	var delta float64
	start := time.Now().UnixNano()
F:
	for {
		select {
		case <-t.C:
			// Calculate delta T in fractions of seconds.
			now = time.Now().UnixNano()
			delta = float64(now-start) / 1000000000
			start = now
			code := g.onUpdate(delta)
			if code == -1 {
				// fmt.Println("stopping loop")
				t.Stop()
				// g.Stop()
				break F
			} else if code > 0 {
				// fmt.Println("resetting ticker:", code)
				t.Reset(time.Duration(code))
			}
			// case <-g.Quit:
			// t.Stop()
		}
	}

	g.onStop()
}

// Start game loop.
func (g *GameLoop) Start() {
	go g.startLoop()
}

// Stop game loop.
func (g *GameLoop) Stop() {
	g.Quit <- true
}

// Restart game loop.
func (g *GameLoop) Restart() {
	g.Stop()
	g.Start()
}
