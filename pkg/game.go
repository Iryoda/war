package pkg

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/asaskevich/EventBus"
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/google/uuid"
)

func scaleDown(number int) float64 {
	if number == 32 {
		return 2
	}

	return 1
}

const UNITY_VELOCITY = 0.01
const GAME_SPEED = 32
const FIVE_MINUTES = 5 * time.Minute

var TIMEOUT_MINUTES = 6 * time.Minute.Seconds() * scaleDown(GAME_SPEED)

const BASE_THICKNESS = 20

type PLAYER_TYPE string
type TOWER_TYPE int
type UNITY_TYPE int
type UNITY_STATE int
type UNITY_SURROUND int
type ACTION string

var Bus = EventBus.New()
var UNITY_THICK = map[UNITY_TYPE]int{
	SOLDIER: 5,
}
var UNITY_BASE_COST = map[UNITY_TYPE]int{
	SOLDIER: 10,
}
var TECH_UPDATE_COST = map[int]int{
	1: 50,
	2: 100,
	3: 200,
}
var MINING_LEVEL_COST = map[int]int{
	1: 50,
	2: 100,
	3: 200,
}

const (
	UNITY = iota
	SEARCH
)

const (
	SOLDIER UNITY_TYPE = iota
	BOMBER
)

const (
	BASE TOWER_TYPE = iota
)

const (
	RED  PLAYER_TYPE = "RED"
	BLUE             = "BLUE"
)

const (
	IDDLE UNITY_STATE = iota
	MOVING
	COMBAT
	COMBAT_FINISHED
	DEAD
)

const (
	FRONT UNITY_SURROUND = iota
	BACK
	RIGHT
	LEFT
)

const (
	UPDATE_TECH   ACTION = "UPDATE_TECH"
	UPDATE_MINING ACTION = "UPDATE_MINING"
	BUY_SOLDIER   ACTION = "BUY_SOLDIER"
	DO_NOTHING    ACTION = "DO_NOTHING"
)

type GameField struct {
	Width           int
	Height          int
	Border          rl.Rectangle
	BorderThickness int
	BorderIsUp      bool
}

type Position struct {
	X int
	Y int
}

func CreateField(size int) GameField {
	return GameField{
		Width:           size,
		Height:          size,
		Border:          rl.NewRectangle(0, 0, float32(size), float32(size)),
		BorderThickness: 3,
		BorderIsUp:      true,
	}
}

type Player struct {
	Id              PLAYER_TYPE
	Coins           int
	TechnologyLevel int
	MiningLevel     int
}

type Screen struct {
	Width  int
	Height int
}

type CollisionBox = rl.Rectangle

func CreatePlayer(id PLAYER_TYPE) *Player {
	return &Player{Id: id, Coins: 100, MiningLevel: 1, TechnologyLevel: 1}
}

type Game struct {
	ID          uuid.UUID
	Field       GameField
	DisplayTime int
	ElapsedTime float64
	Screen      Screen
	PlayerRed   Player
	PlayerBlue  Player
	Speed       int
	Unities     []Unity
	Towers      []Tower
	Wall        Position
	Winner      PLAYER_TYPE
	Finished    bool
}

type CreateGameArgs struct {
	Speed int
}

type GameUpdateEvent struct {
	GameID           uuid.UUID
	Owner            PLAYER_TYPE
	Time             int
	Coins            int
	TechLevel        int
	TechUpdateCost   int
	MiningLevel      int
	MiningUpdateCost int
	Unities          int
	EnemyUnities     int
	UnityCost        int
}

func NewGame(args CreateGameArgs) Game {
	speed := args.Speed
	if speed == 0 {
		speed = GAME_SPEED
	}

	return Game{
		ID:         uuid.New(),
		Field:      CreateField(400),
		Screen:     Screen{Width: 400, Height: 400},
		Speed:      speed,
		PlayerRed:  *CreatePlayer(RED),
		PlayerBlue: *CreatePlayer(BLUE),
		Finished:   false,
	}
}

