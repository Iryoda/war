package pkg

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"time"
)

// Input Layer
// Time -> w0
// Coins -> w1
// Technology Level -> w2
// Tech Update Cost -> w3
// Mining Level -> w4
// Mining Update Cost -> w5
// My Unities -> w6
// Enemy Unities -> w7
// Unity Cost -> w8

// Output Layer
// Upgrade Tech -> w0
// Upgrade Mining -> w1
// Buy Unity -> w2
func (e GameUpdateEvent) ToInput() [INPUT_SIZE]float64 {
	normalizedTime := float64(e.Time) / (5 * time.Minute.Seconds())
	micron := 100.0

	return [INPUT_SIZE]float64{
		normalizedTime,
		float64(e.Coins) / micron,
		float64(e.TechLevel) / 3,
		float64(e.TechUpdateCost) / micron,
		float64(e.MiningLevel) / 3,
		float64(e.MiningUpdateCost) / micron,
		float64(e.Unities / 100),
		float64(e.EnemyUnities / 100),
		float64(e.UnityCost) / micron,
	}
}

const INPUT_SIZE = 9
const HIDDEN_LAYER = 12
const HIDDEN_LAYER_2 = 5
const OUTPUT_SIZE = 3

type Input [INPUT_SIZE]float64
type Output [OUTPUT_SIZE]float64

type Model struct {
	Type PLAYER_TYPE
	// Input
	W_I_H1 [][]float64

	// Hidden 1
	W_H1_H2 [][]float64
	B_H1    []float64
	// Hidden 2
	W_H2_O [][]float64
	B_H2   []float64
	// Output
	B_O    []float64
	Points int
}

type BatchMap struct {
	Model  Model
	Points int
}

type Train struct {
	Id           int
	Game         *Game
	ModelRed     *Model
	ModelBlue    *Model
	Winner       PLAYER_TYPE
	WinnerPoints int
}

const GENERATIONS = 30
const GAME_PER_GEN = 4

func RunTrain() {
	trains := map[int][]Train{}
	currentBetter := Model{}

	for gen := range GENERATIONS {
		// 10 ROUNDS
		for j := range GAME_PER_GEN {
			// Prepare Game
			g := NewGame(CreateGameArgs{
				Speed: 32,
			})
			g.Init()

			// Neural Network Blue
			var mB Model
			// Neural Network Red
			var mR Model

			if currentBetter.Points == 0 {
				fmt.Println("Generating Random Models")
				mB = Model{Type: g.PlayerBlue.Id}
				mR = Model{Type: g.PlayerRed.Id}

				mB.InitRandom()
				mR.InitRandom()
			}

			if currentBetter.Type != "" {
				// BLUE
				m, _ := currentBetter.Copy()
				mB = *m
				mB.Mutate(0.2)

				// RED
				m, _ = currentBetter.Copy()
				mR = *m
				mR.Mutate(0.2)
			}

			t := Train{
				Id:        gen + j,
				ModelRed:  &mR,
				ModelBlue: &mB,
				Game:      &g,
			}

			// Subs to Event
			Bus.Subscribe(fmt.Sprint("game:", g.ID, "/update"), func(e GameUpdateEvent) {
				actions := mB.HandleUpdate(e)
				for _, a := range actions {
					g.HandleActionEvent(ActionEvent{
						Owner:  BLUE,
						Action: a,
					})
				}
			})
			Bus.Subscribe(fmt.Sprint("game:", g.ID, "/update"), func(e GameUpdateEvent) {
				actions := mR.HandleUpdate(e)
				for _, a := range actions {
					g.HandleActionEvent(ActionEvent{
						Owner:  RED,
						Action: a,
					})
				}
			})

			fmt.Println("TRAIN GEN:", gen, "GAME: ", j, "STARTED")
			winner := RunGame(&g)

			t.Winner = winner

			points := t.GetWinnerPoints()
			t.WinnerPoints = points

			if winner == BLUE {
				mB.Points = points
			} else {
				mR.Points = points
			}

			trains[gen] = append(trains[gen], t)
			fmt.Println("TRAIN GEN:", gen, "GAME: ", j, "WINNER: ", winner, "TRAIN", t, "POINTS:", points)
		}

		fmt.Println("TRAIN GEN FINISHED:", gen, trains[gen])

		for _, v := range trains[gen] {
			if v.WinnerPoints > currentBetter.Points {
				fmt.Println("CURRENT UPDATED", v.WinnerPoints)
				winner := func() Model {
					if v.Winner == BLUE {
						return *v.ModelBlue
					}
					return *v.ModelRed
				}()

				currentBetter = winner
				continue
			}
		}

		WriteModel(currentBetter, fmt.Sprint("./train/", "gen-", gen, ".json"))
	}

	WriteModel(currentBetter, "best.json")

	fmt.Println(trains)
}

func WriteModel(m Model, file string) {
	fmt.Println("WRITTING CURRENT")
	res, err := json.Marshal(m)
	if err != nil {
		fmt.Errorf(err.Error())
	}

	err = os.WriteFile(file, res, 0644)
	if err != nil {
		fmt.Errorf(err.Error())
	}
}

func (t *Train) GetWinnerPoints() int {
	enemyId := func() PLAYER_TYPE {
		if t.Winner == BLUE {
			return RED
		}
		return BLUE
	}()

	winner := t.Game.GetPlayerById(t.Winner)
	loser := t.Game.GetPlayerById(enemyId)

	totalUnities := len(t.Game.GetUnitiesByPlayerId(winner.Id))
	// aliveUnits := len(t.Game.GetAliveUnitiesByPlayerId(winner.Id))
	enemyTotalUnities := len(t.Game.GetUnitiesByPlayerId(loser.Id))

	techLevel := winner.TechnologyLevel

	miningLevel := winner.MiningLevel

	if totalUnities == 0 {
		return 0
	}

	return enemyTotalUnities*50 + totalUnities*100 + techLevel*1000 + miningLevel*1000 + int(winner.TotalCoins/2) - winner.Coins*10
}

