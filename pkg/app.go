package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func Run() {
	g := NewGame(CreateGameArgs{Speed: 1})
	g.Init()

	RunGame(&g)
}

func AiXAi() {
	f, err := os.Open("best.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	var m Model
	err = json.Unmarshal(b, &m)
	if err != nil {
		panic(err)
	}

	mB, _ := m.Copy()
	mR, _ := m.Copy()

	mB.Type = BLUE
	mR.Type = RED

	g := NewGame(CreateGameArgs{
		Speed: 32,
	})
	g.Init()

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

	RunGame(&g)
}

func PalyerXAi() {
	f, err := os.Open("best.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	var m Model
	err = json.Unmarshal(b, &m)
	if err != nil {
		panic(err)
	}

	mR, _ := m.Copy()

	mR.Type = RED

	g := NewGame(CreateGameArgs{
		Speed: 1,
	})
	g.Init()

	// Subs to Event
	Bus.Subscribe(fmt.Sprint("game:", g.ID, "/update"), func(e GameUpdateEvent) {
		actions := mR.HandleUpdate(e)
		for _, a := range actions {
			g.HandleActionEvent(ActionEvent{
				Owner:  RED,
				Action: a,
			})
		}
	})

	RunGame(&g)
}
