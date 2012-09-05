package main

import (
	"../c4"
	"fmt"
	"runtime"
)

func textShow(game c4.State) {
	// Board
	var piece c4.Piece
	for row := c4.MaxRows - 1; row >= 0; row-- {
		for col := 0; col < c4.MaxColumns; col++ {
			piece = game.GetPiece(col, row)
			if piece == c4.Red {
				fmt.Print("R")
			} else if piece == c4.Black {
				fmt.Print("B")
			} else {
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
	for col := 0; col < c4.MaxColumns; col++ {
		fmt.Print(col)
	}
	fmt.Print("\n\n")
	// Turn
	turn := game.GetTurn()
	if turn == c4.Red {
		fmt.Println("It is red's turn.")
	} else if turn == c4.Black {
		fmt.Println("It is black's turn.")
	}
}

type TextHuman struct{}

func (ui TextHuman) NextMove(game c4.State) int {
	var col int
	for {
		fmt.Print("Enter the column to place your piece: ")

		_, err := fmt.Scanln(&col)
		if err == nil {
			return col
		} else {
			fmt.Println()
		}
	}
	return 0
}

func main() {
	// Use all processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	c4.RunGame(
		TextHuman{},
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
		textShow,
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
}
	