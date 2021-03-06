package c4

import (
	"errors"
	"fmt"
	"math"
)

const MaxColumns = 7
const MaxRows = 6
const WinCount = 4

const (
	None = iota
	Red
	Black
)

type Piece byte

func (p Piece) Other() Piece {
	if p == Red {
		return Black
	} else if p == Black {
		return Red
	}
	return None
}

// This is all game state management

// Note that row 0, column 0 is the lower-left
type State struct {
	board    [MaxColumns][MaxRows]Piece
	top      [MaxColumns]int
	turn     Piece
	lastMove int
}

func NewState() State {
	return State{
		[MaxColumns][MaxRows]Piece{},
		[MaxColumns]int{},
		Red,
		0}
}

func (this *State) Move(player Piece, col int) error {
	// Catch all the invalid states and moves
	if col >= 0 && col < MaxColumns &&
		(player == Red || player == Black) &&
		player == this.turn &&
		this.top[col] < MaxRows {
		// Add the piece
		this.lastMove = col
		this.board[col][this.top[col]] = player
		// The next piece goes one row higher
		this.top[col]++
		// Change turns
		this.turn = this.turn.Other()
		return nil
	}
	return errors.New(fmt.Sprintf(
		"Invalid move by player %v to column %v", player, col))
}

func (this State) AfterMove(player Piece, col int) (game State, err error) {
	game = this
	err = game.Move(player, col)
	return
}

func (this State) IsLegal(player Piece, col int) bool {
	return this.top[col] < MaxRows && player == this.turn
}

func (this State) GetPiece(col, row int) Piece {
	if col >= 0 && col < MaxColumns &&
		row >= 0 && row < MaxRows {
		return this.board[col][row]
	}
	return None
}

func (this State) GetTop(col int) int {
	if col >= 0 && col < MaxColumns {
		return this.top[col]
	}
	return 0
}

func (this State) GetTurn() Piece {
	return this.turn
}

func (this State) GetWinner() Piece {
	return lineTest(this, this.lastMove, this.top[this.lastMove]-1)
}

func (this State) GetBoard() [MaxColumns][MaxRows]Piece {
	return this.board
}

func (this State) IsDone() bool {
	// Check for a winner
	if this.GetWinner() != None {
		return true
	}
	// Check if the board is full
	full := true
	for col := 0; col < MaxColumns; col++ {
		full = full && this.top[col] == MaxRows
	}
	return full
}

// This stuff is all for checking for wins

type pieceCounter struct {
	count     int
	lastPiece Piece
}

func makeCounter() pieceCounter {
	return pieceCounter{0, None}
}

func (this *pieceCounter) add(piece Piece) Piece {
	if piece != None {
		// Count same pieces
		if piece == this.lastPiece {
			this.count++
			if this.count >= 4 {
				return piece
			}
			// Restart the count if not
		} else {
			this.count = 1
			this.lastPiece = piece
		}
		// Ignore empty spaces
	} else {
		this.lastPiece = None
		this.count = 0
	}
	return None
}

func checkDirection(game State, col, row, cOffset, rOffset, length int) Piece {
	cnt := makeCounter()
	// Where do we stop?
	lastCol := col + cOffset*length
	lastRow := row + rOffset*length
	// Start checking at the beginning of a possible line
	col -= cOffset * (length - 1)
	row -= rOffset * (length - 1)
	for col != lastCol || row != lastRow {
		if p := game.GetPiece(col, row); cnt.add(p) != None {
			return p
		}
		col += cOffset
		row += rOffset
	}
	return None
}

// Tests if a location lies on a winning line of pieces
func lineTest(game State, col, row int) Piece {
	if game.GetPiece(col, row) != None {
		// Try ALL the directions!
		if p := checkDirection(game, col, row, 0, 1, WinCount); p != None {
			return p
		} else if p := checkDirection(game, col, row, 1, 0, WinCount); p != None {
			return p
		} else if p := checkDirection(game, col, row, 1, 1, WinCount); p != None {
			return p
		} else if p := checkDirection(game, col, row, 1, -1, WinCount); p != None {
			return p
		} else {
			return None
		}
	}
	return None
}

type Player interface {
	NextMove(State) int
}

func RunGame(redPlayer Player, blackPlayer Player,
	showFunc func(State), errFunc func(error), endFunc func(Piece)) {
	game := NewState()
	var currentColor Piece = Red
	currentPlayer := redPlayer
	var currentMove int
	for {
		showFunc(game)
		// After a successful move
		currentMove = currentPlayer.NextMove(game)
		if err := game.Move(currentColor, currentMove); err == nil {
			// Check for a game ending
			if game.IsDone() {
				showFunc(game)
				endFunc(game.GetWinner())
				break
			}
			// Switch players
			if currentColor == Red {
				currentColor = Black
				currentPlayer = blackPlayer
			} else {
				currentColor = Red
				currentPlayer = redPlayer
			}
		}
	}
}