type Tower struct {
	Id           int
	Hp           int
	Type         TOWER_TYPE
	Position     Position
	Thickness    int
	PlayerOwner  PLAYER_TYPE
	CollisionBox CollisionBox
}

type Unity struct {
	Id                    int
	Hp                    int
	Power                 int
	Defense               int
	Speed                 int
	Position              Position
	Type                  UNITY_TYPE
	PlayerOwner           PLAYER_TYPE
	State                 UNITY_STATE
	TargetUnityId         int
	TargetPosition        Position
	AttackCooldownSeconds float64
	LastAttackAt          float64
}

type ActionEvent struct {
	Owner  PLAYER_TYPE
	Action ACTION
}

func Run() {
	g := NewGame(CreateGameArgs{Speed: 8})
	g.Init()

	RunGame(&g)
}

func RunGame(g *Game) PLAYER_TYPE {
	rl.SetTraceLogLevel(rl.LogError)
	rl.InitWindow(int32(g.Screen.Width), int32(g.Screen.Height), fmt.Sprint("War", g.ID))

	camera := rl.NewCamera2D(
		rl.NewVector2(0, 0),
		rl.NewVector2(0, 0),
		0.0,
		1.0,
	)

	for g.Winner == "" {
		if rl.WindowShouldClose() {
			break
		}

		rl.SetTargetFPS(60)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		rl.BeginMode2D(camera)

		g.Update()

		if g.Field.BorderIsUp {
			g.ListenKeyPress()
		}

		if !g.Field.BorderIsUp {
			g.RunWar(rl.GetTime())
		}

		g.Render()
		g.RenderUI()

		rl.EndDrawing()
	}

	return g.Winner
}

func (u Unity) GetColor() rl.Color {
	if u.State == DEAD {
		return rl.Gray
	}

	switch u.PlayerOwner {
	case RED:
		return rl.Red
	case BLUE:
		return rl.Blue
	}

	return rl.White
}

func (u Tower) GetColor() rl.Color {
	switch u.PlayerOwner {
	case RED:
		return rl.Red
	case BLUE:
		return rl.Blue
	}

	return rl.White
}

func (g *Game) Init() {
	blue := g.PlayerBlue
	red := g.PlayerRed

	// Blue Side
	bt := Tower{Id: 1, Hp: 100, Type: BASE, Position: g.GetBasePosition(blue.Id), PlayerOwner: blue.Id, Thickness: BASE_THICKNESS}
	g.Towers = append(g.Towers, bt)

	// Red Side
	rt := Tower{Id: 1, Hp: 100, Type: BASE, Position: g.GetBasePosition(red.Id), PlayerOwner: red.Id, Thickness: BASE_THICKNESS}
	g.Towers = append(g.Towers, rt)

	g.PlayerBlue = blue
	g.PlayerRed = red
}

func (g *Game) ListenKeyPress() {
	// BLUE
	if rl.IsKeyPressed(rl.KeyOne) {
		g.BuyUnity(SOLDIER, BLUE)
		fmt.Println("Blue bought a soldier")
	}
	if rl.IsKeyPressed(rl.KeyTwo) {
		g.InvestMining(BLUE)
		fmt.Println("Blue invested in mining")
	}
	if rl.IsKeyPressed(rl.KeyThree) {
		g.InvestTechnology(BLUE)
		fmt.Println("Blue invested in tech")
	}

	// RED
	if rl.IsKeyPressed(rl.KeyQ) {
		g.BuyUnity(SOLDIER, RED)
		fmt.Println("Red bought a soldier")
	}
	if rl.IsKeyPressed(rl.KeyW) {
		g.InvestMining(RED)
		fmt.Println("Red invested in mining")
	}
	if rl.IsKeyPressed(rl.KeyE) {
		g.InvestTechnology(RED)
		fmt.Println("Red invested in tech")
	}

	// BORDER
	if rl.IsKeyPressed(rl.KeySpace) {
		g.Field.BorderIsUp = false
	}
}

