package gol

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type LocalMachine struct {
	c distributorChannels
}

// call to return
func callReturn(client *rpc.Client, initialWorld [][]byte, p Params) [][]byte {
	request := Request{World: initialWorld, Params: p}
	response := new(Response)
	err := client.Call(BrokerCalculate, request, response)
	if err != nil {
		log.Fatal("calling err", err)
	}

	return response.WorldRes
}

func handleServer(c distributorChannels, done <-chan bool) {
	//Set the server
	err := rpc.Register(&LocalMachine{c: c})
	if err != nil {
		log.Fatal("err LocalMachine register: ", err)
		return
	}
	ln, err := net.Listen("tcp", ":8060")
	if err != nil {
		log.Fatal("err LocalMachine problem: ", err)
	}
	defer func(ln net.Listener) {
		err := ln.Close()
		if err != nil {
			log.Fatal("listener closing: ", err)
			return
		}
	}(ln)
	rpc.Accept(ln)
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.

	// create a lock
	var mu sync.Mutex
	//create a turn to store the turn
	var turn int

	//create a filename
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioFilename <- filename
	c.ioCommand <- ioInput

	//server address 8050
	TestTry := make(chan bool)
	go handleServer(c, TestTry)

	//ClientAddr := "127.0.0.1:8030"

	client, e := rpc.Dial("tcp", "0.0.0.0:8030")
	if e != nil {
		log.Fatal("err while dial", e)
	}
	defer client.Close()

	//create a 2D world
	world := make([][]byte, p.ImageWidth)
	for i := range world {
		world[i] = make([]byte, p.ImageHeight)
	}

	//initialise the world
	for j := range world {
		for i := range world[0] {
			cell := <-c.ioInput
			world[j][i] = cell
			if cell == 255 {
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{j, i}}
			}
		}
	}

	//panic: send on closed channel. Tell the ticker when to close the channel

	//report the number of cells that are still alive every 2 seconds

	//press the key to execute different commands
	go keyPress(client, p, c, &mu)

	// TODO: Execute all turns of the Game of Life.

	finish := make(chan bool)
	go ticker(client, finish, c, &mu)

	// calling to get res
	fmt.Println("Calling...")
	WorldRes := callReturn(client, world, p)

	//tell the ticker to close the channel
	finish <- true

	// TODO: Report the final state using FinalTurnCompleteEvent.

	cells := calculateAliveCells(WorldRes)

	pgmFilename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
	c.ioFilename <- pgmFilename
	c.ioCommand <- ioOutput

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			cell := WorldRes[y][x]
			c.ioOutput <- cell
		}
	}

	imageOutputEvent := ImageOutputComplete{CompletedTurns: p.Turns, Filename: pgmFilename}
	c.events <- imageOutputEvent

	completeEvent := FinalTurnComplete{p.Turns, cells}
	c.events <- completeEvent

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
	TestTry <- true
}

func offLine(client *rpc.Client) {
	request := new(NewReq)
	response := new(NewRes)
	err := client.Call(BrokerOffLine, request, response)
	if err != nil {
		log.Fatal("shutdown err : ", err)
	}
}

// press the key to execute different commands
func keyPress(broker *rpc.Client, p Params, c distributorChannels, mu sync.Locker) {

	for {
		button := <-c.keyPresses
		request := new(NewReq)
		response := new(NewRes)
		err := broker.Call(BrokerCalculateAliveNums, request, response)
		if err != nil {
			log.Fatal("calling broker err")
		}
		turn := response.T

		switch button {
		//Capture the image of the current round
		case 's':
			pgmFilename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
			c.ioFilename <- pgmFilename
			c.ioCommand <- ioOutput

			var CopyWorld [][]byte
			err := broker.Call("CopyWorld", request, response)
			if err != nil {
				log.Fatal("calling broker copy err")
			}
			CopyWorld = response.Board

			for y := 0; y < p.ImageWidth; y++ {
				for x := 0; x < p.ImageHeight; x++ {
					c.ioOutput <- CopyWorld[y][x]
				}
			}

			imageOutputEvent := ImageOutputComplete{CompletedTurns: turn, Filename: pgmFilename}
			c.events <- imageOutputEvent

		//  LocalController shut down with no error generating in server
		case 'q':
			err := broker.Call(BrokerShutDown, request, response)
			if err != nil {
				log.Fatal("controller out err : ", err)
			}
			fmt.Println("controllerOut")

			os.Exit(0)

			//all components of the distributed system are shut down cleanly with a latest PGM
		case 'k':
			pgmFilename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
			c.ioFilename <- pgmFilename
			c.ioCommand <- ioOutput

			var CopyWorld [][]byte
			err := broker.Call(BrokerCopyWorld, request, response)
			if err != nil {
				log.Fatal("calling broker copy err")
			}
			CopyWorld = response.Board

			for y := 0; y < p.ImageWidth; y++ {
				for x := 0; x < p.ImageHeight; x++ {
					c.ioOutput <- CopyWorld[y][x]
				}
			}

			imageOutputEvent := ImageOutputComplete{CompletedTurns: turn, Filename: pgmFilename}
			c.events <- imageOutputEvent

			go offLine(broker)
			os.Exit(1)

			//pause the processing on the AWS and print turn that is being processed, again to resume
		case 'p':
			//print current turn
			fmt.Println(turn)
			err := broker.Call(BrokerMuLock, request, response)
			if err != nil {
				log.Fatal("locking mutex err", err)
			}
			for {
				//next press
				button = <-c.keyPresses
				if button == 'p' {
					break
				}
			}
			e := broker.Call(BrokerMuUnLock, request, response)
			if e != nil {
				log.Fatal("err while unlocking global mutex : ", err)
			}

			fmt.Println("Continuing")
		}
	}
}

// report the number of cells that are still alive every 2 seconds
func ticker(client *rpc.Client, finish chan bool, c distributorChannels, mu sync.Locker) {
	//report every 2 seconds
	ticker := time.NewTicker(2 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				request := new(NewReq)
				response := new(NewRes)
				// make an RPC call to get live cells
				err := client.Call(BrokerCalculateAliveCells, request, response)
				if err != nil {
					log.Fatal("error with getting alive number")
				}
				defer client.Close()
				c.events <- AliveCellsCount{response.T, response.N}

			case <-finish:
				return

			}
		}
	}()
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
	return aliveCells
}

func (l *LocalMachine) TurnCompleted(req NewReq, res *NewRes) (e error) {
	l.c.events <- TurnComplete{CompletedTurns: req.T}
	return
}

// TellTheDifference server call the LocalMachine every turn to report CellFlipped event
func (l *LocalMachine) TellTheDifference(req NewReq, res *NewRes) (e error) {
	for y := 0; y < len(req.OldBoard); y++ {
		for x := 0; x < len(req.OldBoard); x++ {
			newVal := req.NewBoard[y][x]
			oldVal := req.OldBoard[y][x]
			if oldVal != newVal {
				currentTurn := req.T
				l.c.events <- CellFlipped{CompletedTurns: currentTurn, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}
	fmt.Println("CellFlipped pass event")
	return
}
