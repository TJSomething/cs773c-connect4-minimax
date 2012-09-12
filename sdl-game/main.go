package main

import (
	"../c4"
	"fmt"
	"github.com/0xe2-0x9a-0x9b/Go-SDL/sdl"
	"github.com/0xe2-0x9a-0x9b/Go-SDL/ttf"
	"runtime"
	"time"
)

const BOARD_COLOR = 0xFF4050E0
const SCREEN_WIDTH = 640
const SCREEN_HEIGHT = 480

type SDLHuman struct {
	Ready chan<- int
	Move  <-chan int
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
			int16(SCREEN_WIDTH * col / c4.MaxColumns),
			int16(SCREEN_HEIGHT * (c4.MaxRows - row - 1) / c4.MaxRows),
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

	if ttf.Init() != 0 {
		panic(sdl.GetError())
	}
	defer ttf.Quit()

	screen := sdl.SetVideoMode(640, 480, 32, sdl.ANYFORMAT)
	if screen == nil {
		panic(sdl.GetError())
	}
	screen.SetAlpha(sdl.SRCALPHA, 255)

	sdl.WM_SetCaption("Connect Four", "")

	ticker := time.NewTicker(time.Second / 60 /*60 Hz*/)

	// Make some pipes for communicating with the game logic
	moveReady := make(chan int)
	newState := make(chan c4.State)
	nextMove := make(chan int)
	gameResults := make(chan c4.Piece)
	var winner c4.Piece
	var game c4.State
	waitingForMove := false
	gameOver := false

	// Get ready to write text
	font := ttf.OpenFont("DroidSans.ttf", 36)
	var line1, line2 *sdl.Surface
	showMessage := false

	// Start a game
	startGame := func() {
		c4.RunGame(
			SDLHuman{moveReady, nextMove},
			c4.AlphaBetaAI{
				c4.Black,
				8,
				func(game c4.State, p c4.Piece) float64 {
					// Evolved solution:
					// return c4.EvalFactors{
					// 		0.2502943943301069,
					// 		-0.4952316649483701,
					// 		0.3932539700819625,
					// 		-0.2742452616759889,
					// 		0.4746881137884282,
					// 		0.2091091127191147}.Eval(game, p)
					// Least mean squares solution after 8 iterations:
					return c4.EvalFactors{
						-0.9571905351459684, 0.5937887615514715, 1.5564004899922947e+10, 1.4372659948172928e+10, 1.4639162360474566e+10, 2.0461976265482983e+10}.Eval(game, p)
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
					gameResults <- c4.Red
				} else if winner == c4.Black {
					gameResults <- c4.Black
				} else {
					gameResults <- c4.None
				}
			})
	}
	go startGame()

loop:
	for {
		select {
		case <-ticker.C:
			screen.FillRect(
				&sdl.Rect{0, 0, SCREEN_WIDTH, SCREEN_HEIGHT},
				BOARD_COLOR)
			for col := 0; col < c4.MaxColumns; col++ {
				for row := 0; row < c4.MaxRows; row++ {
					drawPiece(screen, col, row, game.GetPiece(col, row))
				}
			}
			if showMessage {
				screen.Blit(
					&sdl.Rect{
						int16(SCREEN_WIDTH/2 - line1.W/2),
						int16(SCREEN_HEIGHT/2 - line1.H),
						0,
						0},
					line1,
					nil)
				screen.Blit(
					&sdl.Rect{
						int16(SCREEN_WIDTH/2 - line2.W/2),
						int16(SCREEN_HEIGHT / 2),
						0,
						0},
					line2,
					nil)
			}
			screen.Flip()

		case event := <-sdl.Events:
			switch e := event.(type) {
			case sdl.MouseButtonEvent:
				if waitingForMove &&
					e.Type == sdl.MOUSEBUTTONUP &&
					e.Button == sdl.BUTTON_LEFT {
					waitingForMove = false
					nextMove <- int(e.X * c4.MaxColumns / SCREEN_WIDTH)

					// Tell user that the AI is thinking now
					line1 =
						sdl.CreateRGBSurface(0, 0, 0, 32, 0, 0, 0, 0)
					line2 =
						ttf.RenderText_Blended(font,
							"Thinking...",
							sdl.Color{255, 255, 255, 0})
					showMessage = true
				} else if gameOver &&
					e.Type == sdl.MOUSEBUTTONUP &&
					e.Button == sdl.BUTTON_LEFT {
					gameOver = false
					showMessage = false
					go startGame()
				}
			case sdl.QuitEvent:
				break loop
			}

		case game = <-newState:
			// We did the assignment; there's nothing else to do

		case <-moveReady:
			waitingForMove = true
			showMessage = false

		case winner = <-gameResults:
			gameOver = true
			var message string
			if winner == c4.Red {
				message = "You win!"
			} else if winner == c4.Black {
				message = "You lose."
			} else {
				message = "Draw."
			}
			line1 =
				ttf.RenderText_Blended(font,
					message,
					sdl.Color{255, 255, 255, 0})

			line2 =
				ttf.RenderText_Blended(font,
					"Click to play again...",
					sdl.Color{255, 255, 255, 0})

			showMessage = true
		}

	}
}
