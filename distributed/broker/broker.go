package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"sync"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

// Broker to register rpc
//var LocalController *rpc.Client

type Broker struct {
}

var board [][]byte

// Out set default false for controller out
var Out = false

var ControlLocal *rpc.Client

// create a lock
var mu sync.Mutex

// create a turn to store the turn
var turn int
var disWorkers []*rpc.Client

func StateCalculator(req gol.Request, res *gol.Response) [][]byte {
	// initialize a blank gameBoard
	board = make([][]byte, req.Params.ImageHeight)
	for i := range board {
		board[i] = make([]byte, req.Params.ImageWidth)
	}
	board = req.World

	// call LocalController at port 8060
	LocalController, _ := rpc.Dial("tcp", "127.0.0.1:8060")

	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			log.Fatal("client closing", err)
		}
	}(LocalController)

	//n is server number
	n := 1
	// workers ip
	//ip := [3]string{"0.0.0.0", "0.0.0.0", "0.0.0.0"}
	ip := [1]string{"18.206.125.124"}

	// call from 8010
	/*for i := 0; i < n; i++ {
		port := 8010 + i
		address := fmt.Sprintf("%s:%d", ip[i], port)
		fmt.Println(address)
		disWorker, err := rpc.Dial("tcp", address)
		if err != nil {
			log.Fatal("111", err)
		}
		disWorkers = append(disWorkers, disWorker)
	}*/

	lnWork, _ := net.Listen("tcp", "18.206.125.124:8010")
	defer func(lnWork net.Listener) {
		err := lnWork.Close()
		if err != nil {
			log.Fatal("Broker listener closing: ", err)
			return
		}
	}(lnWork)
	//defer rpc.Accept(lnWork)

	address := fmt.Sprintf("%s:8010", ip[0])
	disWorker, err := rpc.Dial("tcp", address)
	if err != nil {
		log.Fatal("111", err)
	}
	disWorkers = append(disWorkers, disWorker)

	for turn < req.Params.Turns {
		// call to distributed worker
		oldWorld := make([][]byte, req.Params.ImageWidth)
		for i := range oldWorld {
			oldWorld[i] = make([]byte, res.Params.ImageHeight)
			oldWorld = board
		}
		breakpoints := divider(n, req.Params.ImageHeight)
		worlds := sliceWorld(breakpoints, oldWorld)
		// make channel
		c := make([]chan [][]byte, n)
		for i := range c {
			c[i] = make(chan [][]byte)
		}
		for index := range worlds {
			if index == 0 {
				upper := oldWorld[len(oldWorld)-1]
				lower := oldWorld[breakpoints[index+1]-1]
				go callReturn(disWorkers[index], worlds[index], c[index], req.Params, upper, lower)
			} else if index == len(worlds)-1 {
				upper := oldWorld[breakpoints[index]]
				lower := oldWorld[0]
				go callReturn(disWorkers[index], worlds[index], c[index], req.Params, upper, lower)
			} else {
				upper := oldWorld[breakpoints[index]-1]
				lower := oldWorld[breakpoints[index+1]-1]
				go callReturn(disWorkers[index], worlds[index], c[index], req.Params, upper, lower)
			}
		}
		var newWorld [][]byte
		for _, channel := range c {
			fmt.Println(313)
			data := <-channel
			fmt.Println(312313)
			newWorld = append(newWorld, data...)
		}

		// get Board in server
		mu.Lock()
		fmt.Println("lock")
		board = newWorld

		turn++
		mu.Unlock()

		//call to pass the CellFlipped & turnCompleted event
		if Out == false {
			fmt.Println(Out)
			sendDifference(oldWorld, board, &turn)
			sendTurnCompleted(&turn)
		} else {
			Out = false
			break
		}
	}
	// initial the turn
	turn = 0
	return board
}

// divider divide world into smaller worlds
func divider(n, height int) []int {
	s := height - 1
	part := s / n
	var p1 int

	breakPoints := make([]int, n+1)
	for i := range breakPoints {
		if i == n {
			breakPoints[i] = height
		} else {
			breakPoints[i] = p1
			p1 = p1 + part
		}
	}
	return breakPoints
}

