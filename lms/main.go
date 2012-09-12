package main

import (
	"../c4"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"
)

const PopSize = 3
const mu = 0.00001

type board [c4.MaxColumns][c4.MaxRows]c4.Piece

// An evaluator that keeps track of all evaluated game states and learns from
// them
type lmsEvaluator struct {
	Coeffs      [6]float64
	coeffsMutex sync.RWMutex
	count       int
}

// Makes a new evaluator
func newEvaluator(coeffs [6]float64) lmsEvaluator {
	var result lmsEvaluator
	result.Coeffs = [6]float64{coeffs[0], coeffs[1], coeffs[2],
		coeffs[3], coeffs[4], coeffs[5]}
	return result
}

// Evaluates the game state
func (me *lmsEvaluator) Eval(game c4.State, p c4.Piece) float64 {
	var bestScore float64

	// Copy out the coefficients to reduce lock contention
	me.coeffsMutex.RLock()
	myCoeffs := me.Coeffs
	me.coeffsMutex.RUnlock()

	// Estimate the game state's utility
	approxScore, currentFeatures := BetterEval(myCoeffs, game, p)

	// Try to get a better estimate of the utility by looking one move ahead
	// with proven weights
	if game.GetTurn() != p {
		bestScore = math.Inf(-1)
	} else {
		bestScore = math.Inf(+1)
	}

	for col := 0; col < c4.MaxColumns; col++ {
		if nextBoard, err := game.AfterMove(game.GetTurn(),
			col); err == nil {
			nextScore, _ := BetterEval(
				[6]float64{
					0.2502943943301069,
					-0.4952316649483701,
					0.3932539700819625,
					-0.2742452616759889,
					0.4746881137884282,
					0.2091091127191147},
				nextBoard,
				nextBoard.GetTurn())

			if game.GetTurn() != p {
				if nextScore > bestScore {
					bestScore = nextScore
				}
			} else {
				if nextScore < bestScore {
					bestScore = nextScore
				}
			}
		}
	}
	// Use the evolved weights as a reference to prevent divergence
	// knownScore, _ = BetterEval([6]float64{
	// 	0.2502943943301069,
	// 	-0.4952316649483701,
	// 	0.3932539700819625,
	// 	-0.2742452616759889,
	// 	0.4746881137884282,
	// 	0.2091091127191147}, game, p)

	// Change the coefficients according to the error
	me.count++
	if me.count%100000 == 0 {
		fmt.Println(me.count)
		fmt.Println(me.Coeffs)
	}
	// if !math.IsInf(bestScore, 0) {
	// 	for j := 0; j < 6; j++ {
	// 		me.Coeffs[j] +=
	// 			mu * (bestScore - approxScore) * currentFeatures[j]
	// 	}
	// }
	go func() {
		if !math.IsInf(bestScore, 0) {
			me.coeffsMutex.Lock()
			for j := 0; j < 6; j++ {
				me.Coeffs[j] +=
					mu * (bestScore - approxScore) * currentFeatures[j]
			}
			me.coeffsMutex.Unlock()
		}
	}()

	return approxScore
}

func BetterEval(coeffs [6]float64, game c4.State, p c4.Piece) (result float64,
	features [6]float64) {
	// Winning factor
	var win, lose float64
	var myOddThreats, theirOddThreats float64
	// Odd threats
	for row := 0; row < c4.MaxRows; row += 2 {
		for col := 0; col < c4.MaxColumns; col++ {
			myOddThreats += float64(c4.CountThreats(game, p, col, row))
			theirOddThreats += float64(c4.CountThreats(game, p.Other(), col, row))
		}
	}
	// Even threats
	var myEvenThreats, theirEvenThreats float64
	for row := 1; row < c4.MaxRows; row += 2 {
		for col := 0; col < c4.MaxColumns; col++ {
			myEvenThreats += float64(c4.CountThreats(game, p, col, row))
			theirEvenThreats += float64(c4.CountThreats(game, p.Other(), col, row))
		}
	}
	winner := game.GetWinner()
	if winner == p {
		result = 1
	} else if winner == p.Other() {
		result = -1
	} else if game.IsDone() && winner == c4.None {
		result = 0
	} else {
		result = coeffs[0]*win +
			coeffs[1]*lose +
			coeffs[2]*myEvenThreats +
			coeffs[3]*theirEvenThreats +
			coeffs[4]*myOddThreats +
			coeffs[5]*theirOddThreats
	}

	features = [6]float64{win, lose, myEvenThreats, theirEvenThreats,
		myOddThreats, theirOddThreats}

	return
}