// Initiate all weights with random numbers from
// -1.0 to 1.0
func (m *Model) InitRandom() {
	// Input
	for range HIDDEN_LAYER {
		// W I -> H1 [12,9]
		w := []float64{}
		for range INPUT_SIZE {
			// Append [9]float
			w = append(w, randomFloat(-1.0, 1.0))
		}
		// [12][9]float64
		m.W_I_H1 = append(m.W_I_H1, w)
		m.B_H1 = append(m.B_H1, randomFloat(-1.0, 1.0))
	}

	for range HIDDEN_LAYER_2 {
		// W H1 -> 2 [5, 12]
		w := []float64{}
		for range HIDDEN_LAYER {
			// [12]float
			w = append(w, randomFloat(-1.0, 1.0))
		}
		// [5][12]float
		m.W_H1_H2 = append(m.W_H1_H2, w)
		m.B_H2 = append(m.B_H2, randomFloat(-1.0, 1.0))
	}

	// Hidden 2
	for range OUTPUT_SIZE {
		// W H2 -> O [5, 3]
		w := []float64{}
		for range HIDDEN_LAYER_2 {
			// [5]float
			w = append(w, randomFloat(-1.0, 1.0))
		}
		// [5][3]float
		m.W_H2_O = append(m.W_H2_O, w)
		m.B_O = append(m.B_O, randomFloat(-1.0, 1.0))
	}
}

func (m *Model) HandleUpdate(e GameUpdateEvent) []ACTION {
	if e.Owner != m.Type {
		return []ACTION{}
	}

	input := e.ToInput()

	out := m.Result(input)

	return OutToAction(out)
}

func OutToAction(out Output) []ACTION {
	res := []ACTION{}

	if step(out[2]) == 1 {
		res = append(res, UPDATE_TECH)
	}

	if step(out[1]) == 1 {
		res = append(res, UPDATE_MINING)
	}

	if step(out[0]) == 1 {
		res = append(res, BUY_SOLDIER)
	}

	return res
}

func (m Model) Result(input Input) Output {
	// Input -> H1 Layer
	var l0 [HIDDEN_LAYER]float64
	for i, n := range m.W_I_H1 {
		for j, in := range input {
			l0[i] += in * n[j]
		}

		l0[i] += m.B_H1[i]
		l0[i] = math.Tanh(l0[i])
	}

	// HIDDEN 1 -> H2
	var h1 [HIDDEN_LAYER_2]float64
	for i, n := range m.W_H1_H2 {
		for j, val := range l0 {
			h1[i] += n[j] * val
		}

		h1[i] += m.B_H2[i]
		h1[i] = math.Tanh(h1[i])
	}

	// HIDDEN 2 -> Out
	var out [OUTPUT_SIZE]float64
	for i, n := range m.W_H2_O {
		for j, val := range h1 {
			out[i] += n[j] * float64(val)
		}

		out[i] += m.B_O[i]
		out[i] = sigmoid(out[i])
	}

	return out
}

func sigmoid(x float64) float64 {
	return (1 / (1 + math.Exp(x*(-1))))
}

func randomFloat(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func step(x float64) float64 {
	if x > 0.5 {
		return 1.0
	}
	return 0
}

func relu(x float64) float64 {
	return math.Max(0.0, x)
}

func (m Model) Copy() (*Model, error) {
	origJson, err := json.Marshal(m)
	clone := Model{}

	if err = json.Unmarshal(origJson, &clone); err != nil {
		return nil, err
	}

	return &clone, nil
}

func (m *Model) Mutate(rate float64) {
	f := func(_ float64) float64 {
		return randomFloat(-1.0, 1.0)
	}

	arr := [][]float64{}
	for _, a := range m.W_I_H1 {
		arr = append(arr, MutateArr(a, rate, f))
	}
	m.W_I_H1 = arr

	arr = [][]float64{}
	for _, a := range m.W_H1_H2 {
		arr = append(arr, MutateArr(a, rate, f))
	}
	m.W_H1_H2 = arr
	m.B_H1 = MutateArr(m.B_H1, rate, f)

	arr = [][]float64{}
	for _, a := range m.W_H2_O {
		arr = append(arr, MutateArr(a, rate, f))
	}
	m.W_H2_O = arr
	m.B_H2 = MutateArr(m.B_H2, rate, f)

	m.B_O = MutateArr(m.B_O, rate, f)
}

func MutateArr(arr []float64, rate float64, f func(float64) float64) []float64 {
	nArr := []float64{}

	for _, val := range arr {
		if rand.Float64() < rate {
			nArr = append(nArr, f(val))
			continue
		}

		nArr = append(nArr, val)
	}

	return nArr
}

// Get 2 models and breed then
// func BreedModels(m1 Model, m2 Model) (M1, M2 Model) {
// 	In := [INPUT_SIZE]float64{}
// 	H1 := [INPUT_SIZE]float64{}
// 	Out := [OUTPUT_SIZE]float64{}
//
// 	In_2 := [INPUT_SIZE]float64{}
// 	H1_2 := [INPUT_SIZE]float64{}
// 	Out_2 := [OUTPUT_SIZE]float64{}
//
// 	return In, H1, Out
// }
