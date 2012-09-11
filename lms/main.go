package main

import (
	"../c4"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"
)

const PopSize = 10
const BattleCount = 5
const mu = 0.1

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

func (me *lmsEvaluator) Learn() {
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

	// Clear the scores and features
	me.featuresList = make([][6]float64, 0, len(me.featuresList))
	me.actualScores = make([]float64, 0, len(me.actualScores))
}

func main() {
	// Use all processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Initialize seed
	rand.Seed(time.Now().UnixNano())

	// Initialize variables
	evalFuncs := make([]lmsEvaluator, 0, PopSize)
	var wins [PopSize]int
	var generation int
	var tempCoeffs [6]float64
	// We need these to find the best player
	var mostWins int
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
	var genomeOrder []int

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
		// Sample the state space
		for battle := 0; battle < BattleCount; battle++ {
			// Initialize a permutation of competitors
			genomeOrder = rand.Perm(PopSize)
			for g1 = 0; g1 < PopSize; g1++ {
				g2 = genomeOrder[g1]
				fmt.Printf(
					"\nGeneration %v, round %v/%v, genome %v/%v:\n\t"+
						"%v (%v wins)\n\tvs\n\t%v (%v wins)\n",
					generation, battle+1, BattleCount, g1+1, PopSize,
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
				}
			}
		}

		// Find the best evaluator
		mostWins = -1
		for i, score := range wins {
			// Keep the best genome of the generation
			if score > mostWins {
				mostWins = score
				bestCoeffs = evalFuncs[i].Coeffs
			}
		}

		// Learn from the experience
		for i := 0; i < PopSize; i++ {
			evalFuncs[i].Learn()
		}

		// Write the latest generation to a file
		if len(os.Args) == 2 {
			if file, err := os.Create(os.Args[1]); err == nil {
				enc := json.NewEncoder(file)
				enc.Encode(&evalFuncs)
				enc.Encode(&generation)
				enc.Encode(&bestCoeffs)
				enc.Encode(&mostWins)
			}
		}

		// Show the best fitness
		fmt.Println("Generation:  ", generation)
		fmt.Println("Best coeffs: ", bestCoeffs)
		fmt.Println("# of wins:   ", mostWins)
		fmt.Println()
		generation++

		// Clear the variables
		for i := 0; i < PopSize; i++ {
			wins[i] = 0
		}
	}
}