func main() {
	// Use all processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Initialize seed
	rand.Seed(time.Now().UnixNano())

	// Initialize variables
	evalFuncs := make([]lmsEvaluator, 0, PopSize)
	var wins [PopSize]int
	var iteration int
	var tempCoeffs [6]float64
	// We need these to find the best player
	var bestCoeffs [6]float64
	// Temps
	var g1, g2 int

	// If there's an argument for it, read the evaluation functions
	if len(os.Args) == 2 {
		file, err := os.Open(os.Args[1])
		if err != nil {
			if os.IsNotExist(err) {
				log.Println(err)
			}
		} else {
			decoder := json.NewDecoder(file)
			if err := decoder.Decode(&evalFuncs); err != nil {
				log.Println(err)
				log.Println("Writing new file")
			} else if err := decoder.Decode(&iteration); err != nil {
				// We also want the the iteration number loaded
				log.Println(err)
				log.Println("Iteration number missing")
			}
		}
	}

	// Otherwise, generate them randomly. This also fills up empty space
	// if not enough load
	for i := len(evalFuncs); i < PopSize; i++ {
		for j := 0; j < 6; j++ {
			tempCoeffs[j] = 2*rand.Float64() - 1
		}
		evalFuncs = append(evalFuncs, newEvaluator(tempCoeffs))
	}

	// Function/closures for each game
	isDone := func(game c4.State) bool {
		return game.IsDone()
	}
	displayNoBoard := func(game c4.State) {}
	showError := func(err error) {
		fmt.Println(err)
	}
	winnerChan := make(chan c4.Piece, 1)

	notifyWinner := func(winner c4.Piece) {
		if winner == c4.Red {
			fmt.Println("Red wins!")
		} else if winner == c4.Black {
			fmt.Println("Black wins!")
		} else {
			fmt.Println("It's a draw.")
		}
		winnerChan <- winner
	}

	// Coefficients to keep the others honest
	evolvedRed := c4.AlphaBetaAI{
		c4.Red,
		8,
		func(game c4.State, p c4.Piece) float64 {
			result, _ := BetterEval([6]float64{
				0.2502943943301069,
				-0.4952316649483701,
				0.3932539700819625,
				-0.2742452616759889,
				0.4746881137884282,
				0.2091091127191147}, game, p)
			return result
		},
		func(game c4.State) bool {
			return game.GetWinner() != c4.None
		},
	}
	evolvedBlack := evolvedRed
	evolvedBlack.Color = c4.Black

	for {
		for g1 = 0; g1 < PopSize; g1++ {
			for g2 = 0; g2 < PopSize; g2++ {
				// Memory profiling stuff
				// memstats := new(runtime.MemStats)
				// runtime.ReadMemStats(memstats)
				// log.Printf("memstats before GC: bytes = %d footprint = %d",
				// 	memstats.HeapAlloc, memstats.Sys)

				// f, err := os.Create("bah.mprof")
				// if err != nil {
				// 	log.Fatal(err)
				// }
				// pprof.WriteHeapProfile(f)
				// f.Close()

				fmt.Printf(
					"\nIteration %v, coeffs %v/%v vs coeffs %v/%v:\n\t"+
						"%v (%v wins)\n\tvs\n\t%v (%v wins)\n",
					iteration, g1+1, PopSize, g2+1, PopSize,
					evalFuncs[g1].Coeffs, wins[g1],
					evalFuncs[g2].Coeffs, wins[g2])
				// Run a game with the competitors
				c4.RunGame(
					c4.AlphaBetaAI{
						c4.Red,
						8,
						func(game c4.State, p c4.Piece) float64 {
							return evalFuncs[g1].Eval(game, p)
						},
						isDone,
					},
					c4.AlphaBetaAI{
						c4.Black,
						8,
						func(game c4.State, p c4.Piece) float64 {
							return evalFuncs[g2].Eval(game, p)
						},
						isDone,
					},
					displayNoBoard,
					showError,
					notifyWinner)
				// Update win counts
				if winner := <-winnerChan; winner == c4.Red {
					wins[g1]++
				} else if winner == c4.Black {
					wins[g2]++
				}
			}
			// Keep them honest by playing them against a proven
			// set of coefficents
			c4.RunGame(
				evolvedRed,
				c4.AlphaBetaAI{
					c4.Black,
					8,
					func(game c4.State, p c4.Piece) float64 {
						return evalFuncs[g1].Eval(game, p)
					},
					isDone,
				},
				displayNoBoard,
				showError,
				notifyWinner)
			if winner := <-winnerChan; winner == c4.Black {
				fmt.Printf("\nCoeffs %v beat the champion as black!", g1+1)
				wins[g1]++
			}
			c4.RunGame(
				c4.AlphaBetaAI{
					c4.Red,
					8,
					func(game c4.State, p c4.Piece) float64 {
						return evalFuncs[g1].Eval(game, p)
					},
					isDone,
				},
				evolvedBlack,
				displayNoBoard,
				showError,
				notifyWinner)
			if winner := <-winnerChan; winner == c4.Red {
				fmt.Printf("\nCoeffs %v beat the champion as red!", g1+1)
				wins[g1]++
			}
		}

		// Find the new best evaluator and run learning
		mostWins := -1
		for i := 0; i < PopSize; i++ {
			// Keep the best coefficients of the iteration
			if mostWins < wins[i] {
				mostWins = wins[i]
				bestCoeffs = evalFuncs[i].Coeffs
			}
		}

		// Write the latest iteration to a file
		if len(os.Args) == 2 {
			if file, err := os.Create(os.Args[1]); err == nil {
				enc := json.NewEncoder(file)
				enc.Encode(&evalFuncs)
				enc.Encode(&iteration)
				enc.Encode(&bestCoeffs)
				enc.Encode(&mostWins)
			}
		}

		// Show the best fitness
		fmt.Println("Iteration:   ", iteration)
		fmt.Println("Best coeffs: ", bestCoeffs)
		fmt.Println("Wins:        ", mostWins)
		fmt.Println()
		iteration++

		// Clear the variables
		for i := 0; i < PopSize; i++ {
			wins[i] = 0
		}
	}
}
