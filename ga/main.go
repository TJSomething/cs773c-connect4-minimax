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
	"sort"
	"time"
	_ "net/http/pprof"
)

const PopSize = 100
const BattleCount = 5
const mutationStdDev = 0.03

func main() {
	// Use all processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Initialize seed
	rand.Seed(time.Now().UnixNano())

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
		}
		pop = append(pop, tempGenome)
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
			fmt.Println("c4.Red wins!")
		} else if winner == c4.Black {
			fmt.Println("c4.Black wins!")
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
	var f1, f2 c4.EvalFactors
	var randNum float64

	for {
		// Save the generation 
		// Determine fitness
		for battle := 0; battle < BattleCount; battle++ {
			// Initialize a permutation of competitors
			genomeOrder = rand.Perm(PopSize)
			for g1 = 0; g1 < PopSize; g1++ {
				g2 = genomeOrder[g1]
				f1 = c4.EvalFactors{pop[g1][0], pop[g1][1], pop[g1][2],
					pop[g1][3], pop[g1][4], pop[g1][5]}
				f2 = c4.EvalFactors{pop[g2][0], pop[g2][1], pop[g2][2],
					pop[g2][3], pop[g2][4], pop[g2][5]}
				fmt.Printf(
					"\nGeneration %v, round %v/%v, genome %v/%v:\n\t"+
						"%v (%v/%v)\n\tvs\n\t%v (%v/%v)\n",
					generation, battle+1, BattleCount, g1+1, PopSize,
					f1, wins[g1], battle*2,
					f2, wins[g2], battle*2)
				// Run a game with the competitors
				c4.RunGame(
					c4.AlphaBetaAI{
						c4.Red,
						8,
						func(game c4.State, p c4.Piece) float64 {
							return f1.Eval(game, p)
						},
						isDone,
					},
					c4.AlphaBetaAI{
						c4.Black,
						8,
						func(game c4.State, p c4.Piece) float64 {
							return f2.Eval(game, p)
						},
						isDone,
					},
					displayNoBoard,
					showError,
					notifyWinner)
				// Update win counts
				if winner = <-winnerChan; winner == c4.Red {
					wins[g1]++
				} else if winner == c4.Black {
					wins[g2]++
				}
			}
		}

		// Calculate win/game ratios
		acc = 0
		bestFitness = math.Inf(-1)
		for i, _ := range wins {
			tempFitness = float64(wins[i]) / float64(BattleCount)
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
				enc.Encode(&pop)
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