func (g *Game) Update() {
	t := rl.GetTime()
	diff := t - float64(g.ElapsedTime)

	if !g.Field.BorderIsUp {
		uB := len(g.GetAliveUnitiesByPlayerId(BLUE))
		uR := len(g.GetAliveUnitiesByPlayerId(RED))

		if g.DisplayTime > int(TIMEOUT_MINUTES) {
			if uB >= uR {
				g.Winner = BLUE
			} else {
				g.Winner = RED
			}

			fmt.Println("GAME TIMEOUT")
			return
		}

		if uR == 0 && uB > 0 {
			g.Winner = BLUE
			fmt.Println("BLUE WINS")
			return
		}

		if uB == 0 && uR > 0 {
			g.Winner = RED
			fmt.Println("RED WINS")
			return
		}
	}

	if g.Field.BorderIsUp && g.DisplayTime > int(FIVE_MINUTES.Seconds()) {
		g.Field.BorderIsUp = false
		return
	}

	if diff > (1.0 / GAME_SPEED) {
		g.ElapsedTime = t
		g.DisplayTime += 1

		g.PlayerBlue.Coins += g.PlayerRed.CalculateCoinsToReceive()
		g.PlayerRed.Coins += g.PlayerBlue.CalculateCoinsToReceive()

		if Bus != nil {
			Bus.Publish(fmt.Sprint("game:", g.ID, "/update"), g.NewGameUpdateEvent(BLUE))
			Bus.Publish(fmt.Sprint("game:", g.ID, "/update"), g.NewGameUpdateEvent(RED))
		}
	}

}

func (g Game) NewGameUpdateEvent(p PLAYER_TYPE) GameUpdateEvent {
	if p == BLUE {
		return GameUpdateEvent{
			GameID:           g.ID,
			Owner:            p,
			Time:             g.DisplayTime,
			TechLevel:        g.PlayerBlue.TechnologyLevel,
			TechUpdateCost:   TECH_UPDATE_COST[g.PlayerBlue.TechnologyLevel],
			MiningLevel:      g.PlayerBlue.MiningLevel,
			MiningUpdateCost: TECH_UPDATE_COST[g.PlayerBlue.MiningLevel],
			Coins:            g.PlayerBlue.Coins,
			UnityCost:        g.GetCurrentUnityCost(SOLDIER),
			Unities:          len(g.GetUnitiesByPlayerId(BLUE)),
			EnemyUnities:     len(g.GetUnitiesByPlayerId(RED)),
		}
	}

	return GameUpdateEvent{
		GameID:           g.ID,
		Owner:            p,
		Time:             g.DisplayTime,
		TechLevel:        g.PlayerRed.TechnologyLevel,
		TechUpdateCost:   TECH_UPDATE_COST[g.PlayerRed.TechnologyLevel],
		MiningLevel:      g.PlayerRed.MiningLevel,
		MiningUpdateCost: TECH_UPDATE_COST[g.PlayerRed.MiningLevel],
		Coins:            g.PlayerRed.Coins,
		UnityCost:        g.GetCurrentUnityCost(SOLDIER),
		Unities:          len(g.GetUnitiesByPlayerId(RED)),
		EnemyUnities:     len(g.GetUnitiesByPlayerId(BLUE)),
	}
}

func (g *Game) Render() {
	// Field
	rec := rl.NewRectangle(0, 0, float32(g.Field.Width), float32(g.Field.Height))
	rl.DrawRectangleLinesEx(rec, 3, rl.White)

	// Border
	if g.Field.BorderIsUp {
		rl.DrawRectangle(0, int32(g.Screen.Height/2), int32(g.Screen.Width), 5, rl.Orange)
	}

	// Towers
	for _, t := range g.Towers {
		switch t.Type {
		case BASE:
			rl.DrawRectangle(int32(t.Position.X), int32(t.Position.Y), int32(t.Thickness), int32(t.Thickness), t.GetColor())
		}
	}

	// Unities
	for _, u := range g.Unities {
		t := UNITY_THICK[u.Type]

		switch u.Type {
		case SOLDIER:
			if u.State == DEAD {
				continue
			}
			x := float64(u.Position.X) - float64(t)/2
			y := float64(u.Position.Y) - float64(t)/2
			rec := rl.NewRectangle(float32(x), float32(y), float32(t), float32(t))

			rl.DrawRectangleRec(rec, u.GetColor())
		case BOMBER:
			rl.DrawCircle(int32(u.Position.X), int32(u.Position.Y), float32(t), u.GetColor())
		}
	}
}

