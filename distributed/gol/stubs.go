package gol

var ServerOff = "RemoteMachine.ServerOff"

var RemoteCalculate = "RemoteMachine.CalculateNextState"

var BrokerCalculate = "Broker.CalculateNextState"

var BrokerCalculateAliveNums = "Broker.CalculateAliveNums"

var BrokerCalculateAliveCells = "Broker.CalculateAliveCells"

var BrokerCopyWorld = "Broker.CopyWorld"

var BrokerMuLock = "Broker.MuLock"

var BrokerMuUnLock = "Broker.MuUnLock"

var BrokerShutDown = "Broker.ShutDown"

var BrokerOffLine = "Broker.offLine"

var LocalDif = "LocalMachine.TellTheDifference"

var LocalComplete = "LocalMachine.TurnCompleted"

type Response struct {
	WorldRes    [][]byte
	Params      Params
	BoardTop    []byte
	BoardBottom []byte
}

type Request struct {
	World       [][]byte
	Params      Params
	BoardTop    []byte
	BoardBottom []byte
}

type NewReq struct {
	T        int
	N        int
	OldBoard [][]byte
	NewBoard [][]byte
}

type NewRes struct {
	T     int
	N     int
	Board [][]byte
}
