package main

import (
	"../c4"
	"encoding/json"
	"errors"
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

// An evaluator that keeps track of all evaluated game states and learns from
// them
type lmsEvaluator struct {
	Coeffs         [6]float64
	featuresList   [][6]float64
	actualScores   []float64
	featuresMutex  sync.Mutex
	featuresSynced sync.WaitGroup
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
	var features [6]float64

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

	// Send a goroutine to add stuff to the features vector, so
	// synchronization doesn't slow us down.
	go func() {
		me.featuresSynced.Add(1)
		me.featuresMutex.Lock()
		me.featuresList = append(me.featuresList, features)
		me.featuresMutex.Unlock()
		me.featuresSynced.Done()
	}()

	// Add up the results
	var result float64
	for i := 0; i < 6; i++ {
		result += features[i] + me.Coeffs[i]
	}

	return result
}

func (me *lmsEvaluator) EndGame(score float64) {
	// Wait for writes to the features list to end
	me.featuresSynced.Wait()
	me.featuresMutex.Lock() // This shouldn't actually be necessary.

	// Keep track of the actual scores, along side the feature vectors
	for len(me.actualScores) < len(me.featuresList) {
		me.actualScores = append(me.actualScores, score)
	}

	me.featuresMutex.Unlock()
}

func (me *lmsEvaluator) Learn() float64 {
	// Make there are as many features as there are scores
	if len(me.featuresList) != len(me.actualScores) {
		panic(errors.New("There are fewer scores than features."))
	}

	var approxScore float64
	for i := 0; i < len(me.actualScores); i++ {
		// Use the latest weights to approximate the score
		approxScore = 0
		for j := 0; j < 6; j++ {
			approxScore += me.featuresList[i][j] * me.Coeffs[j]
		}

		// Update the weight using the error for the feature
		for j := 0; j < 6; j++ {
			me.Coeffs[j] +=
				mu * (me.actualScores[i] - approxScore) *
					me.featuresList[i][j]
		}
	}

	// Find the coefficients that give the least error
	var averageError float64
	for i := 0; i < len(me.actualScores); i++ {
		approxScore = 0
		for j := 0; j < 6; j++ {
			approxScore += me.featuresList[i][j] * me.Coeffs[j]
		}

		averageError += math.Abs(approxScore - me.actualScores[i])
	}
	averageError /= float64(len(me.actualScores))

	// Clear the scores and features
	me.featuresList = make([][6]float64, 0, cap(me.featuresList))
	me.actualScores = make([]float64, 0, cap(me.actualScores))

	return averageError
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
	var winner c4.Piece
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
				if winner = <-winnerChan; winner == c4.Red {
					evalFuncs[g1].EndGame(+1)
					evalFuncs[g2].EndGame(-1)
					wins[g1]++
				} else if winner == c4.Black {
					evalFuncs[g1].EndGame(-1)
					evalFuncs[g2].EndGame(+1)
					wins[g2]++
				} else {
					evalFuncs[g1].EndGame(0)
					evalFuncs[g2].EndGame(0)
				}
			}
		}

		// Run learning and find the new best evaluator
		leastError = math.Inf(1)
		for i := 0; i < PopSize; i++ {
			// Learn and calculate the new error
			averageError := evalFuncs[i].Learn()
			// Keep the best coefficients of the iteration
			if leastError > averageError {
				leastError = averageError
				bestCoeffs = evalFuncs[i].Coeffs
			}
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