func (g *Game) RenderUI() {
	rl.DrawText(fmt.Sprint("Time:", g.DisplayTime), 0, 0, 16, rl.White)
	rl.DrawText(fmt.Sprint("Coins Blue:", g.PlayerBlue.Coins), 0, 25, 16, rl.White)
	rl.DrawText(fmt.Sprint("Coins Red:", g.PlayerRed.Coins), 0, 45, 16, rl.White)

	rl.DrawText(fmt.Sprint("Mining Level Blue:", g.PlayerBlue.MiningLevel), 0, 65, 16, rl.White)
	rl.DrawText(fmt.Sprint("Tech Level Blue:", g.PlayerBlue.TechnologyLevel), 0, 85, 16, rl.White)

	rl.DrawText(fmt.Sprint("Mining Level Red:", g.PlayerRed.MiningLevel), 0, 105, 16, rl.White)
	rl.DrawText(fmt.Sprint("Tech Level Red:", g.PlayerRed.TechnologyLevel), 0, 125, 16, rl.White)

	var aliveUnities = 0
	for _, u := range g.Unities {
		if u.State != DEAD {
			aliveUnities += 1
		}
	}

	rl.DrawText(fmt.Sprint("Unities:", aliveUnities), 0, 145, 16, rl.White)
}

func (g *Game) RunWar(time float64) {
Outer:
	for i, current := range g.Unities {
		if current.State == DEAD {
			continue
		}

		switch current.State {
		case IDDLE:
			u, p, err := g.CalculateUnityNextPosition(current)
			if err != nil {
				continue Outer
			}

			g.Unities[i].TargetUnityId = u.Id
			g.Unities[i].TargetPosition = p
			g.Unities[i].State = MOVING
		case MOVING:
			for idx, u := range g.Unities {
				if u.Id == current.Id || u.State == DEAD {
					continue
				}

				if rl.CheckCollisionRecs(current.GetCollisionBox(), u.GetCollisionBox()) {
					if u.PlayerOwner == current.PlayerOwner {
						target, err := g.GetUnityById(current.TargetUnityId)
						if err != nil {
							g.Unities[i].State = IDDLE
							continue Outer
						}

						if pos, _, err := g.GetAvailableSurroundPosition(target); err == nil {
							g.Unities[i].TargetPosition = pos
						}
					}

					if u.Id == current.TargetUnityId && u.PlayerOwner != current.PlayerOwner {
						g.Unities[i].State = COMBAT
						g.Unities[i].TargetUnityId = u.Id

						g.Unities[idx].State = COMBAT
						g.Unities[idx].TargetUnityId = current.Id

					}

					continue Outer
				}

				xDiff := current.TargetPosition.X - current.Position.X
				if xDiff != 0 {
					if xDiff > 0 {
						nP := Position{
							X: current.Position.X + current.Speed,
							Y: current.Position.Y,
						}

						g.UpdateUnityPosition(i, nP)
					}

					if xDiff < 0 {
						nP := Position{
							X: current.Position.X - current.Speed,
							Y: current.Position.Y,
						}

						g.UpdateUnityPosition(i, nP)
					}
				}

				yDiff := current.TargetPosition.Y - current.Position.Y
				if yDiff != 0 {
					if yDiff > 0 {
						nP := Position{
							X: current.Position.X,
							Y: current.Position.Y + current.Speed,
						}

						g.UpdateUnityPosition(i, nP)
					}

					if yDiff < 0 {
						nP := Position{
							X: current.Position.X,
							Y: current.Position.Y - current.Speed,
						}

						g.UpdateUnityPosition(i, nP)
					}
				}

				u, p, err := g.CalculateUnityNextPosition(g.Unities[i])
				if err != nil {
					continue Outer
				}

				g.Unities[i].TargetUnityId = u.Id
				g.Unities[i].TargetPosition = p
			}
		case COMBAT:
			target, err := g.FindTargetUnityById(current.TargetUnityId)
			if err != nil || target.State == DEAD {
				g.Unities[i].State = IDDLE
			}

			attkCd := current.getCoolDown()
			if attkCd != 0 {
				continue
			}

			dmg := g.CalculateUnityDamage(current, target)
			g.ExecuteDamage(target.Id, dmg)
			g.Unities[i].LastAttackAt = rl.GetTime()
		}
	}
}

