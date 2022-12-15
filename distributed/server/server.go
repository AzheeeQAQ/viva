package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol"

	"uk.ac.bris.cs/gameoflife/util"
)

// RemoteMachine type to register rpc
type RemoteMachine struct{}

// create a lock
var mu sync.Mutex

// create a turn to store the turn
var turn int

func InitializeBoard(request gol.Request) [][]byte {
	Board := make([][]byte, request.Params.ImageHeight)
	for i := range Board {
		Board[i] = make([]byte, request.Params.ImageWidth)
	}
	Board = request.World

	newBoard := make([][]byte, len(Board)+2)
	for i := range newBoard {
		if i == 0 {

			newBoard[i] = make([]byte, len(Board[0]))
			copy(newBoard[i], request.BoardTop)
		} else if i == len(newBoard)-1 {
			newBoard[i] = make([]byte, len(Board[0]))
			copy(newBoard[i], request.BoardBottom)
		} else {
			newBoard[i] = make([]byte, len(Board[0]))
			copy(newBoard[i], Board[i-1])
		}
	}
	if request.Params.Threads == 1 {
		mu.Lock()
		Board = calculateNextState(newBoard)
		turn++
		mu.Unlock()
	} else {
		mu.Lock()
		numJobs := len(Board)
		jobs := make(chan int, numJobs)
		results := make(chan []util.Cell, numJobs)

		for w := 1; w <= request.Params.Threads; w++ {
			go worker(jobs, newBoard, request.Params, results)
		}

		for j := 0; j < numJobs; j++ {
			jobs <- j
		}

		close(jobs)

		var parts []util.Cell
		for a := 1; a <= numJobs; a++ {
			part := <-results
			parts = append(parts, part...)
		}
		Board = CreateWorld(parts, Board)
		//make world from alive cell
		turn++
		//mu.Unlock()
	}
	return Board
}

// CalculateNextState the caller to the StateCalculator
func (r *RemoteMachine) CalculateNextState(req gol.Request, res *gol.Response) (e error) {
	fmt.Println(231231)
	// err LocalMachine  InitializeBoard
	res.WorldRes = InitializeBoard(req)
	fmt.Println("Calculating new Board")
	return
}

func (r *RemoteMachine) GetAliveNums(req gol.NewReq, res *gol.NewRes) (e error) {
	// calculate alive cells
	mu.Lock()
	var board [][]byte
	cell := len(calculateAliveCells(board))
	turn := turn
	res.N = cell
	res.T = turn
	mu.Unlock()
	return
}

// GetCompletedTurns get the number of alive cells for TurnComplete event
func (r *RemoteMachine) GetCompletedTurns(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Lock()
	res.T = turn
	mu.Unlock()
	return
}

func (r *RemoteMachine) ServerOff(req gol.NewReq, res *gol.NewRes) (e error) {
	panic(404)
	return
}

func worker(images chan int, world [][]byte, p gol.Params, c chan []util.Cell) {
	for width := range images {
		//every slice is a workerWorld. It will return the alive cells of each workerWorld
		cells := workerWorld(width, world, p)

		//send the alive cells to the channel
		for i, aliveCell := range cells {
			cells[i] = util.Cell{width, aliveCell.X}
		}
		c <- cells
	}
}

func workerWorld(width int, world [][]byte, p gol.Params) []util.Cell {

	//create a newWorld to store the new workerWorld
	newWorld := make([][]byte, 1)
	newWorld[0] = make([]byte, p.ImageWidth)

	//check the newWold's neighbour
	for h, cell := range world[width] {
		aliveNeighbour := checkNeighbour(width, h, world)

		//judge which cells are alive
		if cell == 255 {
			if aliveNeighbour < 2 || aliveNeighbour > 3 {
				newWorld[0][h] = 0
			} else {
				newWorld[0][h] = 255
			}
		} else {
			if aliveNeighbour == 3 {
				newWorld[0][h] = 255
			} else {
				newWorld[0][h] = 0
			}
		}
	}

	//calculate the alive Cells and return them
	aliveCells := calculateAliveCells(newWorld)
	return aliveCells
}

func calculateNextState(world [][]byte) [][]byte {

	//create a newWorld to store the newWorld
	newWorld := make([][]byte, len(world))
	for i := range newWorld {
		newWorld[i] = make([]byte, len(world[i]))
	}
	//check the newWold's neighbour
	for w := range newWorld {
		for h := range newWorld[w] {
			aliveNeighbour := checkNeighbour(w, h, world)

			//judge which cells are alive
			if world[w][h] == 255 {
				if aliveNeighbour < 2 || aliveNeighbour > 3 {
					newWorld[w][h] = 0
				} else {
					newWorld[w][h] = 255
				}
			} else {
				if aliveNeighbour == 3 {
					newWorld[w][h] = 255
				} else {
					newWorld[w][h] = 0
				}
			}
		}
	}
	return newWorld
}

func checkNeighbour(width int, height int, world [][]byte) int {
	//initialise neighbours
	neighbour := 0

	//get cells neighbour
	for i := width - 1; i <= width+1; i++ {
		for j := height - 1; j <= height+1; j++ {
			//ignore itself
			if i == width && j == height {
				continue
			}

			x := i
			y := j

			//if the cell is the bound
			if x < 0 {
				x = len(world) - 1
			}
			if x >= len(world) {
				x = 0
			}
			if y < 0 {
				y = len(world[0]) - 1
			}
			if y >= len(world[0]) {
				y = 0
			}

			if world[x][y] == 255 {
				neighbour++
			}
		}
	}
	return neighbour
}

// calculate the alive cells
func calculateAliveCells(world [][]byte) []util.Cell {

	aliveCells := []util.Cell{}

	for w, width := range world {
		for h := range width {
			if world[w][h] == 255 {
				aliveCells = append(aliveCells, util.Cell{h, w})
			}
		}
	}
	return aliveCells[0:]
}

// make a new world from cells

func CreateWorld(cells []util.Cell, World [][]uint8) [][]uint8 {
	var CreateWorld [][]byte
	CreateWorld = make([][]byte, len(World))
	for i := range CreateWorld {
		CreateWorld[i] = make([]byte, len(World[0]))
	}
	for _, cell := range cells {
		CreateWorld[cell.Y][cell.X] = 255
	}
	return CreateWorld[0:]
}

func main() {
	pAddr := flag.String("port", "8050", "Port to listen on")
	flag.Parse()
	err := rpc.Register(&RemoteMachine{})
	if err != nil {
		log.Fatal("register server : ", err)
		return
	}
	ln, _ := net.Listen("tcp", "44.201.102.14:"+*pAddr)

	//listener1, _ := net.Listen("tcp", ":"+*pAddr)
	//go func() {
	//	rpc.Accept(listener1)
	//}()

	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal("closed server listening: ", err)
			return
		}
	}(ln)
	fmt.Println("Waiting for the broker...")
	rpc.Accept(ln)

}
