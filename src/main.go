package main

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"os"
	"log"
	"encoding/json"
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

func textShow(game State) {
	// Board
	var piece Piece
	for row := MaxRows - 1; row >= 0; row-- {
		for col := 0; col < MaxColumns; col++ {
			piece = game.GetPiece(col, row)
			if piece == Red {
				fmt.Print("R")
			} else if piece == Black {
				fmt.Print("B")
			} else {
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
	for col := 0; col < MaxColumns; col++ {
		fmt.Print(col)
	}
	fmt.Print("\n\n")
	// Turn
	turn := game.GetTurn()
	if turn == Red {
		fmt.Println("It is red's turn.")
	} else if turn == Black {
		fmt.Println("It is black's turn.")
	}
}

type TextHuman struct{}

func (ui TextHuman) NextMove(game State) int {
	var col int
	for {
		fmt.Print("Enter the column to place your piece: ")

		_, err := fmt.Scanf("%d", &col)
		if err == nil {
			return col
		}
	}
	return 0
}

// An artificial intelligence that runs to some depth.
// Set depth to -1 for unlimited depth
type AlphaBetaAI struct {
	color        Piece
	depth        int
	evalFunc     func(State, Piece) float64
	terminalTest func(State) bool
}

var colCheckOrder []int

func (ai AlphaBetaAI) alphabeta(game State,
	depth int, alpha, beta float64) float64 {
	if depth == 0 || ai.terminalTest(game) {
		return ai.evalFunc(game, ai.color)
	}
	if game.GetTurn() == ai.color {
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
					ai.depth-1,
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

type evalFactors struct {
	win       float64
	lose      float64
	myOdd     float64
	theirOdd  float64
	myEven    float64
	theirEven float64
}

// Detects threats caused by p moving to (col, row)
func countThreats(game State, p Piece, col, row int) int {
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

func (f evalFactors) Eval(game State, p Piece) float64 {
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
			myOddThreats += float64(countThreats(game, p, col, row))
			theirOddThreats += float64(countThreats(game, p.Other(), col, row))
		}
	}
	// Even threats
	var myEvenThreats, theirEvenThreats float64
	for row := 1; row < MaxRows; row += 2 {
		for col := 0; col < MaxColumns; col++ {
			myEvenThreats += float64(countThreats(game, p, col, row))
			theirEvenThreats += float64(countThreats(game, p.Other(), col, row))
		}
	}
	return f.win*win +
		f.lose*lose +
		f.myEven*myEvenThreats +
		f.theirEven*theirEvenThreats +
		f.myOdd*myOddThreats +
		f.theirOdd*theirOddThreats
}

const PopSize = 100
const BattleCount = 5
const mutationStdDev = 0.03

func main() {
	// Use all processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Initialize population
	pop := make([][6]float64, 0, PopSize)
	var newPop [][6]float64
	var wins [PopSize]int
	var fitness [PopSize + 1]float64
	var generation int
	var tempGenome [6]float64
	// If there's an argument for it, read the population
	if len(os.Args) == 2 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			if os.IsNotExist(err) {
				log.Println(err)
			}
		} else {
			decoder := json.NewDecoder(file)
			if err := decoder.Decode(&pop); err != nil {
				log.Println(err)
				log.Println("Writing new file")
			}
		}
	}
	// Otherwise, generate one randomly. This also fills up empty space
	// in undersized populations that have been loaded
	for i := len(pop); i < PopSize; i++ {
		for j := 0; j < 6; j++ {
			tempGenome[j] = 2*rand.Float64() - 1
			pop = append(pop, tempGenome)
		}
	}
	var genomeOrder []int
	// Function/closures for each game
	isDone := func(game State) bool {
		return game.IsDone()
	}
	displayNoBoard := func(game State) {}
	showError := func(err error) {
		fmt.Println(err)
	}
	winnerChan := make(chan Piece, 1)
	var winner Piece
	notifyWinner := func(winner Piece) {
		if winner == Red {
			fmt.Println("Red wins!")
		} else if winner == Black {
			fmt.Println("Black wins!")
		} else {
			fmt.Println("It's a draw.")
		}
		winnerChan <- winner
	}
	// Fitness temps
	var acc float64
	var tempFitness float64
	var bestFitness float64
	var bestGenome [6]float64
	// Temps
	var g1, g2 int
	var f1, f2 evalFactors
	var randNum float64

	for {
		// Save the generation 
		// Determine fitness
		for battle := 0; battle < BattleCount; battle++ {
			// Initialize a permutation of competitors
			genomeOrder = rand.Perm(PopSize)
			for g1 = 0; g1 < PopSize; g1++ {
				g2 = genomeOrder[g1]
				f1 = evalFactors{pop[g1][0], pop[g1][1], pop[g1][2],
					pop[g1][0], pop[g1][1], pop[g1][2]}
				f2 = evalFactors{pop[g2][0], pop[g2][1], pop[g2][2],
					pop[g2][0], pop[g2][1], pop[g2][2]}
				fmt.Printf(
					"Battle %v-%v:\n\t%v (%v/%v)\n\tvs\n\t%v (%v/%v)\n\n", 
					generation, battle+1,
					f1, wins[g1], battle,
					f2, wins[g2], battle)
				// Run a game with the competitors
				RunGame(
					AlphaBetaAI{
						Red,
						8,
						func(game State, p Piece) float64 {
							return f1.Eval(game, p)
						},
						isDone,
					},
					AlphaBetaAI{
						Black,
						8,
						func(game State, p Piece) float64 {
							return f2.Eval(game, p)
						},
						isDone,
					},
					displayNoBoard,
					showError,
					notifyWinner)
				// Update win counts
				if winner = <-winnerChan; winner == Red {
					wins[g1]++
				} else if winner == Black {
					wins[g2]++
				}
			}
		}

		// Calculate win/game ratios
		acc = 0
		bestFitness = math.Inf(-1)
		for i, _ := range wins {
			tempFitness = float64(wins[i])/float64(BattleCount)
			// The actual numbers we use will be consist of weighted ranges
			// picked randomly, which we can speed up using a binary search
			fitness[i] = acc
			acc += tempFitness
			// Keep the best genome of the generation
			if tempFitness > bestFitness {
				bestFitness = tempFitness
				bestGenome = pop[i]
			}
		}
		// Add a top to the last range
		fitness[PopSize] = acc

		newPop = make([][6]float64, 0, PopSize)
		for i := 0; i < PopSize; i++ {
			// SELECTION
			// Find two random genomes
			randNum = rand.Float64() * acc
			// The binary search always goes up from randNum, except at 0,
			// so we need to compensate for that
			if randNum != 0 {
				g1 = sort.SearchFloat64s(fitness[0:len(fitness)], randNum)
				g1--
			} else {
				g1 = 0
			}
			randNum = rand.Float64() * acc
			if randNum != 0 {
				g2 = sort.SearchFloat64s(fitness[0:len(fitness)], randNum)
				g2--
			} else {
				g2 = 0
			}

			// CROSSOVER AND MUTATION
			for j := 0; j < 6; j++ {
				// CROSSOVER
				// We're just going to pick random genes.
				// I don't think gene locality is a thing here anyway
				if rand.Intn(2) == 0 {
					tempGenome[j] = pop[g1][j]
				} else {
					tempGenome[j] = pop[g2][j]
				}

				// MUTATION
				tempGenome[j] += rand.NormFloat64() * mutationStdDev
			}

			newPop = append(newPop, tempGenome)
		}
		pop = newPop[0:PopSize]

		// Write the latest generation to a file
		if len(os.Args) == 2 {
			if file, err := os.Create(os.Args[1]); err == nil {
				enc := json.NewEncoder(file)	
				enc.Encode(&generation)
				enc.Encode(&bestGenome)
				enc.Encode(&bestFitness)
			}
		}

		// Show the best fitness
		fmt.Println("Generation:  ", generation)
		fmt.Println("Best genome: ", bestGenome)
		fmt.Println("Fitness:     ", bestFitness)
		fmt.Println()
		generation++

		// Clear the variables
		for i := 0; i < PopSize; i++ {
			wins[i] = 0
		}
	}
}
