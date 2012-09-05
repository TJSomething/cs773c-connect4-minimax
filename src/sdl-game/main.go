package main

import (
	"github.com/0xe2-0x9a-0x9b/Go-SDL/sdl"
	"time"
	"fmt"
	"../c4"
	"runtime"
)

const BOARD_COLOR = 0xFF4050E0
const SCREEN_WIDTH = 640
const SCREEN_HEIGHT = 480

type SDLHuman struct {
	Ready chan<- int
	Move <-chan int
}

func (ui SDLHuman) NextMove(game c4.State) int {
	// Tell UI that we're ready for a move
	ui.Ready <- 1
	// Wait for a move
	return <-ui.Move
}

func NewUpdater(gameUI chan<- c4.State) func(c4.State) {
	return func(game c4.State) {
		gameUI <- game
	}
}

var redImage, blackImage, noneImage *sdl.Surface
func drawPiece(s *sdl.Surface, col, row int, p c4.Piece) {
	// Load images
	if redImage == nil {
		redImage = sdl.Load("red.png")
	}
	if blackImage == nil {
		blackImage = sdl.Load("black.png")
	}
	if noneImage == nil {
		noneImage = sdl.Load("empty.png")
	}

	// Select image
	var image *sdl.Surface
	if p == c4.Red {
		image = redImage
	} else if p == c4.Black {
		image = blackImage
	} else {
		image = noneImage
	}

	// Draw image
	s.Blit(
		&sdl.Rect{
			int16(SCREEN_WIDTH*col/c4.MaxColumns), 
			int16(SCREEN_HEIGHT*(c4.MaxRows-row-1)/c4.MaxRows), 
			0, 
			0}, 
		image, 
		nil)
}

func main() {
	// Use all processors
	runtime.GOMAXPROCS(runtime.NumCPU())

	// SDL voodoo
	if sdl.Init(sdl.INIT_VIDEO) != 0 {
		panic(sdl.GetError())
	}

	defer sdl.Quit()

	screen := sdl.SetVideoMode(640, 480, 32, sdl.ANYFORMAT)
	if screen == nil {
		panic(sdl.GetError())
	}
	screen.SetAlpha(sdl.SRCALPHA, 255)

	sdl.WM_SetCaption("Connect Four", "")

	ticker := time.NewTicker(time.Second / 60 /*60 Hz*/ )

	// Make some pipes for communicating with the game logic
	moveReady := make(chan int)
	newState := make(chan c4.State)
	nextMove := make(chan int)
	var game c4.State
	waitingForMove := false

	// Start a game
	go c4.RunGame(
		SDLHuman{moveReady, nextMove},
		c4.AlphaBetaAI{
			c4.Black,
			8,
			func(game c4.State, p c4.Piece) float64 {
				return c4.EvalFactors{5, -3, 1, -1, 1, -1}.Eval(game, p)
			},
			func(game c4.State) bool {
				return game.GetWinner() != c4.None
			},
		},
		NewUpdater(newState),
		func(err error) {
			fmt.Println(err)
		},
		func(winner c4.Piece) {
			if winner == c4.Red {
				fmt.Println("Red wins!")
			} else if winner == c4.Black {
				fmt.Println("Black wins!")
			} else {
				fmt.Println("It's a tie.")
			}
		})

loop:
	for {
		select {
		case <-ticker.C:
			screen.FillRect(
				&sdl.Rect{0,0,SCREEN_WIDTH,SCREEN_HEIGHT},
				BOARD_COLOR)
			for col := 0; col < c4.MaxColumns; col++ {
				for row := 0; row < c4.MaxRows; row++ {
					drawPiece(screen, col, row, game.GetPiece(col, row))
				}
			}
			screen.Flip()

		case event := <-sdl.Events:
			switch e := event.(type) {
			case sdl.MouseButtonEvent:
				if waitingForMove &&
					e.Type == sdl.MOUSEBUTTONUP &&
					e.Button == sdl.BUTTON_LEFT {
					waitingForMove = false
					nextMove <- int(e.X*c4.MaxColumns/SCREEN_WIDTH)
				}
			case sdl.QuitEvent:
				break loop
			}

		case game = <-newState:
			// We did the assignment; there's nothing else to do

		case <-moveReady:
			waitingForMove = true
		}
	}
}