// An artificial intelligence that runs to some depth.
// Set depth to -1 for unlimited depth
type AlphaBetaAI struct {
	Color        Piece
	Depth        int
	EvalFunc     func(State, Piece) float64
	TerminalTest func(State) bool
}

var colCheckOrder []int

func (ai AlphaBetaAI) alphabeta(game State,
	depth int, alpha, beta float64) float64 {
	if depth == 0 || ai.TerminalTest(game) {
		return ai.EvalFunc(game, ai.Color)
	}
	if game.GetTurn() == ai.Color {
		for _, col := range colCheckOrder {
			if nextState, err := game.AfterMove(game.GetTurn(), col); err == nil {
				alpha = math.Max(
					alpha,
					ai.alphabeta(
						nextState,
						depth-1,
						alpha,
						beta))
				if beta <= alpha {
					break
				}
			}
		}
		return alpha
	}
	for _, col := range colCheckOrder {
		if nextState, err := game.AfterMove(game.GetTurn(), col); err == nil {
			beta = math.Min(
				beta,
				ai.alphabeta(
					nextState,
					depth-1,
					alpha,
					beta))
			if beta <= alpha {
				break
			}
		}
	}
	return beta
}

type MoveScore struct {
	Col   int
	Score float64
}

func (ai AlphaBetaAI) NextMove(game State) int {
	bestMove := -1
	bestScore := math.Inf(-1)
	var score float64
	scores := make(chan MoveScore)

	// Initialize order to check columns
	if len(colCheckOrder) == 0 {
		colCheckOrder = make([]int, 0, MaxColumns)
		col := MaxColumns / 2
		// Alternate between above and below the center, starting at the center
		for len(colCheckOrder) < MaxColumns {
			colCheckOrder = append(colCheckOrder, col)
			col -= 2*(col-MaxColumns/2) + col/(MaxColumns/2)
		}
	}

	for col := 0; col < MaxColumns; col++ {
		go func(col int) {
			if nextState, err := game.AfterMove(game.GetTurn(), col); err == nil {
				score = ai.alphabeta(
					nextState,
					ai.Depth-1,
					math.Inf(-1),
					math.Inf(+1))
				//fmt.Println("Move %v, score %v", col, score)
				scores <- MoveScore{col, score}
				return
			}
			// Illegal moves are very bad
			scores <- MoveScore{col, math.Inf(-1)}
		}(col)
	}
	for count := 0; count < MaxColumns; count++ {
		ms := <-scores
		if ms.Score > bestScore {
			bestMove = ms.Col
			bestScore = ms.Score
			// If our heuristic isn't very smooth, add randomness to prevent
			// prevent predictability
		} else if ms.Score == bestScore {
			if math.Abs(float64(ms.Col)-MaxColumns/2-0.25) <
				math.Abs(float64(bestMove)-MaxColumns/2-0.25) {
				bestMove = ms.Col
			}
		}
	}
	return bestMove
}

type EvalFactors struct {
	Win       float64
	Lose      float64
	MyOdd     float64
	TheirOdd  float64
	MyEven    float64
	TheirEven float64
}

// Detects threats caused by p moving to (col, row)
func CountThreats(game State, p Piece, col, row int) int {
	// Empty spots don't cause threats
	if game.GetPiece(col, row) != None {
		return 0
	}
	tryLine := func(col, row, cOffset, rOffset int) int {
		for count := 0; count < WinCount-1; count++ {
			col += cOffset
			row += rOffset
			if game.GetPiece(col, row) != p {
				if count == 3 {
					fmt.Printf("Threat at %d, %d\n", col, row)
				}
				return 0
			}
		}
		return 1
	}
	return tryLine(col, row, 1, 1) +
		tryLine(col, row, 1, -1) +
		tryLine(col, row, -1, -1) +
		tryLine(col, row, -1, 1)
}

func (f EvalFactors) Eval(game State, p Piece) float64 {
	// Winning factor
	var win, lose float64
	winner := game.GetWinner()
	if winner == p {
		win = 1
		lose = 0
	} else if winner == None {
		win = 0
		lose = 0
	} else {
		win = 0
		lose = 1
	}
	var myOddThreats, theirOddThreats float64
	// Odd threats
	for row := 0; row < MaxRows; row += 2 {
		for col := 0; col < MaxColumns; col++ {
			myOddThreats += float64(CountThreats(game, p, col, row))
			theirOddThreats += float64(CountThreats(game, p.Other(), col, row))
		}
	}
	// Even threats
	var myEvenThreats, theirEvenThreats float64
	for row := 1; row < MaxRows; row += 2 {
		for col := 0; col < MaxColumns; col++ {
			myEvenThreats += float64(CountThreats(game, p, col, row))
			theirEvenThreats += float64(CountThreats(game, p.Other(), col, row))
		}
	}
	return f.Win*win +
		f.Lose*lose +
		f.MyEven*myEvenThreats +
		f.TheirEven*theirEvenThreats +
		f.MyOdd*myOddThreats +
		f.TheirOdd*theirOddThreats
}
