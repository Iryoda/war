package pkg

import (
	"fmt"
	"math"
	"math/rand/v2"
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
func (e GameUpdateEvent) ToInput() [INPUT_SIZE]int {
	return [INPUT_SIZE]int{
		e.Time,
		e.Coins,
		e.TechLevel,
		e.TechUpdateCost,
		e.MiningLevel,
		e.MiningUpdateCost,
		e.Unities,
		e.EnemyUnities,
		e.UnityCost,
	}
}

const INPUT_SIZE = 9
const HIDDEN_LAYER = 12
const HIDDEN_LAYER_2 = 5
const OUTPUT_SIZE = 3

type Input [INPUT_SIZE]int
type Output [OUTPUT_SIZE]float64

type Model struct {
	Type PLAYER_TYPE
	// Input
	I   []float64
	B_I []float64
	// Hidden 1
	H1   []float64
	B_H1 []float64
	// Hidden 2
	H2   []float64
	B_H2 []float64
	// Output
	O      []float64
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

const GENERATIONS = 10
const GAME_PER_GEN = 5

func RunTrain() {
	trains := map[int][]Train{}
	var currentBetter Model

	for gen := range GENERATIONS {
		// 10 ROUNDS
		for j := range GAME_PER_GEN {
			// Prepare Game
			g := NewGame(CreateGameArgs{
				Speed: 32,
			})
			g.Init()

			var mB Model
			var mR Model

			if currentBetter.Type == "" {
				mB = Model{Type: g.PlayerBlue.Id}
				mR = Model{Type: g.PlayerRed.Id}
				mB.InitRandom()
				mR.InitRandom()
			}

			if currentBetter.Type != "" {
				mB = currentBetter.Copy()
				mB.Mutate(0.2)

				mR = currentBetter.Copy()
				mR.Mutate(0.2)
			}

			t := Train{
				Id:        gen + j,
				ModelRed:  &mR,
				ModelBlue: &mB,
				Game:      &g,
			}

			trains[gen] = append(trains[gen], t)

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

			fmt.Println("TRAIN GEN:", gen, "GAME: ", j, "WINNER: ", winner, "TRAIN", t, "POINTS:", points)
		}

		fmt.Println("TRAIN GEN FINISHED:", gen, trains)

		for _, v := range trains[gen] {
			winner := func() Model {
				if v.Winner == BLUE {
					return *v.ModelBlue
				}
				return *v.ModelRed
			}()

			if winner.Points > currentBetter.Points {
				currentBetter = winner
			}
		}
	}

	// SELECT BETTER MODELS

	// MUTATE AND DO

	fmt.Println(trains)
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
	enemyKilledUnities := len(t.Game.GetUnitiesByPlayerId(loser.Id))

	techLevel := winner.TechnologyLevel
	enemyTechLevel := loser.TechnologyLevel

	miningLevel := winner.MiningLevel
	enemyMiningLevel := loser.MiningLevel

	if totalUnities == 0 {
		return 0
	}

	return totalUnities*2 + enemyKilledUnities*1 + techLevel*10 + enemyTechLevel*5 + miningLevel*10 + enemyMiningLevel*5
}

// Initiate all weights with random numbers from
// -1.0 to 1.0
func (m *Model) InitRandom() {
	// Input Layer
	for range INPUT_SIZE {
		m.I = append(m.I, randomFloat(-1.0, 1.0))
		m.B_I = append(m.B_I, randomFloat(-1.0, 1.0))
	}

	// Hidden 1
	for range HIDDEN_LAYER {
		m.H1 = append(m.H1, randomFloat(-1.0, 1.0))
		m.B_H1 = append(m.B_H1, randomFloat(-1.0, 1.0))
	}

	// Hidden 2
	for range HIDDEN_LAYER_2 {
		m.H2 = append(m.H2, randomFloat(-1.0, 1.0))
		m.B_H2 = append(m.B_H2, randomFloat(-1.0, 1.0))
	}

	// Output
	for range OUTPUT_SIZE {
		m.O = append(m.O, randomFloat(-1.0, 1.0))
		m.B_O = append(m.B_O, randomFloat(-1.0, 1.0))
	}

	fmt.Println("Input Layer", m.I, "Bias", m.B_I)
	fmt.Println("Hidden Layer 1", m.H1, "Bias", m.B_H1)
	fmt.Println("Hidden Layer 2", m.H2, "Bias", m.B_H2)
	fmt.Println("Output", m.O)
}

func (m *Model) HandleUpdate(e GameUpdateEvent) []ACTION {
	if e.Owner != m.Type {
		return []ACTION{}
	}
	fmt.Println(e)

	input := e.ToInput()

	out := m.Result(input)

	return OutToAction(out)
}

func OutToAction(out Output) []ACTION {
	res := []ACTION{}

	if step(out[0]) == 1 {
		res = append(res, BUY_SOLDIER)
	}

	if step(out[1]) == 1 {
		res = append(res, UPDATE_MINING)
	}

	if step(out[2]) == 1 {
		res = append(res, UPDATE_TECH)
	}

	return res
}

func (m Model) Result(input Input) Output {
	// Input Layer
	var l0 [INPUT_SIZE]float64
	for i, w := range m.I {
		for _, val := range input {
			l0[i] += w*float64(val) + m.B_I[i]
		}

		l0[i] = relu(l0[i])
	}

	// HIDDEN 1
	var h1 [HIDDEN_LAYER]float64
	for i, w := range m.H1 {
		for _, val := range l0 {
			h1[i] += w*float64(val) + m.B_H1[i]
		}

		h1[i] = relu(h1[i])
	}
	// HIDDEN 2
	var h2 [HIDDEN_LAYER_2]float64
	for i, w := range m.H2 {
		for _, val := range h1 {
			h2[i] += w*float64(val) + m.B_H2[i]
		}

		h2[i] = relu(h1[i])
	}

	// Output Layer
	var out [OUTPUT_SIZE]float64
	for i, w := range m.O {
		for _, l := range h2 {
			out[i] += w*l + m.B_O[i]
		}

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

func (m Model) Copy() Model {
	return Model{
		I:    m.I,
		B_I:  m.B_I,
		H1:   m.H1,
		B_H1: m.B_H1,
		H2:   m.H2,
		B_H2: m.B_H2,
		O:    m.O,
	}
}

func (m *Model) Mutate(rate float64) {
	f := func(_ float64) float64 {
		return randomFloat(-100.0, 100.0)
	}

	m.I = MutateArr(m.I, rate, f)
	m.B_I = MutateArr(m.B_I, rate, f)

	m.H1 = MutateArr(m.H1, rate, f)
	m.B_H1 = MutateArr(m.B_H1, rate, f)
}

func MutateArr(arr []float64, rate float64, f func(float64) float64) []float64 {
	nArr := []float64{}

	for _, val := range arr {
		if rand.Float64() < rate {
			nArr = append(nArr, val)
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
