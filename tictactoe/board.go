package tictactoe

import "fmt"

type Board struct {
	Grid [3][3]rune
	MovesCount int
}

func NewBoard() *Board {
	board := &Board{}
	board.InitializeBoard()
	return board
}

func (b *Board) InitializeBoard() {
	for row := range 3 {
		for col := range 3 {
			b.Grid[row][col] = '_'
		}
	}
	b.MovesCount = 0
}

func (b *Board) MakeMove(row, col int, symbol rune) error {
	if row < 0 || row >= 3 || col < 0 || col >= 3 || b.Grid[row][col] != '_'{
		return fmt.Errorf("invalid move")
	}
	b.Grid[row][col] = symbol
	b.MovesCount++
	return nil
}

func (b *Board) IsFull() bool{
	return b.MovesCount == 9
}

func (b *Board) HasWinner() bool {
	//check rows
	for row := range 3 {
		if b.Grid[row][0] != '_' && b.Grid[row][0] == b.Grid[row][1] && b.Grid[row][1] == b.Grid[row][2] {
			return true
		}
	}

	//check columns
	for col := range 3 {
		if b.Grid[0][col] != '_' && b.Grid[0][col] == b.Grid[1][col] && b.Grid[1][col] == b.Grid[2][col] {
			return true
		}
	}

	//check diagonals
	if b.Grid[0][0] != '_' && b.Grid[0][0] == b.Grid[1][1] && b.Grid[1][1] == b.Grid[2][2]{
		return true
	}

	return b.Grid[0][2] != '_' && b.Grid[0][2] == b.Grid[1][1] && b.Grid[1][1] == b.Grid[2][0]
}


func (b *Board) PrintBoard() {
	for row := range 3{
		for col := range 3 {
			fmt.Print(string(b.Grid[row][col]) + " ")
		}
		fmt.Println()
	}
	fmt.Println()
}