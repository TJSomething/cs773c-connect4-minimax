Assignment 0
============

This is an AI that plays Connect Four using MiniMax with alpha-beta pruning,
as well as accompanying programs that use the algorithms for the AI.

As such, there are 3 programs that can be built:
* Genetic algorithm in the `ga` directory
* Graphical game in the `sdl-game` directory
* Text-based game in the `text-game` directory

Binaries are in the Downloads tab above.

Dependencies
------------

All programs require [Go](http://golang.org) to compile.
`sdl-game` also requires [Go-SDL](https://github.com/0xe2-0x9a-0x9b/Go-SDL/).

Build
-----

1. Change into the directory for the program (e.g. `cd ga`).
2. Run `go build`. This should give you an executable with the same name as
   the directory.

Use
---

### `ga [<population file>]`

On starting, if the population file is specified and exists, `ga` will
load the population, allowing for the resumption of running `ga` from a
previous instance. If not, the population will be randomly generated from
a uniform distribution over [-1,1]^6.

Everytime after a new generation is crossed over and mutated, the population
is saved, along with the generation number, the best genome from the previous
generation, and the fitness of that genome.

### `text-game`

You start as the first player, red, while the computer plays the second,
black. The board is shown as follows:
	       
	   B   
	   R   
	   RR  
	   BB  
	   RB  
	0123456

	It is red's turn.
	Enter the column to place your piece:

`R` represents your pieces, while `B` represents the computer's pieces.

On each move, you enter the number of the column where you would like to place
a piece, as shown on the bottom of the board.

### `sdl-game`

You start as the first player, red, while the computer plays the second,
black. Click on a column to place a piece.

Static Evaluator
----------------

The static evaluator function is essentially copied from Jenny Lam's report
"[Heuristics in the game of Connect-K](http://www.ics.uci.edu/~jlam2/connectk.pdf)."
However, some slight changes were made.
The resulting function is a linear combination of the following:

1. `p_1` win: This is 1 when the AI is at a winning state, and 0 otherwise.
2. `p_2` win: This is 1 when the oppenent is at a winning state, and 0 otherwise.
3. `p_1` odd threats: number of lines of three of `p_1`'s pieces that have
   an empty spot on one side, where the empty spot is in an odd row.
4. `p_2` odd threats: likewise, but for `p_2` instead of `p_1`
5. `p_1` even threats: number of lines of three of `p_1`'s pieces that have
   an empty spot on one side, where the empty spot is in an even row.
6. `p_2` even threats: likewise, but for `p_2` instead of `p_1`

where `p_1` is the AI and `p_2` is the opponent.

### Coefficients

The coefficients for each of these is then found using a genetic algorithm.

Fitness is determined by counting the number of wins after running 5 rounds
of trials, wherein each genome is set against another random genome. To
prevent genomes from participating in a disproportionate number of trials,
the second player is actually selected, in order, from a random permutation
of all genomes.

For the genetic steps, roulette selection with uniform crossover is used.
This is followed with a mutation operator, where a random number from
the distribution N(0, 0.0009) is added to each gene.

After running this for 10 generations on a population of 100, the following
coefficents were found:

	( 0.2502943943301069,
	 -0.4952316649483701,
	  0.3932539700819625,
	 -0.2742452616759889,
	  0.4746881137884282,
	  0.2091091127191147)

The order of these correspond to the values in the list above.

### Strengths and weaknesses

I cannot actually beat this static evaluator with these coefficients. It 
frequently leads me into unwinnable situations. However, I will admit that
I am terrible at Connect Four, as can be seen in 
[this video](http://youtu.be/0JSBRwHBv6Q).