func getNewUnityPositionByPlayer(id PLAYER_TYPE, game Game, uType UNITY_TYPE) Position {
	bP := game.GetBasePosition(id)
	unities := game.GetUnitiesByPlayerId(id)
	t := UNITY_THICK[uType]

	switch id {
	case BLUE:
		pos := Position{
			X: bP.X - rand.IntN(100) + rand.IntN(100),
			Y: bP.Y + 10 + rand.IntN(100) + BASE_THICKNESS,
		}

		rec := rl.NewRectangle(float32(pos.X)-float32(t/2), float32(pos.Y)-float32(t/2), float32(t), float32(t))

		for _, u := range unities {
			if rl.CheckCollisionRecs(rec, u.GetCollisionBox()) {
				return getNewUnityPositionByPlayer(id, game, uType)
			}
		}

		return pos

	case RED:
		pos := Position{
			X: bP.X - rand.IntN(100) + rand.IntN(100),
			Y: bP.Y - 10 - rand.IntN(100) - BASE_THICKNESS,
		}

		rec := rl.NewRectangle(float32(pos.X)-float32(t/2), float32(pos.Y)-float32(t/2), float32(t), float32(t))

		for _, u := range unities {
			if rl.CheckCollisionRecs(rec, u.GetCollisionBox()) {
				return getNewUnityPositionByPlayer(id, game, uType)
			}
		}

		return pos
	}

	return Position{}
}

func (g *Game) GetUnityPos(id int) Unity {
	for _, u := range g.Unities {
		if u.Id == id {
			return u
		}
	}

	return Unity{}
}

func (g *Game) BuyUnity(u UNITY_TYPE, id PLAYER_TYPE) error {
	p := g.GetPlayer(id)

	switch u {
	case SOLDIER:
		if p.Coins < g.GetCurrentUnityCost(u) {
			return errors.New("Not enough coins")
		}

		p.Coins -= g.GetCurrentUnityCost(u)
		g.addUnity(SOLDIER, id)
	}

	return nil
}

func (g *Game) GetPlayer(id PLAYER_TYPE) *Player {
	if id == RED {
		return &g.PlayerRed
	}

	return &g.PlayerBlue
}

func (p Player) CalculateCoinsToReceive() int {
	switch p.MiningLevel {
	case 1:
		return 1
	case 2:
		return 3
	case 3:
		return 7
	}

	return 1
}

func (g *Game) addUnity(t UNITY_TYPE, player PLAYER_TYPE) error {
	id := 1

	if len(g.Unities) != 0 {
		lastUnity := g.Unities[len(g.Unities)-1]
		id = lastUnity.Id + 1
	}

	switch t {
	case SOLDIER:
		pos := getNewUnityPositionByPlayer(player, *g, t)
		u := Unity{
			Id:                    id,
			Hp:                    10,
			Power:                 7,
			Defense:               2,
			Type:                  SOLDIER,
			Position:              pos,
			PlayerOwner:           player,
			Speed:                 1,
			State:                 IDDLE,
			AttackCooldownSeconds: 1,
		}

		g.Unities = append(g.Unities, u)
		return nil
	}

	return errors.New("Invalid Unity Type")
}

