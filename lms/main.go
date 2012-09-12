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
	Coeffs         [6]float64
	featuresList   [][6]float64
	stateList      []c4.State
	scoreMemo      map[board]float64
	featuresMutex  sync.Mutex
	featuresSynced sync.WaitGroup
	side           c4.Piece
}

// Makes a new evaluator
func newEvaluator(coeffs [6]float64) lmsEvaluator {
	var result lmsEvaluator
	result.Coeffs = [6]float64{coeffs[0], coeffs[1], coeffs[2],
		coeffs[3], coeffs[4], coeffs[5]}
	result.Init()
	return result
}

func (this *lmsEvaluator) Init() {
	// Pre-allocate a crap ton of memory
	this.featuresList = make([][6]float64, 0, PopSize*PopSize*350000)
	this.stateList = make([]c4.State, 0, PopSize*PopSize*350000)
	this.scoreMemo = make(map[board]float64)
	this.side = c4.None
}

// Evaluates the game state
func (me *lmsEvaluator) Eval(game c4.State, p c4.Piece) float64 {
	var features [6]float64
	if me.side != p {
		me.side = p
	}

	// Calculate the features of the game state
	// Winning factor
	winner := game.GetWinner()
	if winner == p {
		features[0] = 1
		features[1] = 0
	} else if winner == c4.None {
		features[0] = 0
		features[1] = 0
	} else {
		features[0] = 0
		features[1] = 1
	}
	// Odd threats
	for row := 0; row < c4.MaxRows; row += 2 {
		for col := 0; col < c4.MaxColumns; col++ {
			features[2] += float64(c4.CountThreats(game, p, col, row))
			features[3] += float64(c4.CountThreats(game, p.Other(), col, row))
		}
	}
	// Even threats
	for row := 1; row < c4.MaxRows; row += 2 {
		for col := 0; col < c4.MaxColumns; col++ {
			features[4] += float64(c4.CountThreats(game, p, col, row))
			features[5] += float64(c4.CountThreats(game, p.Other(), col, row))
		}
	}

	// Add up the results
	var result float64
	for i := 0; i < 6; i++ {
		result += features[i] * me.Coeffs[i]
	}

	//go func(me *lmsEvaluator, features [6]float64,
	//	game c4.State) {
	me.featuresSynced.Add(1)
	me.featuresMutex.Lock()
	me.featuresList = append(me.featuresList, features)
	me.stateList = append(me.stateList, game)
	// if len(me.stateList)%100000 == 0 {
	// 	fmt.Println("Length", len(me.stateList))
	// }
	me.scoreMemo[game.GetBoard()] = result
	me.featuresMutex.Unlock()
	me.featuresSynced.Done()
	//}(me, features, game)

	return result
}

func (me *lmsEvaluator) Learn() {
	var approxScore float64
	var successorExists bool
	var bestScore float64
	var currentState c4.State
	var count int

	me.featuresSynced.Wait()

	for i := 0; i < len(me.featuresList); i++ {
		// Check if we have a successor to use for training
		successorExists = true
		currentState = me.stateList[i]
		if currentState.GetTurn() == me.side {
			bestScore = math.Inf(-1)
		} else {
			bestScore = math.Inf(+1)
		}

		for col := 0; col < c4.MaxColumns; col++ {
			if nextBoard, err := currentState.AfterMove(currentState.GetTurn(),
				col); err == nil {
				var nextScore float64
				nextScore, ok := me.scoreMemo[nextBoard.GetBoard()]
				if !ok && nextBoard.IsDone() {
					nextScore = me.scoreMemo[currentState.GetBoard()]
					ok = true
				}

				if ok {
					if currentState.GetTurn() == me.side {
						if nextScore > bestScore {
							bestScore = nextScore
						}
					} else {
						if nextScore < bestScore {
							bestScore = nextScore
						}
					}
				} else {
					successorExists = false
				}
			}
		}

		// Use the latest weights to approximate the score
		approxScore = 0
		for j := 0; j < 6; j++ {
			approxScore += me.featuresList[i][j] * me.Coeffs[j]
		}

		// Update the weight using the error for the feature
		if successorExists && !math.IsInf(bestScore, 0) {
			for j := 0; j < 6; j++ {
				me.Coeffs[j] +=
					mu * (bestScore - approxScore) *
						me.featuresList[i][j]
			}
			count++
		}
	}

	// Clear the scores and features
	me.featuresList = me.featuresList[0:0]
	me.stateList = me.stateList[0:0]
	me.scoreMemo = make(map[board]float64)

	fmt.Println(count)
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
	var leastError float64
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

	// If we've got evalFuncs loaded, then we need to initialize the private
	// fields
	for i, _ := range evalFuncs {
		evalFuncs[i].Init()
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
		}

		// Find the new best evaluator and run learning
		mostWins := -1
		for i := 0; i < PopSize; i++ {
			// Keep the best coefficients of the iteration
			if mostWins < wins[i] {
				mostWins = wins[i]
				bestCoeffs = evalFuncs[i].Coeffs
			}
			// Learn
			evalFuncs[i].Learn()
		}

		// Learn from the experience
		for i := 0; i < PopSize; i++ {
			evalFuncs[i].Learn()
		}

		// Write the latest iteration to a file
		if len(os.Args) == 2 {
			if file, err := os.Create(os.Args[1]); err == nil {
				enc := json.NewEncoder(file)
				enc.Encode(&evalFuncs)
				enc.Encode(&iteration)
				enc.Encode(&bestCoeffs)
				enc.Encode(&leastError)
			}
		}

		// Show the best fitness
		fmt.Println("Iteration:   ", iteration)
		fmt.Println("Best coeffs: ", bestCoeffs)
		fmt.Println("Error:       ", leastError)
		fmt.Println()
		iteration++

		// Clear the variables
		for i := 0; i < PopSize; i++ {
			wins[i] = 0
		}
	}
}