// return with CalculatedGameBoard
func callReturn(client *rpc.Client, worldSlice [][]byte, out chan [][]byte, p gol.Params, upper []byte, lower []byte) {
	req := gol.Request{World: worldSlice, Params: p, BoardTop: upper, BoardBottom: lower}
	res := new(gol.Response)
	fmt.Println(2313312331231)
	err := client.Call(gol.RemoteCalculate, req, res)
	if err != nil {
		log.Fatal("call distributor err: ", err)
	}
	out <- res.WorldRes
}

func sliceWorld(breakPoints []int, board [][]uint8) [][][]uint8 {
	worlds := make([][][]uint8, len(breakPoints)-1)

	var worldLen int
	for i, _ := range worlds {
		if i == len(worlds)-1 {
			worldLen = (breakPoints[i+1]) - breakPoints[i]
		} else {
			worldLen = (breakPoints[i+1] - 1) - breakPoints[i] + 1
		}
		worlds[i] = make([][]uint8, worldLen)
		for j := range worlds[i] {
			worlds[i][j] = make([]uint8, len(board[j]))
			//break
		}
	}

	PartLine := 0  //line number for parts
	BoardLine := 0 // line number for board
	count := 0     // index of parts
	l := len(board)

	for BoardLine < l-1 {
		if BoardLine == breakPoints[count+1] {
			count += 1
			PartLine = 0
		}
		data := board[BoardLine]
		copy(worlds[count][PartLine], data)
		PartLine += 1
		BoardLine += 1
	}

	return worlds
}

func (b *Broker) ShutDown(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Lock()
	turn = 0
	Out = true
	mu.Unlock()
	return
}

func (b *Broker) offLine(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Lock()
	for _, disWorker := range disWorkers {
		err := disWorker.Call(gol.ServerOff, req, res)
		if err != nil {
			fmt.Println("AWS off err")
		}
	}
	mu.Unlock()
	go os.Exit(1)
	return
}

func (b *Broker) MuLock(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Lock()
	return
}

func (b *Broker) MuUnLock(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Unlock()
	return
}

// CalculateNextState the caller to the StateCalculator
func (b *Broker) CalculateNextState(req gol.Request, res *gol.Response) (e error) {
	// calling StateCalculator calculateNextState
	fmt.Println("calculateNextState")
	res.WorldRes = StateCalculator(req, res)
	fmt.Println("Calculating new GameBoard")
	return
}

func (b *Broker) CalculateAliveCells(req gol.NewReq, res *gol.NewRes) (e error) {
	// calculate alive cells
	mu.Lock()
	cell := len(calculateAliveCells(board))
	turn := turn
	res.N = cell
	res.T = turn
	mu.Unlock()
	return
}

// GetCompletedTurns get the number of alive cells for TurnComplete event
func (b *Broker) GetCompletedTurns(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Lock()
	res.T = turn
	mu.Unlock()
	return
}

func (b *Broker) CopyWorld(req gol.NewReq, res *gol.NewRes) (e error) {
	mu.Lock()
	res.Board = board
	mu.Unlock()
	return
}

func calculateAliveCells(world [][]byte) []util.Cell {
	var cells []util.Cell

	for i, row := range world {
		for j, _ := range row {
			if world[i][j] == 255 {
				theCell := util.Cell{X: j, Y: i}
				cells = append(cells, theCell)
			}
		}
	}
	return cells[0:]
}

func sendTurnCompleted(turn *int) {
	req := gol.NewReq{OldBoard: nil, NewBoard: nil, T: *turn}
	res := new(gol.NewRes)
	err := ControlLocal.Call(gol.LocalComplete, req, res)
	if err != nil {
		return
	}
}

func sendDifference(oldBoard [][]byte, newBoard [][]byte, turn *int) {
	req := gol.NewReq{OldBoard: oldBoard, NewBoard: newBoard, T: *turn}
	res := new(gol.NewRes)
	// solve fault tolerance
	err := ControlLocal.Call(gol.LocalDif, req, res)
	if err != nil {
		return
	}
}

func main() {
	brokerAddr := flag.String("port", "8030", "Broker port to listen to")
	flag.Parse()
	// register with RPC service
	err := rpc.Register(&Broker{})
	if err != nil {
		log.Fatal("Broker register : ", err)
		return
	}
	listener, _ := net.Listen("tcp", "18.206.125.124:"+*brokerAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			log.Fatal("Broker listener closing: ", err)
			return
		}
	}(listener)

	rpc.Accept(listener)
}