func (g *Game) InvestMining(id PLAYER_TYPE) error {
	p := g.GetPlayer(id)

	if p.MiningLevel == 3 {
		fmt.Println("Max level reached")
		return nil
	}

	switch p.MiningLevel {
	case 1:
		if p.Coins < MINING_LEVEL_COST[1] {
			fmt.Println("Investment error")
			return errors.New("Not enough coins")
		}

		p.Coins -= MINING_LEVEL_COST[1]
	case 2:
		if p.Coins < MINING_LEVEL_COST[2] {
			fmt.Println("Investment error")
			return errors.New("Not enough coins")
		}

		p.Coins -= MINING_LEVEL_COST[2]
	case 3:
		if p.Coins < MINING_LEVEL_COST[2] {
			fmt.Println("Investment error")
			return errors.New("Not enough coins")
		}

		p.Coins -= MINING_LEVEL_COST[3]
	}

	p.MiningLevel++

	return nil
}

func (g *Game) InvestTechnology(id PLAYER_TYPE) error {
	p := g.GetPlayer(id)

	if p.TechnologyLevel == 3 {
		fmt.Println("Max level reached")
		return nil
	}

	switch p.TechnologyLevel {
	case 1:
		if p.Coins < TECH_UPDATE_COST[1] {
			fmt.Println("Investment error")
			return errors.New("Not enough coins")
		}
		p.Coins -= TECH_UPDATE_COST[1]
	case 2:
		if p.Coins < TECH_UPDATE_COST[2] {
			fmt.Println("Investment error")
			return errors.New("Not enough coins")
		}

		p.Coins -= TECH_UPDATE_COST[2]
	case 3:
		if p.Coins < TECH_UPDATE_COST[3] {
			fmt.Println("Investment error")
			return errors.New("Not enough coins")
		}

		p.Coins -= TECH_UPDATE_COST[3]
	}

	p.TechnologyLevel++

	return nil
}

func (g Game) GetBasePosition(id PLAYER_TYPE) Position {
	x := g.Screen.Width/2 - (BASE_THICKNESS / 2)

	switch id {
	case BLUE:
		return Position{X: x, Y: BASE_THICKNESS}
	case RED:
		return Position{X: x, Y: g.Screen.Height - BASE_THICKNESS}
	}

	return Position{}
}

func (g Game) GetUnitiesByPlayerId(id PLAYER_TYPE) []Unity {
	var unities []Unity

	for _, u := range g.Unities {
		if u.PlayerOwner == id {
			unities = append(unities, u)
		}
	}

	return unities
}

func (g *Game) GetUnityById(id int) (Unity, error) {
	for _, u := range g.Unities {
		if u.Id == id {
			return u, nil
		}
	}

	return Unity{}, errors.New("Unity not found")
}

func CalculateUnityCBox(p Position, u UNITY_TYPE) CollisionBox {
	t := UNITY_THICK[u]

	switch u {
	case SOLDIER:
		return rl.NewRectangle(float32(p.X)-float32(t)/2, float32(p.Y)-float32(t)/2, 5, 5)
	}

	return rl.NewRectangle(0, 0, 0, 0)
}

func (g Game) FindTargetUnityById(id int) (Unity, error) {
	for _, u := range g.Unities {
		if u.Id == id {
			return u, nil
		}
	}

	return Unity{}, errors.New("Unity not found")
}

func FindClosestEnemyUnity(u Unity, g *Game) Unity {
	var closest Unity
	var distance float64

	for _, unity := range g.Unities {
		if unity.State == DEAD {
			continue
		}

		if unity.PlayerOwner == u.PlayerOwner {
			continue
		}

		d := rl.Vector2Distance(
			rl.NewVector2(float32(u.Position.X), float32(u.Position.Y)),
			rl.NewVector2(float32(unity.Position.X), float32(unity.Position.Y)),
		)

		if d < float32(distance) || distance == 0 {
			distance = float64(d)
			closest = unity
		}
	}
	return closest
}

