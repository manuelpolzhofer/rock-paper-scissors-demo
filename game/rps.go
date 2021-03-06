package game
import (
	"math/big"
	"bufio"
	"log"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"
	"encoding/base64"
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-peer"

	"github.com/mikiquantum/rock-paper-scissors-demo/p2p"
)

type Choice int

const (
	Rock Choice = iota
	Paper
	Scissors
)

func (move Choice) String() string {
	names := [...]string{"Rock", "Paper", "Scissors"}
	return names[move]
}

type Result int

const (
	Draw Result = iota
	Win
	Lose
)

func (result Result) String() string {
	resultNames := [...]string{"Draw", "Win", "Lose"}
	return resultNames[result]
}

const MAX_ROUNDS = 5

var appChannel = make(chan string, 1)

type Player struct {
	host host.Host
}

func (player Player) StartPlaying() {
	opponent := player.waitForOpponent()
	player.startGame(opponent)
}

func NewPlayer(host host.Host) (player Player) {
	host.SetStreamHandler(p2p.GAME_STREAM_PID, HandleResponse)
	return Player{host: host}
}

func (player Player) startGame(opponent peer.ID) {
	roundCount := 0
	winCount := 0
	for roundCount < MAX_ROUNDS {
		choice := generateChoice()
		log.Printf("My Choice [%s]", choice)
	
		salt := generateSalt()
		commitment := generateCommitment(salt, choice)
		log.Printf("Generating commitment and sending it to opponent: %s\n", commitment)
		p2p.SendString(player.host, commitment, opponent)

		// Waiting to receive commitment from counterpart
		opponentCommitment := <- appChannel
		log.Printf("Got commitment from opponent: %s", opponentCommitment)

		log.Printf("Revealing commitment: %s", fmt.Sprintf("%s,%d", salt, choice))
		p2p.SendString(player.host, fmt.Sprintf("%s,%d", salt, choice), opponent)
		opponentChoiceMessage := <- appChannel
		log.Printf("Opponent revealed commitment: %s", opponentChoiceMessage)
		s := strings.Split(opponentChoiceMessage, ",")
		opponentSalt := s[0]
		opponentChoiceInt, _ := strconv.Atoi(s[1])
		opponentChoice := Choice(opponentChoiceInt)
		
		log.Printf("Checking commitment to match salt+choice: %s, %s, %s", opponentCommitment, opponentSalt, opponentChoice)
		if !checkCommitment(opponentSalt, opponentChoice, opponentCommitment) {
			log.Printf("\x1b[41mOpponent commitment verification failed. Aborting game!\x1b[0m") //ansicolor red
			return
		}
		result := analyzeResult(choice, opponentChoice)
		log.Printf("\x1b[4m[%s] - [%s]\x1b[0m", choice, opponentChoice)
		log.Printf("Round [%d] -> You %s\n", roundCount, result)
		if result == Win {
			winCount++
		}

		if result != Draw {
			roundCount++
		}
		time.Sleep(2 * time.Second)
	}
	if float32(winCount) > float32(MAX_ROUNDS/2) {
		log.Printf("Finished Game! \x1b[42mYou WIN! %d/%d\x1b[0m", winCount, MAX_ROUNDS) //ansicolor green
	} else {
		log.Printf("Finished Game! \x1b[41mYou LOSE! %d/%d\x1b[0m", winCount, MAX_ROUNDS) //ansicolor red
	}
}

func analyzeResult(myChoice Choice, opponentChoice Choice) Result {
	return Result((((myChoice - opponentChoice) % 3) + 3) % 3)
}

func HandleResponse(s net.Stream) {
	// Create a buffer stream for non blocking read and write.
	rwBuffer := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	go readMsg(rwBuffer)
}

func readMsg(rw *bufio.ReadWriter) {
	for {
		str, _ := rw.ReadString('\n')

		if str == "" {
			return
		}
		if str != "\n" {
			appChannel <- str[:len(str)-1]
		}

	}
}

func (player Player) waitForOpponent() (opponent peer.ID) {
	timeoutTries := 150
	tryCount := 0
	found := false
	log.Printf("Waiting for Opponent\n")
	for {
		if tryCount >= timeoutTries {
			log.Printf("Error: Timeout waiting for an opponent")
			break
		}
		for _, v := range player.host.Peerstore().Peers() {
			// Checking if the new entry in peerstore is neither local peer or bootstrap one and pick the first one available
			if v != player.host.ID() && v.Pretty() != strings.Split(p2p.HARDCODED_BOOTSTRAP_NODE, "/")[6] {
				opponent = v
				found = true
				break
			} else {
				tryCount++
				time.Sleep(2 * time.Second)
			}
		}
		if found {
			log.Printf("Opponent [%s] found\n", opponent.Pretty())
			break
		}
	}
	return
}

func generateChoice() (choice Choice) {
	choiceInt, _ := rand.Int(rand.Reader, big.NewInt(3))
	return Choice(int(choiceInt.Int64()))
}

func generateSalt() (salt string) {
	saltBytes := make([]byte, 32)
	rand.Read(saltBytes)
	return base64.URLEncoding.EncodeToString(saltBytes)
}

func generateCommitment(salt string, choice Choice) string {
	sum := sha256.Sum256(append([]byte(salt), []byte{byte(choice)}...))
	return base64.URLEncoding.EncodeToString(sum[:])
}

func checkCommitment(salt string, choice Choice, commitment string) bool {
	return generateCommitment(salt, choice) == commitment
}