func (g *Game) CalculateUnityDamage(attacker Unity, target Unity) int {
	return attacker.Power - target.Defense
}

func (g *Game) ExecuteDamage(unityId int, dmg int) {
	for i, u := range g.Unities {
		if u.Id == unityId {
			g.Unities[i].Hp = u.Hp - dmg

			if g.Unities[i].Hp <= 0 {
				g.Unities[i].State = DEAD
			}

			break
		}
	}
}

func (u Unity) GetCollisionBox() CollisionBox {
	t := UNITY_THICK[u.Type]
	return rl.NewRectangle(float32(u.Position.X)-float32(t)/2, float32(u.Position.Y)-float32(t)/2, 5, 5)
}

func (g *Game) GetUnityByPosition(p Position) (Unity, error) {
	for i, u := range g.Unities {
		if u.Position == p {
			return g.Unities[i], nil
		}
	}

	return Unity{}, errors.New("Unity not found")
}

func (g *Game) GetAvailableSurroundPosition(u Unity) (Position, UNITY_SURROUND, error) {
	front := Position{X: u.Position.X, Y: u.Position.Y + 1}
	right := Position{X: u.Position.X + 1, Y: u.Position.Y}
	left := Position{X: u.Position.X - 1, Y: u.Position.Y}
	back := Position{X: u.Position.X, Y: u.Position.Y - 1}

	pos := map[UNITY_SURROUND]Position{
		FRONT: front,
		RIGHT: right,
		LEFT:  left,
		BACK:  back,
	}

	for k, p := range pos {
		if !g.isPositionOverBorder(p) {
			if _, err := g.GetUnityByPosition(p); err == nil {
				return front, k, nil
			}
		}
	}

	return Position{}, FRONT, errors.New("No available position")
}

func (g Game) isPositionOverBorder(p Position) bool {
	if p.X < 0 || p.X > g.Field.Width {
		return true
	}

	if p.Y < 0 || p.Y > g.Field.Height {
		return true
	}

	return false
}

func (g *Game) CalculateUnityNextPosition(u Unity) (Unity, Position, error) {
	closestUnity := FindClosestEnemyUnity(u, g)

	if closestUnity.Id == 0 {
		return Unity{}, Position{}, errors.New("No enemy found")
	}

	return closestUnity, closestUnity.Position, nil
}

func (g *Game) UpdateUnityPosition(idx int, p Position) {
	if !g.isPositionOverBorder(p) {
		g.Unities[idx].Position = p
	}
}

func (u Unity) getCoolDown() float64 {
	if u.LastAttackAt == 0 {
		return 0
	}

	diff := rl.GetTime() - u.LastAttackAt

	if diff > float64(u.AttackCooldownSeconds) {
		return 0
	}

	return diff
}

func (g Game) GetCurrentUnityCost(u UNITY_TYPE) int {
	f := int(math.Floor(float64(g.DisplayTime)/float64(time.Minute.Seconds()))) + 1

	switch u {
	case SOLDIER:
		return f * UNITY_BASE_COST[u]
	}

	return UNITY_BASE_COST[SOLDIER]
}

func (g *Game) HandleActionEvent(e ActionEvent) {
	if !g.Field.BorderIsUp {
		return
	}

	switch e.Action {
	case BUY_SOLDIER:
		g.BuyUnity(SOLDIER, e.Owner)
	case UPDATE_TECH:
		g.InvestTechnology(e.Owner)
	case UPDATE_MINING:
		g.InvestMining(e.Owner)
	}
}

func (g *Game) GetPlayerById(id PLAYER_TYPE) Player {
	if id == BLUE {
		return g.PlayerBlue
	}

	return g.PlayerRed
}

func (g *Game) GetAliveUnitiesByPlayerId(id PLAYER_TYPE) []Unity {
	var aliveUnities []Unity
	for _, u := range g.Unities {
		if u.State != DEAD && u.PlayerOwner == id {
			aliveUnities = append(aliveUnities, u)
		}
	}

	return aliveUnities
}
